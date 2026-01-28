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

		key := options.KeyPrefix + flowReq.key
		stored, found, lookupErr := repository.Get(c.Request.Context(), key)
		if lookupErr != nil {
			log.Printf("lookup failed: %v", lookupErr)
			c.Status(http.StatusBadGateway)
			return
		}
		if found {
			if writeErr := writeStoredResponse(c.Writer, stored); writeErr != nil {
				log.Printf("write response failed: %v", writeErr)
			}
			return
		}

		if options.LogNotFound {
			log.Printf("cache miss: %s", key)
		}
		if options.Upstream == nil {
			c.Status(http.StatusNotFound)
			return
		}

		resp, respBody, fetchErr := options.Upstream.Fetch(c.Request.Context(), c.Request, flowReq.body)
		if fetchErr != nil {
			log.Printf("upstream fetch failed: %v", fetchErr)
			c.Status(http.StatusBadGateway)
			return
		}

		stored = storedResponseFromHTTP(resp, respBody)
		if storeErr := repository.Set(c.Request.Context(), key, stored, options.RecordOverwrite); storeErr != nil {
			log.Printf("store response failed: %v", storeErr)
		}
		if writeErr := writeStoredResponse(c.Writer, stored); writeErr != nil {
			log.Printf("write response failed: %v", writeErr)
		}
	})
	return router
}
