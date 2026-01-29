package replay

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type memoryRepo struct {
	data       map[string]StoredResponse
	getCalls   int
	setCalls   int
	closeCalls int
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{data: make(map[string]StoredResponse)}
}

func (m *memoryRepo) Get(_ context.Context, key string) (StoredResponse, bool, error) {
	m.getCalls++
	value, ok := m.data[key]
	return value, ok, nil
}

func (m *memoryRepo) Set(_ context.Context, key string, value StoredResponse, overwrite bool) error {
	m.setCalls++
	if !overwrite {
		if _, ok := m.data[key]; ok {
			return nil
		}
	}
	m.data[key] = value
	return nil
}

func (m *memoryRepo) Close() error {
	m.closeCalls++
	return nil
}

func TestServerRequestPluginError(t *testing.T) {
	repo := newMemoryRepo()
	router := NewReplayRouter(repo, ServerOptions{
		Plugins: []Plugin{
			testPlugin{
				name: "fail",
				onRequest: func(*RequestContext) error {
					return PluginError{Status: http.StatusTooManyRequests, Err: errors.New("blocked")}
				},
			},
		},
	})

	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/hello")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}

func TestServerRequestPluginResponse(t *testing.T) {
	repo := newMemoryRepo()
	body := base64.StdEncoding.EncodeToString([]byte("ok"))
	router := NewReplayRouter(repo, ServerOptions{
		Plugins: []Plugin{
			testPlugin{
				name: "short-circuit",
				onRequest: func(ctx *RequestContext) error {
					ctx.Response = &StoredResponse{
						StatusCode: http.StatusCreated,
						BodyBase64: body,
					}
					return nil
				},
			},
		},
	})

	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/hello")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	payload, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(payload, []byte("ok")) {
		t.Fatalf("unexpected body: %s", payload)
	}
	if repo.getCalls != 0 {
		t.Fatalf("expected no repository lookups, got %d", repo.getCalls)
	}
}
