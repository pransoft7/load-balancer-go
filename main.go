package main

import (
	"io"
	"load-balancer-go/backends"
	"log"
	"net"
	"sync"
)

type ServicePool struct {
	instances []*backends.Service
	next      int
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
	// fmt.Println("after services declaration!")
	pool := newServicePool(services)
	// fmt.Println("Created service pool with services")

	lb := LoadBalancer{
		listenAddr: ":8080",
		pool:       pool,
	}
	log.Println("Starting Load Balancer...")
	lb.Start()
}

func newServicePool(addresses []string) *ServicePool {
	var instances []*backends.Service
	// fmt.Println("inside newServicePool")
	for _, a := range addresses {
		newService := &backends.Service{
			Address: a,
		}
		// fmt.Println("declared service: ", a)
		go newService.Create()
		// fmt.Println("service created for address: ", a)
		instances = append(instances, newService)
	}
	return &ServicePool{
		instances: instances,
		next:      0,
	}
}

// Round robin implementation
func (p *ServicePool) nextInstance() *backends.Service {
	p.mu.Lock()
	index := p.next % len(p.instances)
	p.next++
	p.mu.Unlock()

	return p.instances[index]
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
			log.Println("Service connection failed", err)
			conn.Close()
			continue
		}

		go lb.handleConn(conn)
	}

	// return nil
}

func (lb *LoadBalancer) handleConn(clientConn net.Conn) {
	service := lb.pool.nextInstance()
	serviceConn, err := net.Dial("tcp", service.Address)
	if err != nil {
		log.Println("error connecting to service", err)
	}
	log.Println("connection from ", clientConn.RemoteAddr(), " routed to: ", service.Address)
	proxy(clientConn, serviceConn)
}

// TODO: Understand and implement properly
func proxy(clientConn net.Conn, serviceConn net.Conn) {
	defer clientConn.Close()
	defer serviceConn.Close()
	go io.Copy(serviceConn, clientConn)
	io.Copy(clientConn, serviceConn)
}
