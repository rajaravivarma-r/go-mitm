package replay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSortQueryParams(t *testing.T) {
	got, err := sortQueryParams("b=2&a=1&a=0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "a=0&a=1&b=2"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildKeyJSON(t *testing.T) {
	body := `{"b":2,"a":1}`
	req := httptest.NewRequest(http.MethodPost, "http://example.com/alpha?b=2&a=1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	key, err := buildKey(req, []byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `/alpha|POST|a=1&b=2|{"a":1,"b":2}`
	if key != want {
		t.Fatalf("got %q want %q", key, want)
	}
}

func TestBuildKeyFormBody(t *testing.T) {
	body := "b=2&a=1&a=0"
	req := httptest.NewRequest(http.MethodPost, "http://example.com/form", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	key, err := buildKey(req, []byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/form|POST||a=0&a=1&b=2"
	if key != want {
		t.Fatalf("got %q want %q", key, want)
	}
}

func TestCanonicalJSONASCII(t *testing.T) {
	body := `{"name":"café"}`
	got, err := canonicalJSON([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `{"name":"caf\u00e9"}`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
