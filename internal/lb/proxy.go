package lb

import (
	"io"
	"net"
	"sync"
)

func proxy(clientConn net.Conn, serviceConn net.Conn) {
	// clientConn lifecycle is owned by handleConn
	// defer clientConn.Close()
	defer serviceConn.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done() // Safety net if proxy() panics
		io.Copy(serviceConn, clientConn)
	}()
	io.Copy(clientConn, serviceConn)
	serviceConn.Close() // unblocks the goroutine's pending writes - defer close is for safety
	wg.Wait()
}
