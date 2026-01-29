package replay

import (
	"reflect"
	"testing"
)

func TestStoredResponseRoundTrip(t *testing.T) {
	original := StoredResponse{
		StatusCode: 201,
		Headers: []Header{
			{Key: "X-Test", Value: "ok"},
		},
		BodyBase64: "YWJj",
	}
	payload, err := encodeStoredResponse(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := decodeStoredResponse(payload)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(original, decoded) {
		t.Fatalf("mismatch: %#v != %#v", original, decoded)
	}
}
