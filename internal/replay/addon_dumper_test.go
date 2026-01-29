package replay

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDumperWritesOutput(t *testing.T) {
	var out bytes.Buffer
	dumper := NewDumper(&out, 1)

	req := httptest.NewRequest(http.MethodPost, "http://example.com/hello", strings.NewReader("hi"))
	ctx := &RequestContext{
		Request: req,
		Body:    []byte("hi"),
	}
	stored := &StoredResponse{
		StatusCode: 200,
		Headers: []Header{
			{Key: "Content-Type", Value: "text/plain"},
		},
		BodyBase64: base64.StdEncoding.EncodeToString([]byte("ok")),
	}

	if err := dumper.OnResponse(ctx, stored); err != nil {
		t.Fatalf("OnResponse: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "POST /hello") {
		t.Fatalf("missing request line: %s", got)
	}
	if !strings.Contains(got, "200 OK") {
		t.Fatalf("missing response line: %s", got)
	}
	if !strings.Contains(got, "ok") {
		t.Fatalf("missing body: %s", got)
	}
}
