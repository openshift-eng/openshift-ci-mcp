#!/usr/bin/env python3
"""Smoke tests for openshift-ci-mcp server.

Starts the MCP server over stdio, calls every tool, and validates responses.
Requires network access to Sippy, Release Controller, and Search.CI.

Usage:
    python tests/smoke_test.py --binary bin/openshift-ci-mcp
    python tests/smoke_test.py --container quay.io/rh-edge-enablement/openshift-ci-mcp:latest
    python tests/smoke_test.py --binary bin/openshift-ci-mcp --tools get_releases get_variants
"""

import argparse
import json
import select
import subprocess
import sys
import threading
import time

CONFIG = {"release": "4.18", "timeout": 30}

PASS = "\033[92mPASS\033[0m"
FAIL = "\033[91mFAIL\033[0m"
SKIP = "\033[93mSKIP\033[0m"
BOLD = "\033[1m"
RESET = "\033[0m"


class MCPClient:
    """Communicates with an MCP server over stdio using JSON-RPC."""

    def __init__(self, cmd):
        self.proc = subprocess.Popen(
            cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        )
        self._id = 0
        self._stderr_lines = []
        self._stderr_thread = threading.Thread(target=self._drain_stderr, daemon=True)
        self._stderr_thread.start()
        self._initialize()

    def _drain_stderr(self):
        for line in self.proc.stderr:
            self._stderr_lines.append(line.decode().rstrip())

    def _next_id(self):
        self._id += 1
        return self._id

    def _send(self, msg):
        self.proc.stdin.write(json.dumps(msg).encode() + b"\n")
        self.proc.stdin.flush()

    def _recv(self, timeout=None):
        if timeout is None:
            timeout = CONFIG["timeout"]
        ready, _, _ = select.select([self.proc.stdout], [], [], timeout)
        if not ready:
            raise TimeoutError(f"no response within {timeout}s")
        line = self.proc.stdout.readline()
        if not line:
            stderr = "\n".join(self._stderr_lines[-20:])
            raise ConnectionError(f"server closed connection. stderr:\n{stderr}")
        return json.loads(line)

    def _initialize(self):
        self._send({
            "jsonrpc": "2.0", "id": self._next_id(), "method": "initialize",
            "params": {
                "protocolVersion": "2024-11-05", "capabilities": {},
                "clientInfo": {"name": "smoke-test", "version": "0.1.0"},
            },
        })
        resp = self._recv()
        info = resp["result"]["serverInfo"]
        assert info["name"] == "openshift-ci-mcp", f"unexpected server: {info}"
        self._send({"jsonrpc": "2.0", "method": "notifications/initialized"})

    def list_tools(self):
        self._send({
            "jsonrpc": "2.0", "id": self._next_id(),
            "method": "tools/list", "params": {},
        })
        return self._recv()["result"]["tools"]

    def call_tool(self, name, args=None):
        self._send({
            "jsonrpc": "2.0", "id": self._next_id(), "method": "tools/call",
            "params": {"name": name, "arguments": args or {}},
        })
        result = self._recv()["result"]
        text = result["content"][0]["text"]
        is_error = result.get("isError", False)
        try:
            return json.loads(text), is_error
        except json.JSONDecodeError:
            return text, is_error

    def close(self):
        try:
            self.proc.stdin.close()
            self.proc.wait(timeout=5)
        except Exception:
            self.proc.kill()


class SmokeTests:
    def __init__(self, client, tool_filter=None):
        self.client = client
        self.tool_filter = tool_filter
        self.ctx = {}
        self.results = []

    def _should(self, name):
        return self.tool_filter is None or name in self.tool_filter

    def _test(self, name, fn, skip_reason=None):
        if skip_reason:
            self.results.append(("SKIP", name, skip_reason))
            print(f"  {SKIP} {name}: {skip_reason}")
            return
        t0 = time.time()
        try:
            fn()
            self.results.append(("PASS", name, None))
            print(f"  {PASS} {name} ({time.time() - t0:.1f}s)")
        except Exception as e:
            self.results.append(("FAIL", name, str(e)))
            print(f"  {FAIL} {name} ({time.time() - t0:.1f}s): {e}")

    def _section(self, title):
        print(f"\n{BOLD}{title}{RESET}")

    def run_all(self):
        self._test_protocol()
        self._test_releases()
        self._test_variants()
        self._test_jobs()
        self._test_tests()
        self._test_component_readiness()
        self._test_payloads()
        self._test_pull_requests()
        self._test_search()
        self._test_proxy()
        return self._summary()

    def _test_protocol(self):
        self._section("Protocol")

        def t():
            tools = self.client.list_tools()
            names = {t["name"] for t in tools}
            assert len(tools) == 21, f"expected 21 tools, got {len(tools)}"
            for n in ["get_releases", "get_job_report", "sippy_api", "search_ci_logs"]:
                assert n in names, f"missing tool: {n}"

        self._test("list_tools (21 registered)", t)

    def _test_releases(self):
        self._section("Releases")

        if self._should("get_releases"):
            def t():
                data, err = self.client.call_tool("get_releases")
                assert not err, data
                releases = data.get("releases", [])
                assert isinstance(releases, list) and len(releases) > 0
            self._test("get_releases", t)

        if self._should("get_release_health"):
            def t():
                data, err = self.client.call_tool("get_release_health", {"release": CONFIG["release"]})
                assert not err, data
                health = data.get("health", data)
                assert "indicators" in health, "missing indicators"
            self._test("get_release_health", t)

    def _test_variants(self):
        self._section("Variants")

        if self._should("get_variants"):
            def t():
                data, err = self.client.call_tool("get_variants")
                assert not err, data
                variants = data.get("variants", {})
                assert isinstance(variants, dict) and len(variants) > 0
            self._test("get_variants", t)

    def _test_jobs(self):
        self._section("Jobs")

        if self._should("get_job_report"):
            def t():
                data, err = self.client.call_tool("get_job_report", {
                    "release": CONFIG["release"], "platform": "aws", "limit": 5,
                })
                assert not err, data
                assert isinstance(data, list) and len(data) > 0, "no jobs returned"
                self.ctx["job_name"] = data[0].get("name") or data[0].get("briefName")
            self._test("get_job_report", t)

            def t():
                data, err = self.client.call_tool("get_job_report", {
                    "release": CONFIG["release"], "arch": "amd64", "topology": "ha",
                    "network": "ovn", "limit": 3,
                })
                assert not err, data
                assert isinstance(data, list)
            self._test("get_job_report (variant filter)", t)

        if self._should("get_job_runs"):
            def t():
                data, err = self.client.call_tool("get_job_runs", {
                    "release": CONFIG["release"], "job_name": self.ctx["job_name"], "limit": 3,
                })
                assert not err, data
                rows = data.get("rows", data) if isinstance(data, dict) else data
                if isinstance(rows, list) and rows:
                    run = rows[0]
                    run_id = run.get("prow_id") or run.get("prowJobRunID") or run.get("id")
                    if run_id:
                        self.ctx["prow_job_run_id"] = str(run_id)
            self._test("get_job_runs", t,
                        skip_reason=None if self.ctx.get("job_name") else "no job_name from get_job_report")

        if self._should("get_job_run_summary"):
            def t():
                data, err = self.client.call_tool("get_job_run_summary", {
                    "prow_job_run_id": self.ctx["prow_job_run_id"],
                })
                assert not err, data
            self._test("get_job_run_summary", t,
                        skip_reason=None if self.ctx.get("prow_job_run_id") else "no prow_job_run_id from get_job_runs")

    def _test_tests(self):
        self._section("Tests")

        if self._should("get_test_report"):
            def t():
                data, err = self.client.call_tool("get_test_report", {
                    "release": CONFIG["release"], "limit": 5,
                })
                assert not err, data
                assert isinstance(data, list) and len(data) > 0, "no tests returned"
                self.ctx["test_name"] = data[0].get("name") or data[0].get("testName", "")
            self._test("get_test_report", t)

        if self._should("get_test_details"):
            def t():
                data, err = self.client.call_tool("get_test_details", {
                    "release": CONFIG["release"], "test_name": self.ctx["test_name"],
                })
                assert not err, data
            self._test("get_test_details", t,
                        skip_reason=None if self.ctx.get("test_name") else "no test_name from get_test_report")

        if self._should("get_recent_test_failures"):
            def t():
                data, err = self.client.call_tool("get_recent_test_failures", {
                    "release": CONFIG["release"], "period": "24h",
                })
                assert not err, data
            self._test("get_recent_test_failures", t)

    def _test_component_readiness(self):
        self._section("Component Readiness")

        if self._should("get_component_readiness"):
            def t():
                data, err = self.client.call_tool("get_component_readiness", {"release": CONFIG["release"]})
                assert not err, data
            self._test("get_component_readiness", t)

        if self._should("get_regressions"):
            def t():
                data, err = self.client.call_tool("get_regressions", {"release": CONFIG["release"]})
                assert not err, data
                regs = data if isinstance(data, list) else data.get("regressions", [])
                if regs:
                    self.ctx["regression_id"] = str(regs[0].get("id") or regs[0].get("regressionId", ""))
            self._test("get_regressions", t)

        if self._should("get_regression_detail"):
            def t():
                data, err = self.client.call_tool("get_regression_detail", {
                    "regression_id": self.ctx["regression_id"],
                })
                assert not err, data
            self._test("get_regression_detail", t,
                        skip_reason=None if self.ctx.get("regression_id") else "no regression_id from get_regressions")

    def _test_payloads(self):
        self._section("Payloads")

        if self._should("get_payload_status"):
            def t():
                data, err = self.client.call_tool("get_payload_status", {"release": CONFIG["release"]})
                assert not err, data
                tags = data.get("tags", [])
                assert isinstance(tags, list) and len(tags) > 0, "no payload tags"
                self.ctx["payload_tag"] = tags[0].get("name", "")
            self._test("get_payload_status", t)

        if self._should("get_payload_diff"):
            def t():
                data, err = self.client.call_tool("get_payload_diff", {
                    "release": CONFIG["release"], "to_tag": self.ctx["payload_tag"],
                })
                assert not err, data
            self._test("get_payload_diff", t,
                        skip_reason=None if self.ctx.get("payload_tag") else "no payload_tag from get_payload_status")

        if self._should("get_payload_test_failures"):
            def t():
                args = {"release": CONFIG["release"]}
                if self.ctx.get("payload_tag"):
                    args["payload_tag"] = self.ctx["payload_tag"]
                data, err = self.client.call_tool("get_payload_test_failures", args)
                assert not err, data
            self._test("get_payload_test_failures", t)

    def _test_pull_requests(self):
        self._section("Pull Requests")

        if self._should("get_pull_requests"):
            def t():
                data, err = self.client.call_tool("get_pull_requests", {
                    "org": "openshift", "limit": 5,
                })
                assert not err, data
                assert isinstance(data, list), f"expected list, got {type(data)}"
                if data:
                    pr = data[0]
                    self.ctx["pr_org"] = pr.get("org", "openshift")
                    self.ctx["pr_repo"] = pr.get("repo", "")
                    self.ctx["pr_number"] = str(pr.get("number") or pr.get("prNumber", ""))
            self._test("get_pull_requests", t)

        if self._should("get_pull_request_impact"):
            def t():
                data, err = self.client.call_tool("get_pull_request_impact", {
                    "org": self.ctx["pr_org"], "repo": self.ctx["pr_repo"],
                    "pr_number": self.ctx["pr_number"],
                })
                assert not err, data
            has_pr = all(self.ctx.get(k) for k in ["pr_org", "pr_repo", "pr_number"])
            self._test("get_pull_request_impact", t,
                        skip_reason=None if has_pr else "no PR data from get_pull_requests")

    def _test_search(self):
        self._section("Search")

        if self._should("search_ci_logs"):
            def t():
                data, err = self.client.call_tool("search_ci_logs", {
                    "query": "operator install timeout", "max_age": "24h",
                })
                assert not err, data
            self._test("search_ci_logs", t)

    def _test_proxy(self):
        self._section("Proxy")

        if self._should("sippy_api"):
            def t():
                data, err = self.client.call_tool("sippy_api", {"path": "/api/releases"})
                assert not err, data
            self._test("sippy_api", t)

        if self._should("release_controller_api"):
            def t():
                data, err = self.client.call_tool("release_controller_api", {
                    "path": f"/api/v1/releasestream/{CONFIG['release']}.0-0.nightly/tags",
                })
                assert not err, data
            self._test("release_controller_api", t)

        if self._should("search_ci_api"):
            def t():
                data, err = self.client.call_tool("search_ci_api", {
                    "query": "e2e-aws-ovn", "params": {"maxAge": "6h"},
                })
                assert not err, data
            self._test("search_ci_api", t)

    def _summary(self):
        passed = sum(1 for r in self.results if r[0] == "PASS")
        failed = sum(1 for r in self.results if r[0] == "FAIL")
        skipped = sum(1 for r in self.results if r[0] == "SKIP")
        total = len(self.results)
        print(f"\n{BOLD}Results: {passed}/{total} passed, {failed} failed, {skipped} skipped{RESET}")
        if failed:
            print(f"\n\033[91mFailures:\033[0m")
            for status, name, detail in self.results:
                if status == "FAIL":
                    print(f"  {name}: {detail}")
        return 1 if failed else 0


def main():
    parser = argparse.ArgumentParser(description="Smoke tests for openshift-ci-mcp")
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument("--binary", help="Path to server binary")
    group.add_argument("--container", help="Container image to test")
    parser.add_argument("--release", default=CONFIG["release"],
                        help=f"Release to test against (default: {CONFIG['release']})")
    parser.add_argument("--timeout", type=int, default=CONFIG["timeout"],
                        help=f"Per-request timeout in seconds (default: {CONFIG['timeout']})")
    parser.add_argument("--tools", nargs="+",
                        help="Only test specific tools (e.g. --tools get_releases get_variants)")
    args = parser.parse_args()

    CONFIG["release"] = args.release
    CONFIG["timeout"] = args.timeout

    cmd = [args.binary] if args.binary else ["podman", "run", "-i", "--rm", args.container]

    print(f"{BOLD}openshift-ci-mcp smoke tests{RESET}")
    print(f"Server: {' '.join(cmd)}")
    print(f"Release: {CONFIG['release']}")

    client = MCPClient(cmd)
    try:
        rc = SmokeTests(client, set(args.tools) if args.tools else None).run_all()
    finally:
        client.close()
    sys.exit(rc)


if __name__ == "__main__":
    main()
