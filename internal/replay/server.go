package replay

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ServerOptions struct {
	KeyPrefix       string
	LogNotFound     bool
	Upstream        *UpstreamClient
	RecordMiss      bool
	RecordOverwrite bool
	Plugins         []Plugin
}

func NewReplayRouter(repository Repository, options ServerOptions) *gin.Engine {
	router := gin.Default()
	router.Any("/*any", func(c *gin.Context) {
		flowReq, readErr := readFlowRequest(c.Request)
		if readErr != nil {
			log.Printf("read request: %v", readErr)
			c.Status(http.StatusBadRequest)
			return
		}

		ctx := &RequestContext{
			Request:    c.Request,
			Body:       flowReq.body,
			Key:        flowReq.key,
			Repository: repository,
		}
		if pluginErr := applyRequestPlugins(options.Plugins, ctx); pluginErr != nil {
			log.Printf("request plugin failed: %v", pluginErr)
			c.Status(statusFromPluginError(pluginErr))
			return
		}
		if ctx.Response != nil {
			if pluginErr := applyResponsePlugins(options.Plugins, ctx, ctx.Response); pluginErr != nil {
				log.Printf("response plugin failed: %v", pluginErr)
				c.Status(statusFromPluginError(pluginErr))
				return
			}
			if writeErr := writeStoredResponse(c.Writer, *ctx.Response); writeErr != nil {
				log.Printf("write response failed: %v", writeErr)
			}
			return
		}

		key := options.KeyPrefix + ctx.Key
		var stored StoredResponse
		if !ctx.SkipCache {
			var found bool
			var lookupErr error
			stored, found, lookupErr = repository.Get(c.Request.Context(), key)
			if lookupErr != nil {
				log.Printf("lookup failed: %v", lookupErr)
				c.Status(http.StatusBadGateway)
				return
			}
			if found {
				ctx.CacheHit = true
				if pluginErr := applyResponsePlugins(options.Plugins, ctx, &stored); pluginErr != nil {
					log.Printf("response plugin failed: %v", pluginErr)
					c.Status(statusFromPluginError(pluginErr))
					return
				}
				if writeErr := writeStoredResponse(c.Writer, stored); writeErr != nil {
					log.Printf("write response failed: %v", writeErr)
				}
				return
			}
		}

		if options.LogNotFound && !ctx.SkipCache {
			log.Printf("cache miss: %s", key)
		}
		if options.Upstream == nil {
			c.Status(http.StatusNotFound)
			return
		}

		resp, respBody, fetchErr := options.Upstream.Fetch(c.Request.Context(), ctx.Request, ctx.Body)
		if fetchErr != nil {
			log.Printf("upstream fetch failed: %v", fetchErr)
			c.Status(http.StatusBadGateway)
			return
		}

		stored = storedResponseFromHTTP(resp, respBody)
		if !ctx.SkipStore {
			if storeErr := repository.Set(c.Request.Context(), key, stored, options.RecordOverwrite); storeErr != nil {
				log.Printf("store response failed: %v", storeErr)
			}
		}
		if pluginErr := applyResponsePlugins(options.Plugins, ctx, &stored); pluginErr != nil {
			log.Printf("response plugin failed: %v", pluginErr)
			c.Status(statusFromPluginError(pluginErr))
			return
		}
		if writeErr := writeStoredResponse(c.Writer, stored); writeErr != nil {
			log.Printf("write response failed: %v", writeErr)
		}
	})
	return router
}
