package warpcheck

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// TraceURL is the Cloudflare endpoint that reports warp=on only for traffic leaving through WARP.
const TraceURL = "https://www.cloudflare.com/cdn-cgi/trace"

func Verify(gateway, traceURL string, tlsConfig *tls.Config) (string, error) {
	var pinned string
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: gateway}),
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				conn, err := dialer.DialContext(ctx, network, address)
				if err == nil {
					pinned = conn.RemoteAddr().String()
				}
				return conn, err
			},
			TLSClientConfig:   tlsConfig,
			DisableKeepAlives: true,
		},
	}
	response, err := client.Get(traceURL)
	if err != nil {
		return "", fmt.Errorf("cannot reach the WARP trace through the gateway: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("WARP trace answered %s through the gateway", response.Status)
	}
	scanner := bufio.NewScanner(io.LimitReader(response.Body, 4096))
	for scanner.Scan() {
		if scanner.Text() == "warp=on" {
			return pinned, nil
		}
	}
	return "", errors.New("gateway is not WARP: the Cloudflare trace did not report warp=on")
}
