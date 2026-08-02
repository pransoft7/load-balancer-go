package main

import (
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const failThresholdForBackend int32 = 3

type Service struct {
	Address     string
	FailCounter atomic.Int32
	Healthy     atomic.Bool
}

type ServicePool struct {
	instances []*Service
	next      atomic.Uint64
	// Use when we need to dynamically add/remove backends
	// mu        sync.Mutex
}

type LoadBalancer struct {
	listenAddr string
	pool       *ServicePool
	rl         *RateLimiter
}

func main() {
	rl := NewRateLimiter(
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

	go healthCheck(pool)

	lb := LoadBalancer{
		listenAddr: ":8080",
		pool:       pool,
		rl:         rl,
	}
	log.Println("Starting Load Balancer...")
	lb.Start()
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

// Round robin implementation
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

// TODO: Check the structure and order of process in this func
func (lb *LoadBalancer) Start() error {
	listener, err := net.Listen("tcp", lb.listenAddr)
	if err != nil {
		return err
	}
	// Graceful shutdown of listener
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("LB connection failed", err)
			continue
		}

		go lb.handleConn(conn)
	}

	// return nil
}

func healthCheck(pool *ServicePool) {
	for {
		for _, svc := range pool.instances {
			healthConn, err := net.DialTimeout("tcp", svc.Address, time.Millisecond*500)
			if err != nil {
				if svc.FailCounter.Add(1) > 3 {
					svc.Healthy.Store(false)
				}
				continue
			}
			healthConn.Close()
			svc.FailCounter.Store(0)
			svc.Healthy.Store(true)
		}
		time.Sleep(time.Second)
	}
}

func (lb *LoadBalancer) handleConn(clientConn net.Conn) {
	defer clientConn.Close()
	// Ratelimiter logic
	host, _, _ := net.SplitHostPort(clientConn.RemoteAddr().String())
	if !lb.rl.Allow(host) {
		log.Println("rate limited: ", host)
		clientConn.Close()
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
	serviceConn.Close() // io.Copy goroutine returns -> wg.Done
	wg.Wait()
}

// Add live metrics endpoint and build a visualization frontend with claude code to find bottlenecks during benchmark
