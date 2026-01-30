package replay

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/tidwall/match"
)

type RequestMatch struct {
	Method       []string          `json:"method"`
	Host         string            `json:"host"`
	Path         string            `json:"path"`
	URL          string            `json:"url"`
	Header       map[string]string `json:"header"`
	BodyContains string            `json:"body_contains"`
}

func (m RequestMatch) matches(ctx *RequestContext) bool {
	if ctx == nil || ctx.Request == nil {
		return false
	}
	req := ctx.Request
	if len(m.Method) > 0 && !containsString(m.Method, req.Method) {
		return false
	}
	if m.Host != "" && m.Host != requestHost(req) {
		return false
	}
	if m.Path != "" && !match.Match(req.URL.Path, m.Path) {
		return false
	}
	if m.URL != "" && !match.Match(canonicalRequestURL(req), m.URL) {
		return false
	}
	if len(m.Header) > 0 && !headersMatch(req.Header, m.Header) {
		return false
	}
	if m.BodyContains != "" && !bytes.Contains(ctx.Body, []byte(m.BodyContains)) {
		return false
	}
	return true
}

func headersMatch(headers http.Header, matchers map[string]string) bool {
	for key, expected := range matchers {
		if expected == "" {
			continue
		}
		found := false
		for name, values := range headers {
			if !strings.EqualFold(name, key) {
				continue
			}
			for _, value := range values {
				if strings.Contains(strings.ToLower(value), strings.ToLower(expected)) {
					found = true
					break
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func canonicalRequestURL(req *http.Request) string {
	if req == nil || req.URL == nil {
		return ""
	}
	host := requestHost(req)
	scheme := requestScheme(req)
	path := req.URL.Path
	query := req.URL.RawQuery
	if query != "" {
		if sorted, err := sortQueryParams(query); err == nil {
			query = sorted
		}
	}
	if host == "" {
		if query == "" {
			return path
		}
		return path + "?" + query
	}
	if query == "" {
		return scheme + "://" + host + path
	}
	return scheme + "://" + host + path + "?" + query
}
