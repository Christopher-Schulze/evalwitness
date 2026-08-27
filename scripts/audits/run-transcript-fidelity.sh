#!/usr/bin/env bash
# Offline, cross-format transcript conservation and evidence-budget audit.
#
# Usage:
#   run-transcript-fidelity.sh [--format json|text] [--budgets 16384,32768,65536]
#                              [--fuzz-smoke] [trajectory-file ...]
#
# With no trajectory files, the public golden corpus is audited. The command
# never loads provider configuration and never performs a network request.

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)
format="json"
budgets="16384,32768,65536"
fuzz_smoke=false
sources=()

while [[ $# -gt 0 ]]; do
    case $1 in
        --format)
            [[ $# -ge 2 ]] || { echo "--format requires json or text" >&2; exit 2; }
            format=$2
            shift 2
            ;;
        --budgets)
            [[ $# -ge 2 ]] || { echo "--budgets requires a comma-separated list" >&2; exit 2; }
            budgets=$2
            shift 2
            ;;
        --fuzz-smoke)
            fuzz_smoke=true
            shift
            ;;
        --help|-h)
            sed -n '2,8p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        --*)
            echo "unknown option: $1" >&2
            exit 2
            ;;
        *)
            sources+=("$1")
            shift
            ;;
    esac
done

if [[ $format != "json" && $format != "text" ]]; then
    echo "unsupported format: $format (want json or text)" >&2
    exit 2
fi

if [[ ${#sources[@]} -eq 0 ]]; then
    golden="$repo_root/internal/preprocess/testdata/golden"
    sources=(
        "$golden/claude-code.jsonl"
        "$golden/codex-rollout.jsonl"
        "$golden/opencode-export.json"
        "$golden/terminal-bench.json"
        "$golden/swe-bench.json"
        "$golden/plain.txt"
    )
fi

source_flags=()
for source in "${sources[@]}"; do
    if [[ $source != "-" ]]; then
        [[ -f $source ]] || { echo "not a file: $source" >&2; exit 2; }
    fi
    source_flags+=(--source "$source")
done

cd "$repo_root"
go run ./cmd/evalwitness fidelity --output "$format" --budgets "$budgets" "${source_flags[@]}"

if [[ $fuzz_smoke == true ]]; then
    go test ./internal/preprocess -run='^$' -fuzz=FuzzIngestNeverPanics -fuzztime=2s >&2
    go test ./internal/preprocess -run='^$' -fuzz=FuzzEvidenceBudgetNeverPanics -fuzztime=2s >&2
fi
