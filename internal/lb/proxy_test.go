package lb

import (
	"bytes"
	"net"
	"testing"
)

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
