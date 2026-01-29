package replay

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewStructFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"value":"ok"}`), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var payload struct {
		Value string `json:"value"`
	}
	if err := newStructFromFile(path, &payload); err != nil {
		t.Fatalf("newStructFromFile: %v", err)
	}
	if payload.Value != "ok" {
		t.Fatalf("unexpected value: %s", payload.Value)
	}
}
