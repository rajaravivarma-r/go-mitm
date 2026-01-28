#!/usr/bin/env python3
import json
import os
import signal
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request

EXPECTED = {
    "args": {"name": "hello"},
    "data": "",
    "files": {},
    "form": {},
    "headers": {
        "Accept": "*/*",
        "Host": "httpbin.org",
        "User-Agent": "curl/8.7.1",
        "X-Amzn-Trace-Id": "Root=1-697a0994-040811514e208084388cf736",
    },
    "json": None,
    "origin": "167.103.72.120",
    "url": "https://httpbin.org/post?name=hello",
}


def wait_for_response(url, method, timeout_seconds=30):
    deadline = time.time() + timeout_seconds
    last_error = None
    while time.time() < deadline:
        req = urllib.request.Request(url, method=method)
        try:
            with urllib.request.urlopen(req, timeout=3) as response:
                return response.read(), response.status
        except urllib.error.URLError as exc:
            last_error = exc
            time.sleep(0.5)
    raise RuntimeError(f"request failed after {timeout_seconds}s: {last_error}")


def main():
    script_dir = os.path.dirname(os.path.abspath(__file__))
    repo_root = os.path.abspath(os.path.join(script_dir, ".."))
    flow_path = os.path.abspath(os.path.join(repo_root, "testdata", "sample.flow"))
    run_script = os.path.abspath(os.path.join(script_dir, "run_replay.sh"))
    dump_script = os.environ.get("MITM_DUMP_SCRIPT")

    if not os.path.exists(flow_path):
        print(f"Missing flow file: {flow_path}")
        return 1
    if not os.path.exists(run_script):
        print(f"Missing run_replay.sh: {run_script}")
        return 1
    if not dump_script:
        print("Missing MITM_DUMP_SCRIPT env var for dump_flows_to_redis.py")
        return 1

    with tempfile.TemporaryDirectory() as temp_dir:
        sqlite_path = os.path.join(temp_dir, "mitm_flows.sqlite")
        cmd = [
            run_script,
            "--flow-file",
            flow_path,
            "--store",
            "sqlite",
            "--sqlite-path",
            sqlite_path,
            "--dump-script",
            dump_script,
        ]

        env = os.environ.copy()
        process = subprocess.Popen(
            cmd,
            cwd=script_dir,
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            preexec_fn=os.setsid,
        )

        try:
            body, status = wait_for_response(
                "http://localhost:8090/post?name=hello", "POST"
            )
            if status != 200:
                print(f"Unexpected status: {status}\n{body.decode('utf-8', 'replace')}")
                return 1

            payload = json.loads(body)
            if payload != EXPECTED:
                print("Response mismatch")
                print("Expected:")
                print(json.dumps(EXPECTED, indent=2, sort_keys=True))
                print("Actual:")
                print(json.dumps(payload, indent=2, sort_keys=True))
                return 1

            print("Replay response matches expected payload")
            return 0
        finally:
            try:
                os.killpg(process.pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass

            output = process.stdout.read() if process.stdout else ""
            if output:
                print("run_replay.sh output:")
                print(output)


if __name__ == "__main__":
    sys.exit(main())
