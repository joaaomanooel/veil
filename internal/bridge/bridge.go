package bridge

import (
	"errors"
	"io"
	"net"
)

// Serve relays every accepted connection to a fresh connection from dial,
// copying bytes both ways. It returns nil once ln is closed.
func Serve(ln net.Listener, dial func() (net.Conn, error)) error {
	for {
		conn, err := ln.Accept()
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		if err != nil {
			return err
		}
		go relay(conn, dial)
	}
}

func relay(client net.Conn, dial func() (net.Conn, error)) {
	defer client.Close()
	upstream, err := dial()
	if err != nil {
		return
	}
	defer upstream.Close()
	done := make(chan struct{}, 2)
	go pipe(upstream, client, done)
	go pipe(client, upstream, done)
	<-done
	<-done
}

func pipe(dst, src net.Conn, done chan<- struct{}) {
	_, _ = io.Copy(dst, src)
	if half, ok := dst.(interface{ CloseWrite() error }); ok {
		_ = half.CloseWrite()
	} else {
		_ = dst.Close()
	}
	done <- struct{}{}
}
