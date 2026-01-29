package replay

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"testing"
)

func TestDecoderGzip(t *testing.T) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("write gzip: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}

	stored := StoredResponse{
		StatusCode: 200,
		Headers: []Header{
			{Key: "Content-Encoding", Value: "gzip"},
		},
		BodyBase64: base64.StdEncoding.EncodeToString(buf.Bytes()),
	}
	decoder := NewDecoder()
	if err := decoder.OnResponse(&RequestContext{}, &stored); err != nil {
		t.Fatalf("OnResponse: %v", err)
	}
	body, err := base64.StdEncoding.DecodeString(stored.BodyBase64)
	if err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if string(body) != "hello" {
		t.Fatalf("unexpected body: %s", body)
	}
	if _, ok := findHeader(stored.Headers, "Content-Encoding"); ok {
		t.Fatalf("expected Content-Encoding to be removed")
	}
}
