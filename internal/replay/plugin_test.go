package replay

import (
	"errors"
	"testing"
)

type testPlugin struct {
	name       string
	onRequest  func(*RequestContext) error
	onResponse func(*RequestContext, *StoredResponse) error
}

func (p testPlugin) Name() string {
	return p.name
}

func (p testPlugin) OnRequest(ctx *RequestContext) error {
	if p.onRequest == nil {
		return nil
	}
	return p.onRequest(ctx)
}

func (p testPlugin) OnResponse(ctx *RequestContext, stored *StoredResponse) error {
	if p.onResponse == nil {
		return nil
	}
	return p.onResponse(ctx, stored)
}

func TestApplyRequestPluginsOrder(t *testing.T) {
	steps := make([]string, 0, 2)
	plugins := []Plugin{
		testPlugin{
			name: "first",
			onRequest: func(ctx *RequestContext) error {
				ctx.Key = "first"
				steps = append(steps, ctx.Key)
				return nil
			},
		},
		testPlugin{
			name: "second",
			onRequest: func(ctx *RequestContext) error {
				ctx.Key = ctx.Key + "-second"
				steps = append(steps, ctx.Key)
				return nil
			},
		},
	}

	ctx := &RequestContext{Key: "start"}
	if err := applyRequestPlugins(plugins, ctx); err != nil {
		t.Fatalf("applyRequestPlugins: %v", err)
	}
	if ctx.Key != "first-second" {
		t.Fatalf("unexpected key: %s", ctx.Key)
	}
	if len(steps) != 2 || steps[0] != "first" || steps[1] != "first-second" {
		t.Fatalf("unexpected steps: %#v", steps)
	}
}

func TestPluginErrorStatus(t *testing.T) {
	plugins := []Plugin{
		testPlugin{
			name: "bad",
			onRequest: func(_ *RequestContext) error {
				return PluginError{Status: 418, Err: errors.New("nope")}
			},
		},
	}
	ctx := &RequestContext{}
	err := applyRequestPlugins(plugins, ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := statusFromPluginError(err); got != 418 {
		t.Fatalf("unexpected status: %d", got)
	}
}

func TestApplyResponsePlugins(t *testing.T) {
	plugins := []Plugin{
		testPlugin{
			name: "mutate",
			onResponse: func(_ *RequestContext, stored *StoredResponse) error {
				stored.StatusCode = 201
				stored.Headers = append(stored.Headers, Header{Key: "X-Test", Value: "ok"})
				return nil
			},
		},
	}
	stored := StoredResponse{StatusCode: 200}
	if err := applyResponsePlugins(plugins, &RequestContext{}, &stored); err != nil {
		t.Fatalf("applyResponsePlugins: %v", err)
	}
	if stored.StatusCode != 201 {
		t.Fatalf("unexpected status: %d", stored.StatusCode)
	}
	if len(stored.Headers) != 1 || stored.Headers[0].Key != "X-Test" {
		t.Fatalf("unexpected headers: %#v", stored.Headers)
	}
}
