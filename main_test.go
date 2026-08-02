package main

import (
	"bytes"
	"net"
	"testing"
)

func TestNextInstanceReturnsBackendInRoundRobinOrder(t *testing.T) {
	pool := newServicePool([]string{
		"A", "B", "C",
	})

	var got []string

	for i := 0; i < 6; i++ {
		svc := pool.nextHealthyInstance()
		got = append(got, svc.Address)
	}

	want := []string{
		"A",
		"B",
		"C",
		"A",
		"B",
		"C",
	}

	for i := range want {
		if want[i] != got[i] {
			t.Fatal("unexpected round robin order of backends")
		}
	}
}

func TestProxyClientToBackend(t *testing.T) {
	clientLB, client := net.Pipe()
	backendLB, backend := net.Pipe()

	defer client.Close()
	defer backend.Close()

	msg1 := []byte("hello")

	go proxy(clientLB, backendLB)
	_, err := client.Write(msg1)
	if err != nil {
		t.Fatal("write from client failed")
	}
	readFromBackend := make([]byte, 5)
	n, err := backend.Read(readFromBackend)
	if err != nil {
		t.Fatal("read to backend failed")
	}
	if !bytes.Equal(readFromBackend[:n], msg1) {
		t.Fatal("expected: hello, got: ", string(readFromBackend[:n]))
	}
}

func TestProxyBackendToClient(t *testing.T) {
	clientLB, client := net.Pipe()
	backendLB, backend := net.Pipe()

	defer client.Close()
	defer backend.Close()

	msg2 := []byte("world")
	go proxy(clientLB, backendLB)
	_, err := backend.Write([]byte("world"))
	if err != nil {
		t.Fatal("write from backend failed")
	}
	readFromClient := make([]byte, 5)
	n, err := client.Read(readFromClient)
	if err != nil {
		t.Fatal("read to client failed")
	}
	if !bytes.Equal(readFromClient[:n], msg2) {
		t.Fatal("expected: world, got: ", string(readFromClient[:n]))
	}
}
