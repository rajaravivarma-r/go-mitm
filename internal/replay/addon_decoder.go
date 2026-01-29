package replay

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"io"
	"strings"
)

// Decoder decodes common content-encodings before responding to clients.
type Decoder struct {
	BasePlugin
}

func NewDecoder() *Decoder {
	return &Decoder{BasePlugin: BasePlugin{PluginName: "decoder"}}
}

func (d *Decoder) OnResponse(_ *RequestContext, stored *StoredResponse) error {
	if stored == nil || stored.BodyBase64 == "" {
		return nil
	}
	encHeader, _ := findHeader(stored.Headers, "Content-Encoding")
	if encHeader == "" {
		return nil
	}
	encodings := parseHeaderTokens(encHeader)
	if len(encodings) == 0 {
		return nil
	}

	body, err := base64.StdEncoding.DecodeString(stored.BodyBase64)
	if err != nil {
		return err
	}

	decoded, remaining, err := decodeBody(body, encodings)
	if err != nil {
		return err
	}
	stored.BodyBase64 = base64.StdEncoding.EncodeToString(decoded)
	if len(remaining) == 0 {
		removeHeader(&stored.Headers, "Content-Encoding")
	} else {
		updateHeader(&stored.Headers, "Content-Encoding", strings.Join(remaining, ", "))
	}
	return nil
}

func decodeBody(body []byte, encodings []string) ([]byte, []string, error) {
	remaining := make([]string, 0, len(encodings))
	current := body
	for _, enc := range encodings {
		switch strings.ToLower(enc) {
		case "gzip":
			decoded, err := decodeGzip(current)
			if err != nil {
				return nil, nil, err
			}
			current = decoded
		case "deflate":
			decoded, err := decodeDeflate(current)
			if err != nil {
				return nil, nil, err
			}
			current = decoded
		case "identity", "":
			continue
		default:
			remaining = append(remaining, enc)
		}
	}
	if len(remaining) > 0 {
		return current, remaining, nil
	}
	return current, nil, nil
}

func decodeGzip(body []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func decodeDeflate(body []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func parseHeaderTokens(value string) []string {
	parts := strings.Split(value, ",")
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func findHeader(headers []Header, key string) (string, bool) {
	for _, header := range headers {
		if strings.EqualFold(header.Key, key) {
			return header.Value, true
		}
	}
	return "", false
}

func updateHeader(headers *[]Header, key, value string) {
	for i, header := range *headers {
		if strings.EqualFold(header.Key, key) {
			(*headers)[i].Value = value
			return
		}
	}
	*headers = append(*headers, Header{Key: key, Value: value})
}

func removeHeader(headers *[]Header, key string) {
	filtered := (*headers)[:0]
	for _, header := range *headers {
		if strings.EqualFold(header.Key, key) {
			continue
		}
		filtered = append(filtered, header)
	}
	*headers = filtered
}
