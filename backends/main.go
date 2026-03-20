package backends

import (
	"io"
	"net"
)

type Service struct {
	Address string
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

		go func(c net.Conn) {
			defer c.Close()

			msg := []byte("hello from " + s.Address + "\n")
			c.Write(msg)

			io.Copy(c, c)
		}(conn)
	}
}
