package replay

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecordPluginStoresResponse(t *testing.T) {
	repo := newMemoryRepo()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/path?a=1&b=2", nil)
	key, err := buildKey(req, nil)
	if err != nil {
		t.Fatalf("buildKey: %v", err)
	}

	ctx := &RequestContext{
		Request:    req,
		Key:        key,
		KeyPrefix:  "pfx:",
		Repository: repo,
	}
	stored := &StoredResponse{StatusCode: 200}

	plugin := NewRecordPlugin()
	if err := plugin.OnResponse(ctx, stored); err != nil {
		t.Fatalf("OnResponse: %v", err)
	}

	if _, found, _ := repo.Get(req.Context(), "pfx:"+key); !found {
		t.Fatalf("expected stored response in repo")
	}
}

func TestRecordPluginSkipRule(t *testing.T) {
	repo := newMemoryRepo()
	req := httptest.NewRequest(http.MethodPost, "http://example.com/submit?b=2&a=1", nil)
	key, err := buildKey(req, nil)
	if err != nil {
		t.Fatalf("buildKey: %v", err)
	}
	ctx := &RequestContext{
		Request:    req,
		Key:        key,
		KeyPrefix:  "",
		Repository: repo,
		Body:       []byte("payload=1"),
	}

	plugin := &RecordPlugin{
		BasePlugin: BasePlugin{PluginName: "record"},
		Enable:     true,
		Rules: []*RecordRule{
			{
				Name:      "skip",
				Enable:    true,
				SkipStore: true,
				Match: RequestMatch{
					Method:       []string{http.MethodPost},
					Path:         "/submit",
					URL:          "http://example.com/submit?a=1&b=2",
					BodyContains: "payload",
				},
			},
		},
	}

	if err := plugin.OnResponse(ctx, &StoredResponse{StatusCode: 200}); err != nil {
		t.Fatalf("OnResponse: %v", err)
	}
	if _, found, _ := repo.Get(req.Context(), key); found {
		t.Fatalf("expected response to be skipped")
	}
}
