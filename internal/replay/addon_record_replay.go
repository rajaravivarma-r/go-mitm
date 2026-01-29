package replay

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/tidwall/match"
)

type RecordReplayMatch struct {
	Method       []string          `json:"method"`
	Host         string            `json:"host"`
	Path         string            `json:"path"`
	URL          string            `json:"url"`
	Header       map[string]string `json:"header"`
	BodyContains string            `json:"body_contains"`
}

func (m RecordReplayMatch) matches(ctx *RequestContext) bool {
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
	if m.URL != "" && !match.Match(requestFullURL(req), m.URL) {
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

type RecordReplayRule struct {
	Name           string            `json:"name"`
	Enable         bool              `json:"enable"`
	Match          RecordReplayMatch `json:"match"`
	AlwaysUpstream bool              `json:"always_upstream"`
	SkipCache      bool              `json:"skip_cache"`
	SkipStore      bool              `json:"skip_store"`
}

type RecordReplay struct {
	BasePlugin
	Rules  []*RecordReplayRule `json:"rules"`
	Enable bool                `json:"enable"`
}

func (rr *RecordReplay) OnRequest(ctx *RequestContext) error {
	if !rr.Enable {
		return nil
	}
	for _, rule := range rr.Rules {
		if rule == nil || !rule.Enable {
			continue
		}
		if !rule.Match.matches(ctx) {
			continue
		}
		rr.applyRule(ctx, rule)
		return nil
	}
	return nil
}

func (rr *RecordReplay) applyRule(ctx *RequestContext, rule *RecordReplayRule) {
	if rule.AlwaysUpstream {
		ctx.SkipCache = true
		ctx.SkipStore = true
		log.Printf("record-replay rule %s: force upstream", ruleName(rule))
		return
	}
	if rule.SkipCache {
		ctx.SkipCache = true
	}
	if rule.SkipStore {
		ctx.SkipStore = true
	}
	log.Printf("record-replay rule %s: skip-cache=%t skip-store=%t", ruleName(rule), ctx.SkipCache, ctx.SkipStore)
}

func (rr *RecordReplay) validate() error {
	for i, rule := range rr.Rules {
		if rule == nil {
			return fmt.Errorf("%d empty rule", i)
		}
	}
	return nil
}

func NewRecordReplayFromFile(filename string) (*RecordReplay, error) {
	var recordReplay RecordReplay
	if err := newStructFromFile(filename, &recordReplay); err != nil {
		return nil, err
	}
	if recordReplay.PluginName == "" {
		recordReplay.PluginName = "record-replay"
	}
	if err := recordReplay.validate(); err != nil {
		return nil, err
	}
	return &recordReplay, nil
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

func requestFullURL(req *http.Request) string {
	if req == nil || req.URL == nil {
		return ""
	}
	host := requestHost(req)
	if host == "" {
		return req.URL.RequestURI()
	}
	scheme := requestScheme(req)
	return scheme + "://" + host + req.URL.RequestURI()
}

func ruleName(rule *RecordReplayRule) string {
	if rule.Name == "" {
		return "unnamed"
	}
	return rule.Name
}
