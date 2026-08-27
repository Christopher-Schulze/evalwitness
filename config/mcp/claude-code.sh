#!/usr/bin/env bash
# Registers evalwitness as a local stdio MCP server in Claude Code.

set -euo pipefail

claude mcp add evalwitness -- /absolute/path/to/evalwitness mcp-serve
