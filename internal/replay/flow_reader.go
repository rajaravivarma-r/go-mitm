package replay

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"unicode/utf16"
)

type flowRequest struct {
	key  string
	body []byte
}

func readFlowRequest(req *http.Request) (flowRequest, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return flowRequest{}, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))

	key, err := buildKey(req, body)
	if err != nil {
		return flowRequest{}, err
	}
	return flowRequest{key: key, body: body}, nil
}

func storedResponseFromHTTP(resp *http.Response, body []byte) StoredResponse {
	headers := make([]Header, 0, len(resp.Header))
	for key, values := range resp.Header {
		for _, value := range values {
			headers = append(headers, Header{Key: key, Value: value})
		}
	}
	sort.Slice(headers, func(i, j int) bool {
		if headers[i].Key == headers[j].Key {
			return headers[i].Value < headers[j].Value
		}
		return headers[i].Key < headers[j].Key
	})

	bodyEncoded := ""
	if len(body) > 0 {
		bodyEncoded = base64.StdEncoding.EncodeToString(body)
	}
	return StoredResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		BodyBase64: bodyEncoded,
	}
}

func writeStoredResponse(w http.ResponseWriter, stored StoredResponse) error {
	for _, header := range stored.Headers {
		if shouldSkipHeader(header.Key, header.Value) {
			continue
		}
		w.Header().Add(header.Key, header.Value)
	}
	w.WriteHeader(stored.StatusCode)
	if stored.BodyBase64 == "" {
		return nil
	}
	bodyBytes, err := base64.StdEncoding.DecodeString(stored.BodyBase64)
	if err != nil {
		return err
	}
	_, err = w.Write(bodyBytes)
	return err
}

type queryPair struct {
	key   string
	value string
}

func sortQueryParams(rawQuery string) (string, error) {
	if rawQuery == "" {
		return "", nil
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", err
	}

	pairs := make([]queryPair, 0)
	for key, vals := range values {
		if len(vals) == 0 {
			pairs = append(pairs, queryPair{key: key, value: ""})
			continue
		}
		for _, val := range vals {
			pairs = append(pairs, queryPair{key: key, value: val})
		}
	}

	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].key == pairs[j].key {
			return pairs[i].value < pairs[j].value
		}
		return pairs[i].key < pairs[j].key
	})

	return encodeQueryPairs(pairs), nil
}

func encodeQueryPairs(pairs []queryPair) string {
	var builder strings.Builder
	for i, pair := range pairs {
		if i > 0 {
			builder.WriteByte('&')
		}
		builder.WriteString(url.QueryEscape(pair.key))
		builder.WriteByte('=')
		builder.WriteString(url.QueryEscape(pair.value))
	}
	return builder.String()
}

type sortedItem struct {
	value   interface{}
	sortKey string
}

func normalizeJSON(value interface{}) interface{} {
	switch val := value.(type) {
	case map[string]interface{}:
		normalized := make(map[string]interface{}, len(val))
		for key, item := range val {
			normalized[key] = normalizeJSON(item)
		}
		return normalized
	case []interface{}:
		items := make([]sortedItem, len(val))
		for i, item := range val {
			normalized := normalizeJSON(item)
			items[i] = sortedItem{
				value:   normalized,
				sortKey: canonicalJSONString(normalized),
			}
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].sortKey < items[j].sortKey
		})
		sorted := make([]interface{}, len(items))
		for i, item := range items {
			sorted[i] = item.value
		}
		return sorted
	default:
		return val
	}
}

func canonicalJSON(body []byte) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	normalized := normalizeJSON(value)
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return escapeJSONASCII(string(encoded)), nil
}

func canonicalJSONString(value interface{}) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return escapeJSONASCII(string(encoded))
}

func escapeJSONASCII(input string) string {
	var builder strings.Builder
	builder.Grow(len(input))
	for _, r := range input {
		if r <= 0x7f {
			builder.WriteRune(r)
			continue
		}
		if r <= 0xffff {
			fmt.Fprintf(&builder, "\\u%04x", r)
			continue
		}
		hi, lo := utf16.EncodeRune(r)
		fmt.Fprintf(&builder, "\\u%04x\\u%04x", hi, lo)
	}
	return builder.String()
}

func buildKey(req *http.Request, body []byte) (string, error) {
	path := req.URL.Path
	method := req.Method

	sortedQuery, err := sortQueryParams(req.URL.RawQuery)
	if err != nil {
		return "", err
	}

	parts := []string{path, method, sortedQuery}
	if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		body = bytes.TrimSpace(body)
		if len(body) > 0 {
			contentType := req.Header.Get("Content-Type")
			if strings.Contains(contentType, "application/json") {
				normalized, err := canonicalJSON(body)
				if err != nil {
					return "", err
				}
				parts = append(parts, normalized)
			} else if strings.Contains(contentType, "application/x-www-form-urlencoded") {
				encoded, err := sortQueryParams(string(body))
				if err != nil {
					return "", err
				}
				parts = append(parts, encoded)
			}
		}
	}

	return strings.Join(parts, "|"), nil
}

func shouldSkipHeader(key, value string) bool {
	if strings.EqualFold(key, "Content-Length") {
		return true
	}
	if strings.EqualFold(key, "Transfer-Encoding") {
		return true
	}
	if strings.EqualFold(key, "Content-Encoding") && strings.EqualFold(value, "gzip") {
		return true
	}
	return false
}
