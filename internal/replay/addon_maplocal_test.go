package replay

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestMapLocalResponseFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(file, []byte("hello"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	ml := &MapLocal{
		BasePlugin: BasePlugin{PluginName: "map-local"},
		Enable:     true,
		Items: []*mapLocalItem{
			{
				From:   &mapFrom{Path: "/hello.txt"},
				To:     &mapLocalTo{Path: file},
				Enable: true,
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/hello.txt", nil)
	ctx := &RequestContext{Request: req}
	if err := ml.OnRequest(ctx); err != nil {
		t.Fatalf("OnRequest: %v", err)
	}
	if ctx.Response == nil || ctx.Response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected response: %#v", ctx.Response)
	}
	body, err := base64.StdEncoding.DecodeString(ctx.Response.BodyBase64)
	if err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if string(body) != "hello" {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestMapLocalResponseDir(t *testing.T) {
	dir := t.TempDir()
	staticDir := filepath.Join(dir, "static")
	file := filepath.Join(staticDir, "asset.txt")
	if err := os.MkdirAll(staticDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(file, []byte("asset"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	ml := &MapLocal{
		BasePlugin: BasePlugin{PluginName: "map-local"},
		Enable:     true,
		Items: []*mapLocalItem{
			{
				From:   &mapFrom{Path: "/static/*"},
				To:     &mapLocalTo{Path: staticDir},
				Enable: true,
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/static/asset.txt", nil)
	ctx := &RequestContext{Request: req}
	if err := ml.OnRequest(ctx); err != nil {
		t.Fatalf("OnRequest: %v", err)
	}
	if ctx.Response == nil || ctx.Response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected response: %#v", ctx.Response)
	}
}

func TestMapLocalNotFound(t *testing.T) {
	ml := &MapLocal{
		BasePlugin: BasePlugin{PluginName: "map-local"},
		Enable:     true,
		Items: []*mapLocalItem{
			{
				From:   &mapFrom{Path: "/missing"},
				To:     &mapLocalTo{Path: filepath.Join(t.TempDir(), "missing")},
				Enable: true,
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/missing", nil)
	ctx := &RequestContext{Request: req}
	if err := ml.OnRequest(ctx); err != nil {
		t.Fatalf("OnRequest: %v", err)
	}
	if ctx.Response == nil || ctx.Response.StatusCode != http.StatusNotFound {
		t.Fatalf("unexpected response: %#v", ctx.Response)
	}
}
