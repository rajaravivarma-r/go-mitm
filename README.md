# mitmredis

Replay mitmproxy flows from Redis or SQLite, and optionally record new responses from an upstream server.

## Prerequisites

- Go 1.22+
- Redis (only if using the Redis backend)
- `mitmdump` from mitmproxy (for loading `.flow` files)

## Load flow file into Redis or SQLite

Use the mitmproxy helper to load a `.flow` file into Redis keys or a SQLite database:

```
FLOW_FILE=/path/to/file.flow \
STORE=redis \
REDIS_URL=redis://localhost:6379/0 \
KEY_PREFIX="" \
mitmdump -s /Users/rajaravivarma/Github/scripts-collection/python/mitm-scripts/dump_flows_to_redis.py -n
```

```
FLOW_FILE=/path/to/file.flow \
STORE=sqlite \
SQLITE_PATH=./mitm_flows.sqlite \
KEY_PREFIX="" \
mitmdump -s /Users/rajaravivarma/Github/scripts-collection/python/mitm-scripts/dump_flows_to_redis.py -n
```

## One-step wrapper

The wrapper loads the `.flow` file and starts the server in one go:

```
./run_replay.sh --flow-file /path/to/file.flow --store redis
```

```
./run_replay.sh --flow-file /path/to/file.flow --store sqlite --sqlite-path ./mitm_flows.sqlite
```

## Run the replay server (Redis)

```
go run ./mitmredis \
  -listen :8090 \
  -store redis \
  -redis-addr 127.0.0.1:6379 \
  -redis-db 0 \
  -key-prefix ""
```

## Run the replay server (SQLite)

```
go run ./mitmredis \
  -listen :8090 \
  -store sqlite \
  -sqlite-path ./mitm_flows.sqlite \
  -key-prefix ""
```

## Forward cache misses to an upstream

When `-upstream` is set, cache misses are forwarded to the upstream server and cached in the selected backend automatically.

```
go run ./mitmredis \
  -listen :8090 \
  -store redis \
  -redis-addr 127.0.0.1:6379 \
  -upstream https://api.example.com
```

```
go run ./mitmredis \
  -listen :8090 \
  -store redis \
  -redis-addr 127.0.0.1:6379 \
  -record-overwrite \
  -upstream https://api.example.com
```

## Tests

```
go test ./mitmredis
```
