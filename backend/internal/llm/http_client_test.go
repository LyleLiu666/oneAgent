package llm

import (
	"net/http"
	"testing"
	"time"
)

func TestNewHTTPClient_DoesNotSetWholeRequestTimeout(t *testing.T) {
	c := newHTTPClient(0)
	if c.Timeout != 0 {
		t.Fatalf("expected client.Timeout=0 for streaming-safe behavior, got %v", c.Timeout)
	}

	tr, ok := c.Transport.(*http.Transport)
	if !ok || tr == nil {
		t.Fatalf("expected Transport to be *http.Transport, got %#v", c.Transport)
	}
	if tr.ResponseHeaderTimeout != 120*time.Second {
		t.Fatalf("expected default ResponseHeaderTimeout=120s, got %v", tr.ResponseHeaderTimeout)
	}
}

func TestNewHTTPClient_UsesResponseHeaderTimeout(t *testing.T) {
	c := newHTTPClient(15 * time.Second)
	if c.Timeout != 0 {
		t.Fatalf("expected client.Timeout=0, got %v", c.Timeout)
	}

	tr, ok := c.Transport.(*http.Transport)
	if !ok || tr == nil {
		t.Fatalf("expected Transport to be *http.Transport, got %#v", c.Transport)
	}
	if tr.ResponseHeaderTimeout != 15*time.Second {
		t.Fatalf("expected ResponseHeaderTimeout=15s, got %v", tr.ResponseHeaderTimeout)
	}
}

