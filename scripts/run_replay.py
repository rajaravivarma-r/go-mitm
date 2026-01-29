#!/usr/bin/env python3
import argparse
import os
import subprocess
import sys
from pathlib import Path
from urllib.parse import urlparse


def abspath(value: str) -> str:
    return str(Path(value).expanduser().resolve())


def parse_redis_url(url: str):
    parsed = urlparse(url)
    host = parsed.hostname or ""
    port = parsed.port
    hostport = host
    if port is not None:
        hostport = f"{host}:{port}"
    if parsed.netloc and not hostport:
        hostport = parsed.netloc

    password = parsed.password or ""
    db = "0"
    if parsed.path and parsed.path != "/":
        db = parsed.path.lstrip("/") or "0"
    return hostport, db, password


def flag_present(flag: str) -> bool:
    return flag in sys.argv[1:]


def main() -> int:
    parser = argparse.ArgumentParser(prog="run_replay.sh")
    parser.add_argument("--store", default="redis", choices=["redis", "sqlite"], help="Storage backend")
    parser.add_argument("--flow-file", required=True, help="Path to the .flow file")
    parser.add_argument("--key-prefix", default="", help="Storage key prefix")
    parser.add_argument("--listen", default=":8090", help="Listen address")
    parser.add_argument("--batch-size", default="1000", help="Batch size for loading flows")
    parser.add_argument("--overwrite", action="store_true", help="Overwrite existing keys when loading flows")
    parser.add_argument("--include-empty", action="store_true", help="Include responses with empty bodies")
    parser.add_argument("--include-errors", action="store_true", help="Include responses with status >= 400")
    parser.add_argument("--log-not-found", action="store_true", help="Log cache misses")
    parser.add_argument("--dump-script", help="Path to mitmdump dump_flows_to_redis.py script")

    parser.add_argument("--redis-url", default="redis://localhost:6379/0", help="Redis URL for loading flows")
    parser.add_argument("--redis-addr", default="127.0.0.1:6379", help="Redis address for server")
    parser.add_argument("--redis-db", default="0", help="Redis DB for server")
    parser.add_argument("--redis-password", default="", help="Redis password for server")
    parser.add_argument("--sqlite-path", help="SQLite database path")

    parser.add_argument("--record-miss", action="store_true", help="Deprecated: upstream responses are cached automatically")
    parser.add_argument("--record-overwrite", action="store_true", help="Overwrite stored entries while recording")
    parser.add_argument("--upstream", default="", help="Upstream base URL for cache misses")

    args = parser.parse_args()

    script_dir = Path(__file__).resolve().parent
    root_dir = script_dir.parent

    dump_script = args.dump_script or os.environ.get("MITM_DUMP_SCRIPT", "")
    if not dump_script:
        print("Missing --dump-script or MITM_DUMP_SCRIPT env var", file=sys.stderr)
        return 1

    flow_file = abspath(args.flow_file)
    sqlite_path = abspath(args.sqlite_path or str(root_dir / "mitm_flows.sqlite"))
    dump_script = abspath(dump_script)

    if not Path(dump_script).is_file():
        print(f"Dump script not found: {dump_script}", file=sys.stderr)
        return 1

    if args.record_miss and not args.upstream:
        print("--upstream is required when --record-miss is set", file=sys.stderr)
        return 1

    redis_addr = args.redis_addr
    redis_db = args.redis_db
    redis_password = args.redis_password
    if args.redis_url:
        parsed_addr, parsed_db, parsed_password = parse_redis_url(args.redis_url)
        if not flag_present("--redis-addr") and parsed_addr:
            redis_addr = parsed_addr
        if not flag_present("--redis-db") and parsed_db:
            redis_db = parsed_db
        if not flag_present("--redis-password") and parsed_password:
            redis_password = parsed_password

    mitmdump_bin = os.environ.get("MITMDUMP_BIN", "mitmdump")

    env = os.environ.copy()
    env.update(
        {
            "FLOW_FILE": flow_file,
            "STORE": args.store,
            "KEY_PREFIX": args.key_prefix,
            "BATCH_SIZE": str(args.batch_size),
        }
    )
    if args.overwrite:
        env["OVERWRITE"] = "1"
    if args.include_empty:
        env["INCLUDE_EMPTY"] = "1"
    if args.include_errors:
        env["INCLUDE_ERRORS"] = "1"
    if args.store == "redis":
        env["REDIS_URL"] = args.redis_url
    else:
        env["SQLITE_PATH"] = sqlite_path

    try:
        subprocess.run([mitmdump_bin, "-s", dump_script, "-n"], check=True, env=env)
    except subprocess.CalledProcessError as exc:
        print(f"mitmdump failed: {exc}", file=sys.stderr)
        return exc.returncode

    go_args = [
        "go",
        "run",
        "./cmd/mitmredis",
        "-listen",
        args.listen,
        "-store",
        args.store,
        "-key-prefix",
        args.key_prefix,
    ]
    if args.log_not_found:
        go_args.append("-log-not-found")
    if args.store == "redis":
        go_args.extend(["-redis-addr", redis_addr, "-redis-db", str(redis_db)])
        if redis_password:
            go_args.extend(["-redis-password", redis_password])
    else:
        go_args.extend(["-sqlite-path", sqlite_path])
    if args.upstream:
        go_args.extend(["-upstream", args.upstream])
    if args.record_miss:
        go_args.append("-record-miss")
        if args.record_overwrite:
            go_args.append("-record-overwrite")

    return subprocess.call(go_args, cwd=str(root_dir))


if __name__ == "__main__":
    raise SystemExit(main())
