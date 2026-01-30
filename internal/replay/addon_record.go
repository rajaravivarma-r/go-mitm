package replay

import (
	"log"
)

type RecordRule struct {
	Name           string       `json:"name"`
	Enable         bool         `json:"enable"`
	Match          RequestMatch `json:"match"`
	AlwaysUpstream bool         `json:"always_upstream"`
	SkipStore      bool         `json:"skip_store"`
}

type RecordPlugin struct {
	BasePlugin
	Rules             []*RecordRule `json:"rules"`
	Enable            bool          `json:"enable"`
	Overwrite         bool          `json:"overwrite"`
	IgnoreStatusCodes []int         `json:"ignore_status_codes"`
}

func NewRecordPlugin() *RecordPlugin {
	return &RecordPlugin{BasePlugin: BasePlugin{PluginName: "record"}, Enable: true}
}

func (rp *RecordPlugin) OnResponse(ctx *RequestContext, stored *StoredResponse) error {
	if !rp.Enable || ctx == nil || ctx.Request == nil || ctx.Repository == nil || stored == nil {
		return nil
	}
	if ctx.CacheHit || ctx.SkipStore {
		return nil
	}
	if shouldSkipStatus(stored.StatusCode, rp.IgnoreStatusCodes) {
		return nil
	}
	if rp.shouldSkip(ctx) {
		return nil
	}

	key := ctx.KeyPrefix + ctx.Key
	if err := ctx.Repository.Set(ctx.Request.Context(), key, *stored, rp.Overwrite); err != nil {
		return err
	}
	log.Printf("stored response: %s", key)
	return nil
}

func (rp *RecordPlugin) shouldSkip(ctx *RequestContext) bool {
	for _, rule := range rp.Rules {
		if rule == nil || !rule.Enable {
			continue
		}
		if !rule.Match.matches(ctx) {
			continue
		}
		if rule.AlwaysUpstream || rule.SkipStore {
			log.Printf("record rule %s: skip store", ruleName(rule.Name))
			return true
		}
	}
	return false
}

func shouldSkipStatus(code int, codes []int) bool {
	for _, value := range codes {
		if value == code {
			return true
		}
	}
	return false
}

func NewRecordPluginFromFile(filename string) (*RecordPlugin, error) {
	var record RecordPlugin
	if err := newStructFromFile(filename, &record); err != nil {
		return nil, err
	}
	if record.PluginName == "" {
		record.PluginName = "record"
	}
	return &record, nil
}
