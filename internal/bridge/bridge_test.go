package bridge

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func listenUnix(t *testing.T) net.Listener {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "veil-")
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", filepath.Join(dir, "proxy.sock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close(); os.RemoveAll(dir) })
	return ln
}

func TestServe_CopiesBothDirections(t *testing.T) {
	// Arrange
	echo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	go func() {
		conn, err := echo.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(conn, conn)
	}()
	ln := listenUnix(t)
	go Serve(ln, func() (net.Conn, error) { return net.Dial("tcp", echo.Addr().String()) })
	client, err := net.Dial("unix", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// Act
	_, err = client.Write([]byte("ping"))
	if err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 4)
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err = io.ReadFull(client, reply)

	// Assert
	if err != nil || string(reply) != "ping" {
		t.Fatalf("got %q, %v; want ping", reply, err)
	}
}

func TestServe_ClosesClientWhenDialFails(t *testing.T) {
	// Arrange
	ln := listenUnix(t)
	go Serve(ln, func() (net.Conn, error) { return nil, errors.New("gateway down") })
	client, err := net.Dial("unix", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// Act
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	_, err = client.Read(make([]byte, 1))

	// Assert
	if !errors.Is(err, io.EOF) {
		t.Fatalf("got %v, want EOF", err)
	}
}

func TestServe_ReturnsNilWhenListenerCloses(t *testing.T) {
	// Arrange
	ln := listenUnix(t)
	done := make(chan error, 1)
	go func() { done <- Serve(ln, func() (net.Conn, error) { return nil, errors.New("unused") }) }()

	// Act
	ln.Close()

	// Assert
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("got %v, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not return after the listener closed")
	}
}
