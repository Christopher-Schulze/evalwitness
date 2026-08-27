#!/usr/bin/env bash
# Paired significance test between two selection arms over the same responses.
#
# Both arms select from identical trajectory sets, so there is no sampling noise
# between them and the comparison is paired. An unpaired score difference on
# such data overstates the evidence; McNemar's exact test on the discordant
# pairs is the correct instrument.
#
# Given artifacts that all share one extraction mode, it instead tests that mode
# against the random-pick baseline — which is exactly what Pass@1 is — using an
# exact Poisson-binomial tail, since every decidable task carries its own
# probability of being solved by chance. Several artifacts are pooled, so two
# benchmarks can be combined to reach a usable sample size.
#
# Usage: run-paired-analysis.sh [--question TYPE --margin X --family-size N]
#        <details.json> [<details.json> ...]
#        two artifacts over the same tasks -> paired McNemar
#        anything else                     -> pooled test against random
#
# Any two arms over one task set are paired, whatever differs between them:
# extraction mode, selection strategy, or evidence budget.
#
# Files must be produced with --details. Works for terminal-bench (task_name)
# and SWE-bench (instance_id) artifacts.

set -euo pipefail

if [[ $# -lt 1 ]]; then
    echo "usage: $(basename "$0") [design flags] <details.json> [<details.json> ...]" >&2
    exit 2
fi

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"
exec go run ./scripts/audits/paired-analysis "$@"
