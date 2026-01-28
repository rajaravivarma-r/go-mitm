package replay

import (
	"context"
	"encoding/json"
)

type Header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type StoredResponse struct {
	StatusCode int      `json:"status_code"`
	Headers    []Header `json:"headers"`
	BodyBase64 string   `json:"body_base64"`
}

type Repository interface {
	Get(ctx context.Context, key string) (StoredResponse, bool, error)
	Set(ctx context.Context, key string, value StoredResponse, overwrite bool) error
	Close() error
}

func encodeStoredResponse(response StoredResponse) ([]byte, error) {
	return json.Marshal(response)
}

func decodeStoredResponse(payload []byte) (StoredResponse, error) {
	var response StoredResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return StoredResponse{}, err
	}
	return response, nil
}
