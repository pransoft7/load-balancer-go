package main

import (
	"io"
	"log"
	"net"
	"sync"
)

type Service struct {
	address string
}

type ServicePool struct {
	instances []*Service
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
	var instances []*Service
	// fmt.Println("inside newServicePool")
	for _, a := range addresses {
		newService := &Service{
			address: a,
		}
		// fmt.Println("declared service: ", a)
		go newService.Create(a)
		// fmt.Println("service created for address: ", a)
		instances = append(instances, newService)
	}
	return &ServicePool{
		instances: instances,
		next:      0,
	}
}

func (s *Service) Create(address string) error {
	l, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}

		go func(c net.Conn) {
			io.Copy(c, c)
			c.Close()
		}(conn)
	}
}

// Round robin implementation
func (p *ServicePool) nextInstance() *Service {
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
	serviceConn, err := net.Dial("tcp", service.address)
	if err != nil {
		log.Println("error connecting to service", err)
	}
	log.Println("connection from ", clientConn.RemoteAddr(), " routed to: ", service.address)
	proxy(clientConn, serviceConn)
}

// TODO: Understand and implement properly
func proxy(clientConn net.Conn, serviceConn net.Conn) {
	defer clientConn.Close()
	defer serviceConn.Close()
	go io.Copy(serviceConn, clientConn)
	io.Copy(clientConn, serviceConn)
}
