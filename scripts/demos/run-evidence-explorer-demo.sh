#!/usr/bin/env bash
# Run the bounded, provider-free terminal proof for the offline evidence explorer.
# Usage: run-evidence-explorer-demo.sh --binary PATH --capsule PATH --ledger PATH --repository-root PATH --destination NEW_HTML [--maximum-seconds 90]

set -euo pipefail

binary=""
capsule=""
ledger=""
repository_root=""
destination=""
maximum_seconds=90

while [[ $# -gt 0 ]]; do
    case $1 in
        --binary|--capsule|--ledger|--repository-root|--destination|--maximum-seconds)
            [[ $# -ge 2 ]] || { echo "$1 requires a value" >&2; exit 2; }
            case $1 in
                --binary) binary=$2 ;;
                --capsule) capsule=$2 ;;
                --ledger) ledger=$2 ;;
                --repository-root) repository_root=$2 ;;
                --destination) destination=$2 ;;
                --maximum-seconds) maximum_seconds=$2 ;;
            esac
            shift 2
            ;;
        --help|-h)
            sed -n '2,3p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *)
            echo "unknown option: $1" >&2
            exit 2
            ;;
    esac
done

[[ -n $binary && -n $capsule && -n $ledger && -n $repository_root && -n $destination ]] || {
    echo "--binary, --capsule, --ledger, --repository-root, and --destination are required" >&2
    exit 2
}
[[ $maximum_seconds =~ ^[1-9][0-9]*$ ]] || { echo "--maximum-seconds must be a positive integer" >&2; exit 2; }

working_directory=$PWD
absolute_path() {
    if [[ $1 = /* ]]; then
        printf '%s\n' "$1"
    else
        printf '%s/%s\n' "$working_directory" "$1"
    fi
}

binary=$(absolute_path "$binary")
capsule=$(absolute_path "$capsule")
ledger=$(absolute_path "$ledger")
repository_root=$(absolute_path "$repository_root")
destination=$(absolute_path "$destination")

[[ -x $binary ]] || { echo "--binary must identify an executable file" >&2; exit 2; }
[[ -d $capsule ]] || { echo "--capsule must identify a capsule directory" >&2; exit 2; }
[[ -f $ledger ]] || { echo "--ledger must identify a regular file" >&2; exit 2; }
[[ -d $repository_root ]] || { echo "--repository-root must identify a directory" >&2; exit 2; }
[[ ! -e $destination ]] || { echo "--destination already exists" >&2; exit 2; }
[[ -d $(dirname "$destination") ]] || { echo "--destination parent must exist" >&2; exit 2; }

temporary_root=$(mktemp -d "${TMPDIR:-/tmp}/evalwitness-terminal-demo.XXXXXX")
cleanup() {
    rm -rf "${temporary_root}"
}
trap cleanup EXIT

show_command() {
    printf '\n$'
    printf ' %q' "$@"
    printf '\n'
}

SECONDS=0
cd "$repository_root"

printf 'EvalWitness / offline claim proof\n'
printf 'Scope: one public capsule, one closed claim, zero provider calls\n'

show_command evalwitness claim verify --capsule CAPSULE --ledger LEDGER --claim CLM-011
env -i PATH="$PATH" HOME="$HOME" "$binary" claim verify \
    --capsule "$capsule" --ledger "$ledger" --claim CLM-011 \
    | tee "${temporary_root}/claim-success.json"

show_command evalwitness claim challenge --capsule CAPSULE --ledger LEDGER \
    --claim CLM-011 --challenge clm-011.denominator-deletion
env -i PATH="$PATH" HOME="$HOME" "$binary" claim challenge \
    --capsule "$capsule" --ledger "$ledger" \
    --claim CLM-011 --challenge clm-011.denominator-deletion \
    | tee "${temporary_root}/claim-challenge.json"

show_command evalwitness capsule verify --source CAPSULE
env -i PATH="$PATH" HOME="$HOME" "$binary" capsule verify \
    --source "$capsule" \
    | tee "${temporary_root}/capsule-verification.json"

show_command evalwitness claim render --capsule CAPSULE --ledger LEDGER \
    --repository-root REPOSITORY_ROOT --destination NEW_HTML
env -i PATH="$PATH" HOME="$HOME" "$binary" claim render \
    --capsule "$capsule" --ledger "$ledger" \
    --repository-root . --destination "$destination" \
    > "${temporary_root}/render-result.json"
python3 - "${temporary_root}/render-result.json" "$destination" <<'PY'
import json
import pathlib
import sys

result_path = pathlib.Path(sys.argv[1])
expected_destination = sys.argv[2]
result = json.loads(result_path.read_text(encoding="utf-8"))
if result.get("destination") != expected_destination:
    raise SystemExit("claim render reported an unexpected destination")
result["destination"] = "NEW_HTML"
print(json.dumps(result, sort_keys=True, separators=(",", ":")))
PY

show_command evalwitness artifact scan --class public --path NEW_HTML
env -i PATH="$PATH" HOME="$HOME" "$binary" artifact scan --class public --path "$destination" \
    | tee "${temporary_root}/public-scan.json"

python3 - "$temporary_root" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])

def load(name):
    return json.loads((root / name).read_text(encoding="utf-8"))

success = load("claim-success.json")
challenge = load("claim-challenge.json")
capsule = load("capsule-verification.json")
render = load("render-result.json")
scan = load("public-scan.json")

if success.get("claim_id") != "CLM-011" or not success.get("valid") or not success.get("assertable"):
    raise SystemExit("the reference success claim is not valid and assertable")
if (
    challenge.get("challenge_id") != "clm-011.denominator-deletion"
    or not challenge.get("passed")
    or challenge.get("expected_guard") != challenge.get("observed_guard")
    or challenge.get("sealed_source_digest") != challenge.get("after_sealed_source_digest")
    or not str(challenge.get("after_state", "")).startswith("rejected:")
):
    raise SystemExit("the controlled challenge did not withdraw the claim at its registered guard")
if (
    capsule.get("schema_version") != "evalwitness.capsule-verify-report.v1"
    or not capsule.get("offline")
    or not capsule.get("capsule", {}).get("offline")
    or not capsule.get("capsule", {}).get("valid")
):
    raise SystemExit("the capsule was not verified offline")
if render.get("schema_version") != "evalwitness.claim-render-result.v1" or render.get("network_required") or render.get("provider_calls") != 0:
    raise SystemExit("the explorer render is not provider-free and offline")
if scan.get("class") != "public" or scan.get("files") != 1 or scan.get("findings"):
    raise SystemExit("the rendered explorer did not pass the public artifact gate")
PY

elapsed=$SECONDS
printf '\nProof complete: success verified, challenge withdrawn, capsule offline, HTML public-safe.\n'
printf 'Elapsed: %s seconds (limit: %s seconds)\n' "$elapsed" "$maximum_seconds"
if (( elapsed > maximum_seconds )); then
    echo "terminal proof exceeded its declared duration limit" >&2
    exit 1
fi
