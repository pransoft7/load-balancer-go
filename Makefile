.PHONY: run backends benchmark test build

run:
	go run ./cmd/loadbalancer

backends:
	go run ./backends

benchmark:
	go run ./benchmark

build:
	go build ./...

test:
	go test ./...
