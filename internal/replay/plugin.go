package replay

import (
	"errors"
	"fmt"
	"net/http"
)

// RequestContext carries mutable request state for plugin hooks.
// Plugins may update Key or Body to influence cache lookup and upstream fetch.
type RequestContext struct {
	Request    *http.Request
	Body       []byte
	Key        string
	CacheHit   bool
	Repository Repository
	SkipCache  bool
	SkipStore  bool
	// Response allows plugins to short-circuit cache/upstream handling.
	Response *StoredResponse
}

// Plugin is the base interface for replay plugins.
// Implement RequestPlugin and/or ResponsePlugin to hook into traffic.
type Plugin interface {
	Name() string
}

// BasePlugin provides a default Name implementation.
type BasePlugin struct {
	PluginName string
}

func (p BasePlugin) Name() string {
	if p.PluginName != "" {
		return p.PluginName
	}
	return "plugin"
}

// RequestPlugin is invoked after a request key is built but before cache lookup.
type RequestPlugin interface {
	OnRequest(*RequestContext) error
}

// ResponsePlugin is invoked before a response is written to the client.
type ResponsePlugin interface {
	OnResponse(*RequestContext, *StoredResponse) error
}

// PluginError allows plugins to control the HTTP status returned on failure.
type PluginError struct {
	Status int
	Err    error
}

func (e PluginError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return "plugin error"
}

func (e PluginError) Unwrap() error {
	return e.Err
}

type pluginCallError struct {
	plugin string
	err    error
}

func (e pluginCallError) Error() string {
	return fmt.Sprintf("plugin %s: %v", e.plugin, e.err)
}

func (e pluginCallError) Unwrap() error {
	return e.err
}

func applyRequestPlugins(plugins []Plugin, ctx *RequestContext) error {
	for _, plugin := range plugins {
		if plugin == nil {
			continue
		}
		hook, ok := plugin.(RequestPlugin)
		if !ok {
			continue
		}
		if err := hook.OnRequest(ctx); err != nil {
			return pluginCallError{plugin: plugin.Name(), err: err}
		}
	}
	return nil
}

func applyResponsePlugins(plugins []Plugin, ctx *RequestContext, stored *StoredResponse) error {
	for _, plugin := range plugins {
		if plugin == nil {
			continue
		}
		hook, ok := plugin.(ResponsePlugin)
		if !ok {
			continue
		}
		if err := hook.OnResponse(ctx, stored); err != nil {
			return pluginCallError{plugin: plugin.Name(), err: err}
		}
	}
	return nil
}

func statusFromPluginError(err error) int {
	var pluginErr PluginError
	if errors.As(err, &pluginErr) && pluginErr.Status > 0 {
		return pluginErr.Status
	}
	return http.StatusInternalServerError
}
