package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rajaravivarma/go-mitm/internal/replay"
)

func main() {
	listenAddr := flag.String("listen", ":8090", "Address to listen on")
	storeType := flag.String("store", "redis", "Storage backend: redis or sqlite")
	keyPrefix := flag.String("key-prefix", "", "Prefix for storage keys")
	logNotFound := flag.Bool("log-not-found", false, "Log cache misses")

	redisAddr := flag.String("redis-addr", "127.0.0.1:6379", "Redis host:port")
	redisPassword := flag.String("redis-password", "", "Redis password")
	redisDB := flag.Int("redis-db", 0, "Redis database")
	redisTimeout := flag.Duration("redis-timeout", 5*time.Second, "Redis operation timeout")

	sqlitePath := flag.String("sqlite-path", "mitm_flows.sqlite", "SQLite database path")
	sqliteTimeout := flag.Duration("sqlite-timeout", 5*time.Second, "SQLite busy timeout")

	recordMiss := flag.Bool("record-miss", false, "Deprecated: upstream responses are cached automatically")
	recordOverwrite := flag.Bool("record-overwrite", false, "Overwrite stored response when recording")
	upstreamURL := flag.String("upstream", "", "Upstream base URL for cache misses")
	upstreamTimeout := flag.Duration("upstream-timeout", 30*time.Second, "Timeout for upstream requests")

	flag.Parse()

	gin.SetMode(gin.ReleaseMode)

	var repository replay.Repository
	var err error

	switch *storeType {
	case "redis":
		repository = replay.NewRedisRepository(*redisAddr, *redisPassword, *redisDB, *redisTimeout)
	case "sqlite":
		repository, err = replay.NewSQLiteRepository(*sqlitePath, *sqliteTimeout)
	default:
		log.Fatalf("unsupported store type: %s", *storeType)
	}
	if err != nil {
		log.Fatalf("storage init failed: %v", err)
	}
	defer func() {
		if closeErr := repository.Close(); closeErr != nil {
			log.Printf("storage close failed: %v", closeErr)
		}
	}()

	var upstream *replay.UpstreamClient
	if *upstreamURL != "" {
		upstream, err = replay.NewUpstreamClient(*upstreamURL, *upstreamTimeout)
		if err != nil {
			log.Fatalf("upstream init failed: %v", err)
		}
	}

	router := replay.NewReplayRouter(repository, replay.ServerOptions{
		KeyPrefix:       *keyPrefix,
		LogNotFound:     *logNotFound,
		Upstream:        upstream,
		RecordMiss:      *recordMiss,
		RecordOverwrite: *recordOverwrite,
		Plugins: []replay.Plugin{
			&replay.ReplayPlugin{
				BasePlugin:  replay.BasePlugin{PluginName: "replay"},
				Enable:      true,
				LogNotFound: *logNotFound,
			},
			&replay.RecordPlugin{
				BasePlugin:        replay.BasePlugin{PluginName: "record"},
				Enable:            true,
				Overwrite:         *recordOverwrite,
				IgnoreStatusCodes: []int{http.StatusTooManyRequests},
			},
		},
	})

	if err := router.Run(*listenAddr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
