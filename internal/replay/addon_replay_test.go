package replay

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReplayPluginQueryOrderMatch(t *testing.T) {
	repo := newMemoryRepo()
	reqStored := httptest.NewRequest(http.MethodGet, "http://example.com/path?b=2&a=1", nil)
	keyStored, err := buildKey(reqStored, nil)
	if err != nil {
		t.Fatalf("buildKey stored: %v", err)
	}
	stored := StoredResponse{StatusCode: 200}
	if err := repo.Set(reqStored.Context(), keyStored, stored, true); err != nil {
		t.Fatalf("repo.Set: %v", err)
	}

	reqReplay := httptest.NewRequest(http.MethodGet, "http://example.com/path?a=1&b=2", nil)
	keyReplay, err := buildKey(reqReplay, nil)
	if err != nil {
		t.Fatalf("buildKey replay: %v", err)
	}
	ctx := &RequestContext{
		Request:    reqReplay,
		Key:        keyReplay,
		KeyPrefix:  "",
		Repository: repo,
	}
	plugin := NewReplayPlugin()
	if err := plugin.OnRequest(ctx); err != nil {
		t.Fatalf("OnRequest: %v", err)
	}
	if ctx.Response == nil || !ctx.CacheHit {
		t.Fatalf("expected cached response, got %#v", ctx.Response)
	}
}

func TestReplayPluginSkipRule(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/path?b=2&a=1", nil)
	key, err := buildKey(req, nil)
	if err != nil {
		t.Fatalf("buildKey: %v", err)
	}
	ctx := &RequestContext{
		Request:   req,
		Key:       key,
		KeyPrefix: "",
	}

	plugin := &ReplayPlugin{
		BasePlugin: BasePlugin{PluginName: "replay"},
		Enable:     true,
		Rules: []*ReplayRule{
			{
				Name:       "skip",
				Enable:     true,
				SkipReplay: true,
				Match: RequestMatch{
					URL: "http://example.com/path?a=1&b=2",
				},
			},
		},
	}

	if err := plugin.OnRequest(ctx); err != nil {
		t.Fatalf("OnRequest: %v", err)
	}
	if !ctx.SkipCache {
		t.Fatalf("expected skip cache to be set")
	}
}
