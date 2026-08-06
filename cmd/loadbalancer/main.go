package main

import (
	"context"
	"load-balancer-go/internal/lb"
	"load-balancer-go/ratelimiter"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	rl := ratelimiter.NewRateLimiter(
		10000,
		10,
		100*time.Second,
	)

	services := []string{
		"localhost:9001",
		"localhost:9002",
		"localhost:9003",
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	balancer := lb.NewLoadBalancer(":8080", services, rl)
	log.Println("Starting Load Balancer...")

	if err := balancer.Start(ctx); err != nil {
		log.Fatal(err)
	}
}
