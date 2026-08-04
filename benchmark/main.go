package main

import (
	"fmt"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

const (
	concurrentConns int    = 1000
	server          string = "127.0.0.1:8080"
)

var latens []int64
var latensMu sync.Mutex
var success atomic.Int32
var failure atomic.Int32

func main() {
	startTime := time.Now()
	wg := sync.WaitGroup{}
	for i := 0; i < concurrentConns; i++ {
		wg.Add(1)
		go oneLoad(&wg)
	}
	wg.Wait()

	totalTime := time.Since(startTime)
	throughput := float64(success.Load()) / float64(totalTime.Seconds())
	fmt.Println("Throughput: ", throughput, " requests per second")
	fmt.Println("Success: ", success.Load())
	fmt.Println("Failure: ", failure.Load())

	// Compute latencies
	slices.Sort(latens)
	p50 := latens[len(latens)*50/100]
	p95 := latens[len(latens)*95/100]
	p99 := latens[len(latens)*99/100]

	fmt.Println("p50 latency:", p50)
	fmt.Println("p95 latency:", p95)
	fmt.Println("p99 latency:", p99)
}

func oneLoad(wg *sync.WaitGroup) {
	defer wg.Done()
	latStart := time.Now()
	conn, err := net.Dial("tcp", server)
	if err != nil {
		failure.Add(1)
		return
	}
	defer conn.Close()

	buffer := make([]byte, 1024)
	_, err = conn.Read(buffer)
	if err != nil {
		failure.Add(1)
		return
	}

	_, err = conn.Write([]byte("Hello"))
	if err != nil {
		failure.Add(1)
		return
	}
	m, err := conn.Read(buffer)
	if err != nil {
		failure.Add(1)
		return
	}

	if string(buffer[:m]) != "Hello" {
		failure.Add(1)
		return
	}
	latency := time.Since(latStart).Microseconds()
	latensMu.Lock()
	latens = append(latens, latency)
	latensMu.Unlock()
	success.Add(1)
}
