package replay

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestReplaySampleFlowSQLite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	mitmdumpPath, err := exec.LookPath("mitmdump")
	if err != nil {
		t.Skip("mitmdump not found in PATH")
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve test file location")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	samplePath := filepath.Join(repoRoot, "testdata", "sample.flow")
	if _, err := os.Stat(samplePath); err != nil {
		t.Fatalf("sample flow file missing: %v", err)
	}
	samplePath, err = filepath.Abs(samplePath)
	if err != nil {
		t.Fatalf("sample flow absolute path: %v", err)
	}

	scriptPath := os.Getenv("MITM_DUMP_SCRIPT")
	if scriptPath == "" {
		t.Skip("MITM_DUMP_SCRIPT not set")
	}
	if _, err := os.Stat(scriptPath); err != nil {
		t.Skipf("dump script missing: %v", err)
	}
	scriptPath, err = filepath.Abs(scriptPath)
	if err != nil {
		t.Fatalf("dump script absolute path: %v", err)
	}

	sqlitePath := filepath.Join(t.TempDir(), "flows.sqlite")
	loadFlowWithMitmDump(t, mitmdumpPath, scriptPath, samplePath, sqlitePath)

	repository, err := NewSQLiteRepository(sqlitePath, 5*time.Second)
	if err != nil {
		t.Fatalf("open sqlite repository: %v", err)
	}
	defer repository.Close()

	router := NewReplayRouter(repository, ServerOptions{})
	server := httptest.NewServer(router)
	defer server.Close()

	assertSampleFlowResponse(t, http.MethodPost, server.URL+"/post?name=hello", "https://httpbin.org/post?name=hello")
	assertSampleFlowResponse(t, http.MethodGet, server.URL+"/get?name=hello", "https://httpbin.org/get?name=hello")
}

func loadFlowWithMitmDump(t *testing.T, mitmdumpPath, scriptPath, samplePath, sqlitePath string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, mitmdumpPath, "-s", scriptPath, "-n")
	cmd.Env = append(os.Environ(),
		"FLOW_FILE="+samplePath,
		"STORE=sqlite",
		"SQLITE_PATH="+sqlitePath,
		"KEY_PREFIX=",
		"OVERWRITE=1",
	)

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("mitmdump timed out: %s", output)
	}
	if err != nil {
		t.Fatalf("mitmdump failed: %v\n%s", err, output)
	}
}

func assertSampleFlowResponse(t *testing.T, method, url, expectedURL string) {
	t.Helper()

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status %d: %s", resp.StatusCode, body)
	}

	var payload map[string]interface{}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	args := requireMap(t, payload, "args")
	if name := requireString(t, args, "name"); name != "hello" {
		t.Fatalf("unexpected args.name: %s", name)
	}

	headers := requireMap(t, payload, "headers")
	if host := requireString(t, headers, "Host"); host != "httpbin.org" {
		t.Fatalf("unexpected headers.Host: %s", host)
	}

	if actual := requireString(t, payload, "url"); actual != expectedURL {
		t.Fatalf("unexpected url: %s", actual)
	}

	if origin := requireString(t, payload, "origin"); origin != "167.103.72.120" {
		t.Fatalf("unexpected origin: %s", origin)
	}
}

func requireMap(t *testing.T, payload map[string]interface{}, key string) map[string]interface{} {
	t.Helper()
	value, ok := payload[key]
	if !ok {
		t.Fatalf("missing key: %s", key)
	}
	mapped, ok := value.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected type for %s", key)
	}
	return mapped
}

func requireString(t *testing.T, payload map[string]interface{}, key string) string {
	t.Helper()
	value, ok := payload[key]
	if !ok {
		t.Fatalf("missing key: %s", key)
	}
	str, ok := value.(string)
	if !ok {
		t.Fatalf("unexpected type for %s", key)
	}
	return str
}
