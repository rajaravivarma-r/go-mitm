package replay

import (
	"log"
)

type ReplayRule struct {
	Name           string       `json:"name"`
	Enable         bool         `json:"enable"`
	Match          RequestMatch `json:"match"`
	AlwaysUpstream bool         `json:"always_upstream"`
	SkipReplay     bool         `json:"skip_replay"`
}

type ReplayPlugin struct {
	BasePlugin
	Rules       []*ReplayRule `json:"rules"`
	Enable      bool          `json:"enable"`
	LogNotFound bool          `json:"log_not_found"`
}

func NewReplayPlugin() *ReplayPlugin {
	return &ReplayPlugin{BasePlugin: BasePlugin{PluginName: "replay"}, Enable: true}
}

func (rp *ReplayPlugin) OnRequest(ctx *RequestContext) error {
	if !rp.Enable || ctx == nil || ctx.Request == nil {
		return nil
	}
	if rp.shouldSkip(ctx) {
		ctx.SkipCache = true
		return nil
	}
	if ctx.Repository == nil {
		return nil
	}
	if ctx.SkipCache {
		return nil
	}

	key := ctx.KeyPrefix + ctx.Key
	stored, found, err := ctx.Repository.Get(ctx.Request.Context(), key)
	if err != nil {
		return err
	}
	if !found {
		if rp.LogNotFound {
			log.Printf("cache miss: %s", key)
		}
		return nil
	}
	ctx.CacheHit = true
	ctx.Response = &stored
	return nil
}

func (rp *ReplayPlugin) shouldSkip(ctx *RequestContext) bool {
	for _, rule := range rp.Rules {
		if rule == nil || !rule.Enable {
			continue
		}
		if !rule.Match.matches(ctx) {
			continue
		}
		if rule.AlwaysUpstream || rule.SkipReplay {
			log.Printf("replay rule %s: skip replay", ruleName(rule.Name))
			return true
		}
	}
	return false
}

func NewReplayPluginFromFile(filename string) (*ReplayPlugin, error) {
	var replay ReplayPlugin
	if err := newStructFromFile(filename, &replay); err != nil {
		return nil, err
	}
	if replay.PluginName == "" {
		replay.PluginName = "replay"
	}
	return &replay, nil
}

func ruleName(name string) string {
	if name == "" {
		return "unnamed"
	}
	return name
}
