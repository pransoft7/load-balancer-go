package main

import (
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Service struct {
	Address     string
	FailCounter int
	Healthy     bool
}

type ServicePool struct {
	instances []*Service
	next      atomic.Uint64
	mu        sync.Mutex
}

type LoadBalancer struct {
	listenAddr string
	pool       *ServicePool
}

func main() {
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
	}
	log.Println("Starting Load Balancer...")
	lb.Start()
}

func newServicePool(addresses []string) *ServicePool {
	var instances []*Service
	for _, a := range addresses {
		newService := &Service{
			Address:     a,
			FailCounter: 0,
			Healthy:     true,
		}
		instances = append(instances, newService)
	}
	return &ServicePool{
		instances: instances,
		next:      atomic.Uint64{},
	}
}

// Round robin implementation
func (p *ServicePool) nextInstance() *Service {
	index := p.next.Add(1)
	return p.instances[index%uint64(len(p.instances))]
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
				log.Println("Health check failed to backend: ", svc.Address)
				svc.FailCounter++
				if svc.FailCounter > 3 {
					svc.Healthy = false

				}
				continue
			}
			healthConn.Close()
			log.Println("Service ", svc.Address, " is healthy!")
			svc.FailCounter = 0
			svc.Healthy = true
		}
		time.Sleep(time.Second)
	}
}

func (lb *LoadBalancer) handleConn(clientConn net.Conn) {
	defer clientConn.Close()
	// TODO: Insert rateLimiter logic here!

	// Loop retries all backends if one fails
	for i := 0; i < len(lb.pool.instances); i++ {
		service := lb.pool.nextInstance()
		if service.Healthy {
			serviceConn, err := net.DialTimeout("tcp", service.Address, time.Millisecond*200)
			if err != nil {
				log.Println("error connecting to service", service.Address, err)
				service.FailCounter++ // This will lead to race
				if i == len(lb.pool.instances)-1 {
					log.Println("All services are down!")
				}
				continue
			}
			log.Println("connection from ", clientConn.RemoteAddr(), " routed to: ", service.Address)
			proxy(clientConn, serviceConn)
			break
		} else {
			continue
		}
	}
}

func proxy(clientConn net.Conn, serviceConn net.Conn) {
	defer clientConn.Close()
	defer serviceConn.Close()

	go io.Copy(serviceConn, clientConn)
	io.Copy(clientConn, serviceConn)
}
