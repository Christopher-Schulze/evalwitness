#!/usr/bin/env bash
# Deterministic no-provider smoke test for the CLI verifier path.

set -euo pipefail
cd "$(dirname "$0")/../.."

tmp_cache="$(mktemp -d)"
empty_env="${tmp_cache}/empty.env"
empty_config="${tmp_cache}/empty.json"
tmp_bin=""
trap 'rm -rf "${tmp_cache}"; if [ -n "${tmp_bin}" ]; then rm -f "${tmp_bin}"; fi' EXIT
: > "${empty_env}"
printf '{}\n' > "${empty_config}"

if [ -z "${EVALWITNESS_BIN:-}" ] && [ -n "${LOGPROBE_BIN:-}" ]; then
  EVALWITNESS_BIN="${LOGPROBE_BIN}"
  echo "warning: legacy LOGPROBE_BIN consumed; migrate to EVALWITNESS_BIN; value was not logged" >&2
fi

if [ -n "${EVALWITNESS_BIN:-}" ]; then
  evalwitness_bin="${EVALWITNESS_BIN}"
else
  tmp_bin="$(mktemp /tmp/evalwitness-replay-smoke-XXXXXX)"
  go build -o "${tmp_bin}" ./cmd/evalwitness
  evalwitness_bin="${tmp_bin}"
fi

output="$(
  EVALWITNESS_ENV_FILE="${empty_env}" \
  EVALWITNESS_CONFIG_FILE="${empty_config}" \
  EVALWITNESS_THINKING_MODE=disabled \
  EVALWITNESS_REPLAY_FROM=scripts/tests/golden-delta-replay.jsonl \
  EVALWITNESS_CACHE_DIR="${tmp_cache}" \
  "${evalwitness_bin}" verify \
    --provider replay \
    --model golden-delta \
    --base-url https://replay.invalid/v1 \
    --mode delta \
    --task @scripts/tests/sample-task.txt \
    --trajectory @scripts/tests/sample-traj-a.txt \
    --trajectory @scripts/tests/sample-traj-b.txt \
    --criteria generic \
    --n-reps 1 \
    --no-bias-mit \
    --no-cache \
    --output json
)"

printf '%s\n' "${output}"
grep -q '"schema_version": "evalwitness.delta.v2"' <<< "${output}"
grep -q '"winner": "A"' <<< "${output}"
grep -q '"conditional_score_a": 0.997' <<< "${output}"
grep -q '"conditional_score_b": 0.005' <<< "${output}"
grep -q '"schema_version": "evalwitness.score-evidence.v1"' <<< "${output}"
grep -q '"valid_score_mass": 1' <<< "${output}"
if grep -q '"score":' <<< "${output}"; then
  echo "Replay emitted a legacy naked score field." >&2
  exit 1
fi
if grep -q '"confidence":' <<< "${output}"; then
  echo "Replay emitted an uncalibrated confidence field." >&2
  exit 1
fi

echo "Replay smoke passed."
