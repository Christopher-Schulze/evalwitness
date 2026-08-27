#!/usr/bin/env bash
# Drive evalwitness against SWE-bench Verified trajectories and report paper-style
# Pass@1, oracle, and verifier-selected downstream success metrics.

set -euo pipefail
cd "$(dirname "$0")/../.."

if [ -z "${evalwitness:-}" ] && [ -n "${logprobe:-}" ]; then
  evalwitness="${logprobe}"
  echo "warning: legacy logprobe binary override consumed; migrate to evalwitness; value was not logged" >&2
fi
if [ -z "${EVALWITNESS_N_REPS:-}" ] && [ -n "${LOGPROBE_N_REPS:-}" ]; then
  EVALWITNESS_N_REPS="${LOGPROBE_N_REPS}"
  echo "warning: legacy LOGPROBE_N_REPS consumed; migrate to EVALWITNESS_N_REPS; value was not logged" >&2
fi
if [ -z "${EVALWITNESS_MAX_WORKERS:-}" ] && [ -n "${LOGPROBE_MAX_WORKERS:-}" ]; then
  EVALWITNESS_MAX_WORKERS="${LOGPROBE_MAX_WORKERS}"
  echo "warning: legacy LOGPROBE_MAX_WORKERS consumed; migrate to EVALWITNESS_MAX_WORKERS; value was not logged" >&2
fi
evalwitness="${evalwitness:-./evalwitness}"

exec "${evalwitness}" eval-swebench \
  --root eval/trajectories/swebench_verified_trajs \
  --criteria root_cause,code_quality,test_coverage \
  --n-reps "${EVALWITNESS_N_REPS:-4}" \
  --max-workers "${EVALWITNESS_MAX_WORKERS:-16}" \
  --output text \
  "$@"
