package replay

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMapRemoteItemMatch(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://example.com/path/to/resource", nil)
	req.Host = "example.com"
	ctx := &RequestContext{Request: req}

	item := &mapRemoteItem{
		From: &mapFrom{
			Protocol: "https",
			Host:     "example.com",
			Method:   []string{"GET", "POST"},
			Path:     "/path/to/resource",
		},
		To:     nil,
		Enable: true,
	}
	if !item.match(ctx) {
		t.Fatal("expected match")
	}

	item.From = &mapFrom{
		Protocol: "",
		Host:     "example.com",
		Method:   []string{},
		Path:     "/path/to/resource",
	}
	if !item.match(ctx) {
		t.Fatal("expected match with empty protocol/method")
	}

	item.From = &mapFrom{
		Protocol: "",
		Host:     "",
		Method:   []string{},
		Path:     "/path/to/*",
	}
	if !item.match(ctx) {
		t.Fatal("expected match with wildcard path")
	}

	item.From = &mapFrom{
		Protocol: "",
		Host:     "",
		Method:   []string{},
		Path:     "",
	}
	if !item.match(ctx) {
		t.Fatal("expected match with empty selectors")
	}

	item.From = &mapFrom{
		Protocol: "http",
		Host:     "example.com",
		Method:   []string{},
		Path:     "/path/to/resource",
	}
	if item.match(ctx) {
		t.Fatal("expected protocol mismatch")
	}
}

func TestMapRemoteItemReplace(t *testing.T) {
	newCtx := func() *RequestContext {
		req := httptest.NewRequest(http.MethodGet, "https://example.com/path/to/resource", nil)
		req.Host = "example.com"
		return &RequestContext{Request: req}
	}

	item := &mapRemoteItem{
		From: &mapFrom{
			Protocol: "https",
			Host:     "example.com",
			Method:   []string{"GET", "POST"},
			Path:     "/path/to/resource",
		},
		To: &mapRemoteTo{
			Protocol: "http",
			Host:     "hello.com",
		},
		Enable: true,
	}
	ctx := newCtx()
	if err := item.replace(ctx); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := ctx.Request.URL.String(); got != "http://hello.com/path/to/resource" {
		t.Fatalf("unexpected url: %s", got)
	}

	item = &mapRemoteItem{
		From: &mapFrom{
			Protocol: "https",
			Host:     "example.com",
			Method:   []string{"GET", "POST"},
			Path:     "/path/to/resource",
		},
		To: &mapRemoteTo{
			Protocol: "http",
			Host:     "hello.com",
			Path:     "/path/to/world",
		},
		Enable: true,
	}
	ctx = newCtx()
	if err := item.replace(ctx); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := ctx.Request.URL.String(); got != "http://hello.com/path/to/world" {
		t.Fatalf("unexpected url: %s", got)
	}

	item = &mapRemoteItem{
		From: &mapFrom{
			Protocol: "https",
			Host:     "example.com",
			Method:   []string{"GET", "POST"},
			Path:     "/path/to/*",
		},
		To: &mapRemoteTo{
			Protocol: "http",
			Host:     "hello.com",
			Path:     "/world",
		},
		Enable: true,
	}
	ctx = newCtx()
	if err := item.replace(ctx); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := ctx.Request.URL.String(); got != "http://hello.com/world/resource" {
		t.Fatalf("unexpected url: %s", got)
	}
	if ctx.Key == "" {
		t.Fatal("expected key to be set")
	}
}
