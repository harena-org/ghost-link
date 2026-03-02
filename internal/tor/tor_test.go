package tor

import (
	"testing"
)

func TestCheckConnectionNoTor(t *testing.T) {
	// Tor is likely not running in test environment, so this should fail
	err := CheckConnection("127.0.0.1:19999")
	if err == nil {
		t.Error("expected error when Tor is not running")
	}
}

func TestNewHTTPClientDefaultProxy(t *testing.T) {
	client, err := NewHTTPClient("")
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewHTTPClientCustomProxy(t *testing.T) {
	client, err := NewHTTPClient("127.0.0.1:9050")
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}
