package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

type Service struct {
	Address string
}

func main() {
	services := []string{
		"localhost:9001",
		"localhost:9002",
		"localhost:9003",
	}

	fmt.Println("Initiated services list")

	var wg sync.WaitGroup

	for _, svc := range services {
		newService := &Service{
			Address: svc,
		}
		wg.Add(1)
		go newService.Create()
		// if err != nil {
		// 	log.Println("Service start failed for:", svc)
		// }
		fmt.Println("Created service: ", svc)
	}
	fmt.Println("All services created - please start load balancer")
	wg.Wait()
}

func (s *Service) Create() error {
	l, err := net.Listen("tcp", s.Address)
	if err != nil {
		return err
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		log.Println("Connection from: ", conn.RemoteAddr())

		go func(c net.Conn) {
			defer c.Close()

			msg := []byte("hello from " + s.Address + "\n")
			c.Write(msg)

			io.Copy(c, c)
		}(conn)
	}
}
