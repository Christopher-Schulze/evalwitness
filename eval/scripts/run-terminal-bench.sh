#!/usr/bin/env bash
# Drive evalwitness against Terminal-Bench-2 trajectories and report paper-style
# Pass@1, oracle, and verifier-selected downstream success metrics.

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TRAJ_DIR="${ROOT}/eval/trajectories/terminal_trajs/forge_gpt54"
evalwitness="${ROOT}/evalwitness"

if [ -z "${EVALWITNESS_EVAL_CRITERIA:-}" ] && [ -n "${LOGPROBE_EVAL_CRITERIA:-}" ]; then
  EVALWITNESS_EVAL_CRITERIA="${LOGPROBE_EVAL_CRITERIA}"
  echo "warning: legacy LOGPROBE_EVAL_CRITERIA consumed; migrate to EVALWITNESS_EVAL_CRITERIA; value was not logged" >&2
fi
if [ -z "${EVALWITNESS_EVAL_NREPS:-}" ] && [ -n "${LOGPROBE_EVAL_NREPS:-}" ]; then
  EVALWITNESS_EVAL_NREPS="${LOGPROBE_EVAL_NREPS}"
  echo "warning: legacy LOGPROBE_EVAL_NREPS consumed; migrate to EVALWITNESS_EVAL_NREPS; value was not logged" >&2
fi

if [ ! -x "${evalwitness}" ]; then
  echo "build evalwitness first: go build -o evalwitness ./cmd/evalwitness" >&2
  exit 1
fi

if [ ! -d "${TRAJ_DIR}" ]; then
  echo "no trajectories at ${TRAJ_DIR}" >&2
  exit 1
fi

CRITERIA="${EVALWITNESS_EVAL_CRITERIA:-specification,output_match,error_signals}"
NREPS="${EVALWITNESS_EVAL_NREPS:-4}"

exec "${evalwitness}" eval-terminal \
  --trajs "$(basename "${TRAJ_DIR}")" \
  --criteria "${CRITERIA}" \
  --n-reps "${NREPS}" \
  "$@"
