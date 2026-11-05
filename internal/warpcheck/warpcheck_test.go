package warpcheck

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func traceServer(t *testing.T, body string) (*httptest.Server, *tls.Config) {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	}))
	t.Cleanup(server.Close)
	return server, server.Client().Transport.(*http.Transport).TLSClientConfig
}

func connectProxy(t *testing.T, status int) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			client, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer client.Close()
				request, err := http.ReadRequest(bufio.NewReader(client))
				if err != nil || request.Method != http.MethodConnect {
					return
				}
				if status != http.StatusOK {
					fmt.Fprintf(client, "HTTP/1.1 %d Blocked\r\nContent-Length: 0\r\n\r\n", status)
					return
				}
				upstream, err := net.Dial("tcp", request.Host)
				if err != nil {
					return
				}
				defer upstream.Close()
				fmt.Fprint(client, "HTTP/1.1 200 Connection established\r\n\r\n")
				go io.Copy(upstream, client)
				io.Copy(client, upstream)
			}()
		}
	}()
	return ln.Addr().String()
}

func TestVerify_ReturnsTheDialedGatewayAddressWhenTheTraceSaysWarpOn(t *testing.T) {
	// Arrange
	server, tlsConfig := traceServer(t, "fl=1\nh=www.cloudflare.com\nwarp=on\n")
	gateway := connectProxy(t, http.StatusOK)

	// Act
	pinned, err := Verify(gateway, server.URL+"/cdn-cgi/trace", tlsConfig)

	// Assert
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if pinned != gateway {
		t.Fatalf("got pinned address %q, want %q", pinned, gateway)
	}
}

func TestVerify_RejectsAGatewayWhoseEgressIsNotWarp(t *testing.T) {
	// Arrange
	server, tlsConfig := traceServer(t, "fl=1\nwarp=off\n")
	gateway := connectProxy(t, http.StatusOK)

	// Act
	_, err := Verify(gateway, server.URL+"/cdn-cgi/trace", tlsConfig)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "gateway is not WARP") {
		t.Fatalf("expected a not-WARP error, got %v", err)
	}
}

func TestVerify_RejectsAGatewayThatBlocksTheTraceEndpoint(t *testing.T) {
	// Arrange
	server, tlsConfig := traceServer(t, "warp=on\n")
	gateway := connectProxy(t, http.StatusForbidden)

	// Act
	_, err := Verify(gateway, server.URL+"/cdn-cgi/trace", tlsConfig)

	// Assert
	if err == nil {
		t.Fatal("expected an error when the gateway blocks the trace endpoint")
	}
}

func TestVerify_RejectsATraceServedWithAnUntrustedCertificate(t *testing.T) {
	// Arrange
	server, _ := traceServer(t, "warp=on\n")
	gateway := connectProxy(t, http.StatusOK)

	// Act
	_, err := Verify(gateway, server.URL+"/cdn-cgi/trace", nil)

	// Assert
	if err == nil {
		t.Fatal("expected a certificate error for a forged trace endpoint")
	}
}
