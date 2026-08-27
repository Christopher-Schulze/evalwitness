#!/usr/bin/env bash
# Exercise the production MCP binary across modern and legacy eras, then run
# the complete current Best-of-N integration inventory.

set -euo pipefail
cd "$(dirname "$0")/../.."

if [ "$#" -ne 0 ]; then
  echo "usage: scripts/tests/run-product-conformance.sh" >&2
  exit 2
fi

temporary_root="$(mktemp -d /tmp/evalwitness-product-conformance.XXXXXX)"
trap 'rm -rf "${temporary_root}"' EXIT
binary="${temporary_root}/evalwitness"
go build -trimpath -o "${binary}" ./cmd/evalwitness

python3 - "${binary}" "${temporary_root}" <<'PY'
import json
import os
import pathlib
import subprocess
import sys

binary = pathlib.Path(sys.argv[1])
root = pathlib.Path(sys.argv[2])
home = root / "home"
home.mkdir(mode=0o700)
environment = {
    "PATH": os.environ["PATH"],
    "HOME": str(home),
    "EVALWITNESS_OFFLINE": "true",
    "EVALWITNESS_PROVIDER": "ollama",
}
version = subprocess.run(
    [str(binary), "version"], check=True, text=True, capture_output=True,
    cwd=root, env=environment,
).stdout.strip()

def exchange(frames):
    process = subprocess.run(
        [str(binary), "mcp-serve"], input="\n".join(frames) + "\n",
        check=True, text=True, capture_output=True, cwd=root, env=environment,
    )
    return [json.loads(line) for line in process.stdout.splitlines() if line]

metadata = {
    "io.modelcontextprotocol/protocolVersion": "2026-07-28",
    "io.modelcontextprotocol/clientInfo": {"name": "evalwitness-conformance", "version": "1.0.0"},
    "io.modelcontextprotocol/clientCapabilities": {},
}
modern = exchange([
    json.dumps({"jsonrpc": "2.0", "id": "discover", "method": "server/discover", "params": {"_meta": metadata}}, separators=(",", ":")),
    json.dumps({"jsonrpc": "2.0", "id": "tools", "method": "tools/list", "params": {"_meta": metadata}}, separators=(",", ":")),
    json.dumps({"jsonrpc": "2.0", "id": "ping", "method": "ping", "params": {"_meta": metadata}}, separators=(",", ":")),
    json.dumps({"jsonrpc": "2.0", "id": "logging", "method": "logging/setLevel", "params": {"level": "debug", "_meta": metadata}}, separators=(",", ":")),
])
if len(modern) != 4:
    raise SystemExit("modern MCP exchange returned the wrong response count")
discovery, listing = (response["result"] for response in modern[:2])
if discovery["supportedVersions"][0] != "2026-07-28" or discovery["resultType"] != "complete":
    raise SystemExit("modern MCP discovery did not select the current stateless revision")
if listing["resultType"] != "complete" or listing["cacheScope"] != "public" or listing["ttlMs"] <= 0:
    raise SystemExit("modern tools/list omitted result or cache semantics")
server_info = listing["_meta"]["io.modelcontextprotocol/serverInfo"]
if server_info != {"name": "evalwitness", "version": version}:
    raise SystemExit("modern MCP server identity differs from the product version")
tool_names = [tool["name"] for tool in listing["tools"]]
expected = [
    "evalwitness_pairwise", "evalwitness_absolute", "evalwitness_delta",
    "logprobe_pairwise", "logprobe_absolute", "logprobe_delta",
    "evalwitness_calibration_evaluate",
]
if tool_names != expected:
    raise SystemExit(f"modern MCP tool inventory or order changed: {tool_names}")
for response in modern[2:]:
    if response.get("error", {}).get("code") != -32601:
        raise SystemExit("modern MCP accepted a method removed in 2026-07-28")

legacy = exchange([
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}',
    '{"jsonrpc":"2.0","method":"notifications/initialized"}',
    '{"jsonrpc":"2.0","id":2,"method":"tools/list"}',
])
if len(legacy) != 2 or legacy[0]["result"]["protocolVersion"] != "2025-11-25":
    raise SystemExit("legacy MCP negotiation failed")
if "resultType" in legacy[1]["result"] or [tool["name"] for tool in legacy[1]["result"]["tools"]] != expected:
    raise SystemExit("legacy MCP response shape or canonical handler inventory changed")

print(f"MCP product conformance passed: version={version} modern=2026-07-28 legacy=2025-11-25 tools={len(expected)}")
PY

bon_tests="$(go test ./cmd/evalwitness -list '^(TestBon|TestRunBon|TestApplyBon|TestPrepareBon)' | rg '^Test')"
bon_test_count="$(printf '%s\n' "${bon_tests}" | rg -c '^Test')"
if [ "${bon_test_count}" -lt 10 ]; then
  echo "Best-of-N integration inventory is incomplete: ${bon_test_count}" >&2
  exit 1
fi
go test -count=1 ./cmd/evalwitness -run '^(TestBon|TestRunBon|TestApplyBon|TestPrepareBon)'
printf 'Best-of-N integration passed: tests=%d\n' "${bon_test_count}"
