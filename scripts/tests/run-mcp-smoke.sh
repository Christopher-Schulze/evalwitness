#!/usr/bin/env bash
# MCP end-to-end smoke test: drives the complete shipped MCP path through the
# real server + ToolHandler + verification.Service, asserting a decodeable
# Selection. Catches rot in the JSON-RPC → ToolHandler → Service chain that
# unit tests with stub dispatchers cannot detect.
#
# The Go test (internal/mcp/smoke_test.go) exercises the full path in-process
# with a fixture provider. This script runs it so a CI gate or a developer
# can invoke it from scripts/tests/ alongside the other conformance scripts.

set -euo pipefail
cd "$(dirname "$0")/../.."

if [ "$#" -ne 0 ]; then
  echo "usage: scripts/tests/run-mcp-smoke.sh" >&2
  exit 2
fi

go test -count=1 -v ./internal/mcp/ -run 'TestMCPEndToEndSmokeProducesSelection'
echo "MCP end-to-end smoke passed."
