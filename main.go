package main

import (
	"context"
	"io"
	"load-balancer-go/ratelimiter"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const failThresholdForBackend int32 = 3
const shutdownTimeout = 30 * time.Second
const healthCheckInterval = 5 * time.Second

type Service struct {
	Address     string
	FailCounter atomic.Int32
	Healthy     atomic.Bool
}

type ServicePool struct {
	instances []*Service
	next      atomic.Uint64
	// Might use when we need to add/remove backends on-the-fly
	// mu        sync.Mutex
}

type LoadBalancer struct {
	listenAddr string
	pool       *ServicePool
	rl         *ratelimiter.RateLimiter
}

func main() {
	rl := ratelimiter.NewRateLimiter(
		10000,           // capacity
		10,              // tokens per second
		100*time.Second, // TTL
	)

	services := []string{
		"localhost:9001",
		"localhost:9002",
		"localhost:9003",
	}
	pool := newServicePool(services)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go healthCheck(ctx, pool)

	lb := LoadBalancer{
		listenAddr: ":8080",
		pool:       pool,
		rl:         rl,
	}
	log.Println("Starting Load Balancer...")

	if err := lb.Start(ctx); err != nil {
		log.Fatal(err)
	}
}

func newServicePool(addresses []string) *ServicePool {
	var instances []*Service
	for _, a := range addresses {
		newService := &Service{Address: a}
		newService.Healthy.Store(true)
		instances = append(instances, newService)
	}
	return &ServicePool{
		instances: instances,
		next:      atomic.Uint64{},
	}
}

func (p *ServicePool) nextHealthyInstance() *Service {
	total := len(p.instances)
	for i := 0; i < total; i++ {
		index := p.next.Add(1) - 1
		svc := p.instances[int(index)%total]
		if svc.Healthy.Load() {
			return svc
		}
	}
	return nil
}

func (lb *LoadBalancer) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", lb.listenAddr)
	if err != nil {
		return err
	}
	// Graceful shutdown of listener
	go func() {
		<-ctx.Done()
		listener.Close()
	}()
	defer listener.Close()

	var wg sync.WaitGroup
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				log.Println("Shutting down, draining in-flight connections...")
				done := make(chan struct{})
				go func() {
					wg.Wait()
					close(done)
				}()

				select {
				case <-done:
					log.Println("All connections drained cleanly")
				case <-time.After(shutdownTimeout):
					log.Println("Shutdown timeout exceeded, forcing exit")
				}

				return nil
			}
			log.Println("LB connection failed:", err)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			lb.handleConn(conn)
		}()
	}
}

func healthCheck(ctx context.Context, pool *ServicePool) {
	for {
		allHealthy := true
		for _, svc := range pool.instances {
			healthConn, err := net.DialTimeout("tcp", svc.Address, time.Millisecond*500)
			if err != nil {
				allHealthy = false
				if svc.Healthy.Load() && svc.FailCounter.Add(1) >= failThresholdForBackend {
					log.Println("Backend unhealthy:", svc.Address)
					svc.Healthy.Store(false)
				}
				continue
			}
			healthConn.Close()
			if !svc.Healthy.Load() {
				log.Println("backend recovered:", svc.Address)
			}
			svc.FailCounter.Store(0)
			svc.Healthy.Store(true)
		}

		interval := healthCheckInterval
		// Unhealthy backends reduce the health check time interval
		if !allHealthy {
			interval = interval / 5
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func (lb *LoadBalancer) handleConn(clientConn net.Conn) {
	defer clientConn.Close()
	// Rate Limiter
	host, _, _ := net.SplitHostPort(clientConn.RemoteAddr().String())
	if !lb.rl.Allow(host) {
		log.Println("rate limited: ", host)
		return
	}

	for i := 0; i < len(lb.pool.instances); i++ {
		service := lb.pool.nextHealthyInstance()
		if service == nil {
			break
		}
		serviceConn, err := net.DialTimeout("tcp", service.Address, time.Millisecond*200)
		if err != nil {
			log.Println("error connecting to backend:", service.Address, err)
			continue // try to connect to next backend
		}
		log.Println("connection from ", clientConn.RemoteAddr(), " routed to: ", service.Address)
		proxy(clientConn, serviceConn)
		return
	}
	log.Println("All backends down")
}

func proxy(clientConn net.Conn, serviceConn net.Conn) {
	// clientConn lifecycle is owned by handleConn
	// defer clientConn.Close()
	defer serviceConn.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done() // Safety net if proxy() panics
		io.Copy(serviceConn, clientConn)
	}()
	io.Copy(clientConn, serviceConn)
	serviceConn.Close() // unblocks the goroutine's pending writes - defer close is for safety
	wg.Wait()
}

// Add live metrics endpoint and build a visualization frontend with claude code to find bottlenecks during benchmark
