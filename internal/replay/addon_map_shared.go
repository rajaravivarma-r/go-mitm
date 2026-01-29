package replay

import (
	"net/http"

	"github.com/tidwall/match"
)

type mapFrom struct {
	Protocol string   `json:"protocol"`
	Host     string   `json:"host"`
	Method   []string `json:"method"`
	Path     string   `json:"path"`
}

func (mf *mapFrom) match(req *http.Request) bool {
	if mf.Protocol != "" && mf.Protocol != requestScheme(req) {
		return false
	}
	if mf.Host != "" && mf.Host != requestHost(req) {
		return false
	}
	if len(mf.Method) > 0 && !containsString(mf.Method, req.Method) {
		return false
	}
	if mf.Path != "" && !match.Match(req.URL.Path, mf.Path) {
		return false
	}
	return true
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func requestHost(req *http.Request) string {
	if req.URL != nil && req.URL.Host != "" {
		return req.URL.Host
	}
	return req.Host
}

func requestScheme(req *http.Request) string {
	if req.URL != nil && req.URL.Scheme != "" {
		return req.URL.Scheme
	}
	if req.TLS != nil {
		return "https"
	}
	return "http"
}
