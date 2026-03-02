package tor

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/proxy"
)

const DefaultProxyAddr = "127.0.0.1:9050"

// NewHTTPClient creates an HTTP client that routes through a SOCKS5 proxy (Tor).
func NewHTTPClient(proxyAddr string) (*http.Client, error) {
	if proxyAddr == "" {
		proxyAddr = DefaultProxyAddr
	}

	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOCKS5 proxy: %w", err)
	}

	transport := &http.Transport{
		DialContext:         dialer.(proxy.ContextDialer).DialContext,
		TLSHandshakeTimeout: 30 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}, nil
}

// CheckConnection tests if the Tor proxy is reachable.
func CheckConnection(proxyAddr string) error {
	if proxyAddr == "" {
		proxyAddr = DefaultProxyAddr
	}

	conn, err := net.DialTimeout("tcp", proxyAddr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("cannot connect to Tor proxy (%s): %w", proxyAddr, err)
	}
	conn.Close()
	return nil
}
