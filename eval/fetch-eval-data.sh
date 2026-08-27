#!/usr/bin/env bash
# Fetch the benchmark trajectory data (Terminal-Bench 445 trajectories,
# SWE-bench Verified 1500 trajectories, ~226 MB unpacked) from the GitHub
# release assets and verify checksums.
#
# Usage:
#   eval/fetch-eval-data.sh [tag]         # via gh CLI (default: latest release)
#   EVALWITNESS_DATA_BASE_URL=https://... eval/fetch-eval-data.sh   # via curl from a mirror

set -euo pipefail
cd "$(dirname "$0")/.."

if [ -z "${EVALWITNESS_DATA_BASE_URL:-}" ] && [ -n "${LOGPROBE_DATA_BASE_URL:-}" ]; then
  EVALWITNESS_DATA_BASE_URL="${LOGPROBE_DATA_BASE_URL}"
  echo "warning: legacy LOGPROBE_DATA_BASE_URL consumed; migrate to EVALWITNESS_DATA_BASE_URL; value was not logged" >&2
fi

dest="eval/trajectories"
tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT

assets=(terminal_trajs.tar.gz swebench_verified_trajs.tar.gz SHA256SUMS-eval-data)

if [ -d "${dest}/terminal_trajs" ] && [ -d "${dest}/swebench_verified_trajs" ]; then
  echo "eval data already present at ${dest}; delete it to re-fetch"
  exit 0
fi

if [ -n "${EVALWITNESS_DATA_BASE_URL:-}" ]; then
  echo "==> downloading from ${EVALWITNESS_DATA_BASE_URL}"
  for a in "${assets[@]}"; do
    curl -fsSL -o "${tmp}/${a}" "${EVALWITNESS_DATA_BASE_URL%/}/${a}"
  done
else
  if ! command -v gh >/dev/null; then
    echo "error: gh CLI not found; install it or set EVALWITNESS_DATA_BASE_URL" >&2
    exit 1
  fi
  tag="${1:-}"
  echo "==> downloading release assets via gh ${tag:+(tag ${tag})}"
  if [ -n "${tag}" ]; then
    gh release download "${tag}" --dir "${tmp}" --pattern 'terminal_trajs.tar.gz' --pattern 'swebench_verified_trajs.tar.gz' --pattern 'SHA256SUMS-eval-data'
  else
    gh release download --dir "${tmp}" --pattern 'terminal_trajs.tar.gz' --pattern 'swebench_verified_trajs.tar.gz' --pattern 'SHA256SUMS-eval-data'
  fi
fi

echo "==> verifying checksums"
(cd "${tmp}" && shasum -a 256 -c SHA256SUMS-eval-data)

echo "==> unpacking to ${dest}"
go run ./cmd/evalwitness archive extract \
  --source "${tmp}/terminal_trajs.tar.gz" \
  --source "${tmp}/swebench_verified_trajs.tar.gz" \
  --expected-root terminal_trajs \
  --expected-root swebench_verified_trajs \
  --destination "${dest}"

echo "==> validating dataset shape"
terminal_tasks="$(find "${dest}/terminal_trajs/forge_gpt54" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')"
echo "terminal tasks: ${terminal_tasks} (expected 89)"

echo "done"
