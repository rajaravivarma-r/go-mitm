#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT_DIR=$(cd "$SCRIPT_DIR/.." && pwd)

STORE="redis"
FLOW_FILE=""
KEY_PREFIX=""
LISTEN=":8090"
BATCH_SIZE=1000
OVERWRITE=false
INCLUDE_EMPTY=false
INCLUDE_ERRORS=false
LOG_NOT_FOUND=false

REDIS_URL="redis://localhost:6379/0"
REDIS_ADDR="127.0.0.1:6379"
REDIS_DB="0"
REDIS_PASSWORD=""

SQLITE_PATH="$ROOT_DIR/mitm_flows.sqlite"
DUMP_SCRIPT="${MITM_DUMP_SCRIPT:-}"

RECORD_MISS=false
RECORD_OVERWRITE=false
UPSTREAM_URL=""

usage() {
  cat <<'USAGE'
Usage: run_replay.sh --flow-file PATH [options]

Options:
  --store redis|sqlite        Storage backend (default: redis)
  --flow-file PATH            Path to the .flow file (required)
  --key-prefix PREFIX         Storage key prefix
  --listen ADDR               Listen address (default: :8090)
  --batch-size N              Batch size for loading flows (default: 1000)
  --overwrite                 Overwrite existing keys when loading flows
  --include-empty             Include responses with empty bodies
  --include-errors            Include responses with status >= 400 (except 404)
  --log-not-found             Log cache misses
  --dump-script PATH          Path to mitmdump dump_flows_to_redis.py script

Redis options:
  --redis-url URL             Redis URL for loading flows (default: redis://localhost:6379/0)
  --redis-addr HOST:PORT      Redis address for server (default: 127.0.0.1:6379)
  --redis-db N                Redis DB for server (default: 0)
  --redis-password PASS       Redis password for server

SQLite options:
  --sqlite-path PATH          SQLite database path (default: ./mitm_flows.sqlite)

Recording options:
  --record-miss               Deprecated: upstream responses are cached automatically
  --record-overwrite          Overwrite stored entries while recording
  --upstream URL              Upstream base URL for cache misses
USAGE
}

abspath() {
  local path="$1"
  if [[ "$path" == /* ]]; then
    printf '%s\n' "$path"
    return
  fi
  local dir
  dir=$(cd "$(dirname "$path")" && pwd)
  printf '%s/%s\n' "$dir" "$(basename "$path")"
}

parse_redis_url() {
  local url="$1"
  local no_scheme="${url#*://}"
  local hostport="${no_scheme%%/*}"
  local path="${no_scheme#*/}"
  local db="0"
  if [[ "$hostport" == *"@"* ]]; then
    local userinfo="${hostport%%@*}"
    hostport="${hostport#*@}"
    if [[ "$userinfo" == *":"* ]]; then
      REDIS_PASSWORD="${userinfo#*:}"
    fi
  fi
  if [[ "$no_scheme" != "$hostport" ]]; then
    db="${path%%\?*}"
    if [[ -z "$db" ]]; then
      db="0"
    fi
  fi
  REDIS_ADDR="$hostport"
  REDIS_DB="$db"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --store)
      STORE="$2"
      shift 2
      ;;
    --flow-file)
      FLOW_FILE="$2"
      shift 2
      ;;
    --key-prefix)
      KEY_PREFIX="$2"
      shift 2
      ;;
    --dump-script)
      DUMP_SCRIPT="$2"
      shift 2
      ;;
    --listen)
      LISTEN="$2"
      shift 2
      ;;
    --batch-size)
      BATCH_SIZE="$2"
      shift 2
      ;;
    --overwrite)
      OVERWRITE=true
      shift
      ;;
    --include-empty)
      INCLUDE_EMPTY=true
      shift
      ;;
    --include-errors)
      INCLUDE_ERRORS=true
      shift
      ;;
    --log-not-found)
      LOG_NOT_FOUND=true
      shift
      ;;
    --redis-url)
      REDIS_URL="$2"
      parse_redis_url "$REDIS_URL"
      shift 2
      ;;
    --redis-addr)
      REDIS_ADDR="$2"
      shift 2
      ;;
    --redis-db)
      REDIS_DB="$2"
      shift 2
      ;;
    --redis-password)
      REDIS_PASSWORD="$2"
      shift 2
      ;;
    --sqlite-path)
      SQLITE_PATH="$2"
      shift 2
      ;;
    --record-miss)
      RECORD_MISS=true
      shift
      ;;
    --record-overwrite)
      RECORD_OVERWRITE=true
      shift
      ;;
    --upstream)
      UPSTREAM_URL="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ -z "$FLOW_FILE" ]]; then
  echo "Missing --flow-file" >&2
  usage
  exit 1
fi
if [[ -z "$DUMP_SCRIPT" ]]; then
  echo "Missing --dump-script or MITM_DUMP_SCRIPT env var" >&2
  usage
  exit 1
fi

FLOW_FILE=$(abspath "$FLOW_FILE")
SQLITE_PATH=$(abspath "$SQLITE_PATH")
DUMP_SCRIPT=$(abspath "$DUMP_SCRIPT")

if [[ "$STORE" != "redis" && "$STORE" != "sqlite" ]]; then
  echo "Invalid --store value: $STORE" >&2
  usage
  exit 1
fi
if [[ ! -f "$DUMP_SCRIPT" ]]; then
  echo "Dump script not found: $DUMP_SCRIPT" >&2
  exit 1
fi

if [[ "$RECORD_MISS" == "true" && -z "$UPSTREAM_URL" ]]; then
  echo "--upstream is required when --record-miss is set" >&2
  exit 1
fi

MITMDUMP_BIN=${MITMDUMP_BIN:-mitmdump}

DUMP_ENV=(FLOW_FILE="$FLOW_FILE" STORE="$STORE" KEY_PREFIX="$KEY_PREFIX" BATCH_SIZE="$BATCH_SIZE")
if [[ "$OVERWRITE" == "true" ]]; then
  DUMP_ENV+=(OVERWRITE=1)
fi
if [[ "$INCLUDE_EMPTY" == "true" ]]; then
  DUMP_ENV+=(INCLUDE_EMPTY=1)
fi
if [[ "$INCLUDE_ERRORS" == "true" ]]; then
  DUMP_ENV+=(INCLUDE_ERRORS=1)
fi
if [[ "$STORE" == "redis" ]]; then
  DUMP_ENV+=(REDIS_URL="$REDIS_URL")
else
  DUMP_ENV+=(SQLITE_PATH="$SQLITE_PATH")
fi

env "${DUMP_ENV[@]}" "$MITMDUMP_BIN" -s "$DUMP_SCRIPT" -n

cd "$ROOT_DIR"

GO_ARGS=(go run ./cmd/mitmredis -listen "$LISTEN" -store "$STORE" -key-prefix "$KEY_PREFIX")
if [[ "$LOG_NOT_FOUND" == "true" ]]; then
  GO_ARGS+=(-log-not-found)
fi
if [[ "$STORE" == "redis" ]]; then
  GO_ARGS+=(-redis-addr "$REDIS_ADDR" -redis-db "$REDIS_DB")
  if [[ -n "$REDIS_PASSWORD" ]]; then
    GO_ARGS+=(-redis-password "$REDIS_PASSWORD")
  fi
else
  GO_ARGS+=(-sqlite-path "$SQLITE_PATH")
fi
if [[ -n "$UPSTREAM_URL" ]]; then
  GO_ARGS+=(-upstream "$UPSTREAM_URL")
fi
if [[ "$RECORD_MISS" == "true" ]]; then
  GO_ARGS+=(-record-miss)
  if [[ "$RECORD_OVERWRITE" == "true" ]]; then
    GO_ARGS+=(-record-overwrite)
  fi
fi

exec "${GO_ARGS[@]}"
