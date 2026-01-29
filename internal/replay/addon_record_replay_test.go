package replay

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecordReplayAlwaysUpstreamRule(t *testing.T) {
	rr := &RecordReplay{
		BasePlugin: BasePlugin{PluginName: "record-replay"},
		Enable:     true,
		Rules: []*RecordReplayRule{
			{
				Name:           "force-upstream",
				Enable:         true,
				AlwaysUpstream: true,
				Match: RecordReplayMatch{
					Path: "/api/*",
				},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/test", nil)
	ctx := &RequestContext{Request: req}
	if err := rr.OnRequest(ctx); err != nil {
		t.Fatalf("OnRequest: %v", err)
	}
	if !ctx.SkipCache || !ctx.SkipStore {
		t.Fatalf("expected skip cache/store, got cache=%t store=%t", ctx.SkipCache, ctx.SkipStore)
	}
}

func TestRecordReplayHeaderBodyMatch(t *testing.T) {
	rr := &RecordReplay{
		BasePlugin: BasePlugin{PluginName: "record-replay"},
		Enable:     true,
		Rules: []*RecordReplayRule{
			{
				Name:      "skip-store",
				Enable:    true,
				SkipStore: true,
				Match: RecordReplayMatch{
					Method:       []string{http.MethodPost},
					Path:         "/submit",
					Header:       map[string]string{"X-Test": "abc"},
					BodyContains: "payload",
				},
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "http://example.com/submit", nil)
	req.Header.Set("X-Test", "abc-123")
	ctx := &RequestContext{Request: req, Body: []byte("payload=1")}
	if err := rr.OnRequest(ctx); err != nil {
		t.Fatalf("OnRequest: %v", err)
	}
	if ctx.SkipCache {
		t.Fatalf("expected cache to remain enabled")
	}
	if !ctx.SkipStore {
		t.Fatalf("expected store to be skipped")
	}
}
