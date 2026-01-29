package replay

import (
	"net/http"
	"testing"
	"time"
)

func TestNewUpstreamClientValidation(t *testing.T) {
	if _, err := NewUpstreamClient("", time.Second); err == nil {
		t.Fatal("expected error for empty upstream URL")
	}
	if _, err := NewUpstreamClient("example.com", time.Second); err == nil {
		t.Fatal("expected error for missing scheme")
	}
}

func TestCloneRequestHeaders(t *testing.T) {
	headers := http.Header{
		"Connection":      []string{"keep-alive"},
		"Keep-Alive":      []string{"timeout=5"},
		"Content-Length":  []string{"5"},
		"Host":            []string{"example.com"},
		"Accept-Encoding": []string{"gzip"},
		"X-Test":          []string{"ok"},
	}
	cloned := cloneRequestHeaders(headers)
	if cloned.Get("Host") != "" {
		t.Fatal("expected Host to be stripped")
	}
	if cloned.Get("Content-Length") != "" {
		t.Fatal("expected Content-Length to be stripped")
	}
	if cloned.Get("Connection") != "" || cloned.Get("Keep-Alive") != "" {
		t.Fatal("expected hop-by-hop headers to be stripped")
	}
	if cloned.Get("Accept-Encoding") != "identity" {
		t.Fatalf("unexpected Accept-Encoding: %s", cloned.Get("Accept-Encoding"))
	}
	if cloned.Get("X-Test") != "ok" {
		t.Fatalf("expected X-Test header preserved")
	}
}
