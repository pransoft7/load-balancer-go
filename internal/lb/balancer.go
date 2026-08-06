package lb

import (
	"context"
	"load-balancer-go/ratelimiter"
	"log"
	"net"
	"sync"
	"time"
)

const shutdownTimeout = 30 * time.Second

type LoadBalancer struct {
	listenAddr string
	pool       *ServicePool
	rl         *ratelimiter.RateLimiter
}

func NewLoadBalancer(listenAddr string, addrs []string, rl *ratelimiter.RateLimiter) *LoadBalancer {
	return &LoadBalancer{
		listenAddr: listenAddr,
		pool:       newServicePool(addrs),
		rl:         rl,
	}
}

func (lb *LoadBalancer) Start(ctx context.Context) error {
	// Health Check goroutine
	go lb.pool.healthCheck(ctx)

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
