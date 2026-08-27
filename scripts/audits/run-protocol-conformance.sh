#!/usr/bin/env bash
# Offline conformance gate for the EvalWitness verifier-audit protocol.
#
# Usage: run-protocol-conformance.sh [--format text|json]

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)
format="text"

while [[ $# -gt 0 ]]; do
    case $1 in
        --format)
            [[ $# -ge 2 ]] || { echo "--format requires text or json" >&2; exit 2; }
            format=$2
            shift 2
            ;;
        --help|-h)
            sed -n '2,4p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *)
            echo "unknown option: $1" >&2
            exit 2
            ;;
    esac
done

if [[ $format != "text" && $format != "json" ]]; then
    echo "unsupported format: $format (want text or json)" >&2
    exit 2
fi

tmp_dir=$(mktemp -d)
tmp_bin=$(mktemp /tmp/evalwitness-protocol-XXXXXX)
trap 'rm -rf "${tmp_dir}"; rm -f "${tmp_bin}"' EXIT

cd "$repo_root"
go build -o "$tmp_bin" ./cmd/evalwitness

env -i PATH="$PATH" EVALWITNESS_PROTOCOL_OFFLINE=1 \
    "$tmp_bin" protocol run --output json > "$tmp_dir/in-process.json"
env -i PATH="$PATH" EVALWITNESS_PROTOCOL_OFFLINE=1 \
    "$tmp_bin" protocol run --output json \
    --adapter "$tmp_bin" \
    --adapter-arg protocol \
    --adapter-arg reference-adapter > "$tmp_dir/subprocess.json"
env -i PATH="$PATH" EVALWITNESS_PROTOCOL_OFFLINE=1 \
    "$tmp_bin" protocol cases > "$tmp_dir/cases.json"
env -i PATH="$PATH" EVALWITNESS_PROTOCOL_OFFLINE=1 \
    "$tmp_bin" protocol schema --name reliability-extension.schema.json > "$tmp_dir/reliability-schema.json"

python3 eval/python-reference/verify_request_fingerprints.py >/dev/null

python3 - "$tmp_dir" "$format" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
output_format = sys.argv[2]

def load(name):
    with (root / name).open(encoding="utf-8") as handle:
        return json.load(handle)

in_process = load("in-process.json")
subprocess = load("subprocess.json")
cases = load("cases.json")
reliability_schema = load("reliability-schema.json")

for label, run in (("in_process", in_process), ("subprocess", subprocess)):
    if run.get("schema_version") != "evalwitness.protocol.audit-run.v1":
        raise SystemExit(f"{label}: wrong audit-run schema")
    if run.get("protocol_version") != "1.2.0" or run.get("offline") is not True:
        raise SystemExit(f"{label}: run is not current and offline")
    failures = [result for result in run["results"] if result["outcome"] != "passed"]
    if failures:
        raise SystemExit(f"{label}: non-passing cases: {failures[:3]}")
    levels = [status["conformance_level"] for status in run["capability_matrix"]["statuses"]]
    if levels != ["syntax", "deterministic_replay", "live_score_evidence", "empirical_reliability", "independent_reproduction"]:
        raise SystemExit(f"{label}: capability matrix order or coverage changed")
    for status in run["capability_matrix"]["statuses"][2:]:
        if status["not_run"] != 1 or status["passed"] or status["failed"]:
            raise SystemExit(f"{label}: unmeasured claim level was promoted: {status}")

if in_process["corpus_digest"] != subprocess["corpus_digest"]:
    raise SystemExit("in-process and subprocess adapters used different normative corpora")
if in_process["request_corpus_digest"] != subprocess["request_corpus_digest"]:
    raise SystemExit("in-process and subprocess adapters used different frozen request bytes")
if in_process["schema_artifact_digest"] != subprocess["schema_artifact_digest"]:
    raise SystemExit("in-process and subprocess adapters used different schema artifacts")
if in_process["corpus_digest"] != "b1526e257a27cf905d9a103eefdeb8a6dc56282f97558fe5710ea819d37c0ec3":
    raise SystemExit("protocol 1.2 normative corpus digest changed without a version update")
if in_process["request_corpus_digest"] != "198e4af31223975c1f2bbb76486a77486c5d82526b8ab8d286b1fb8a50398278":
    raise SystemExit("request schema 2 corpus digest changed")
if in_process["schema_artifact_digest"] != "7ec73192676d2c98dd2b9eee294ee8681d31c31c2750a2930eaae8a72e223873":
    raise SystemExit("protocol 1.2 schema-artifact digest changed without a version update")
if in_process["results"] != subprocess["results"]:
    raise SystemExit("in-process and subprocess case evidence differs")
if in_process["capability_matrix"] != subprocess["capability_matrix"]:
    raise SystemExit("in-process and subprocess capability matrices differ")
if in_process["run_digest"] != subprocess["run_digest"]:
    raise SystemExit("in-process and subprocess sealed run identities differ")

vectors = cases["cases"]
request_ids = sorted(item["case_id"] for item in vectors if item["case_id"].startswith("request."))
if request_ids != ["request.full-boundary-lineage-variant", "request.full-boundary-unicode", "request.minimal-null-empty", "request.negative-zero-normalization"]:
    raise SystemExit(f"frozen request vector set changed: {request_ids}")
required = [item for item in vectors if item["case_id"].startswith("required.")]
if len(vectors) != 188:
    raise SystemExit(f"protocol 1.2 corpus changed from frozen size 188 to {len(vectors)}")
if len(required) != 111:
    raise SystemExit(f"required-field mutation coverage changed from 111 to {len(required)}")
if not all(item["expected"]["status"] == "rejected" for item in required):
    raise SystemExit("a required-field mutation no longer expects fail-closed rejection")
if reliability_schema.get("$schema") != "https://json-schema.org/draft/2020-12/schema":
    raise SystemExit("reliability extension is not JSON Schema 2020-12")
if reliability_schema.get("additionalProperties") is not False:
    raise SystemExit("reliability extension core fields are not closed")

summary = {
    "schema_version": "evalwitness.protocol.conformance-gate.v1",
    "protocol_version": in_process["protocol_version"],
    "corpus_digest": in_process["corpus_digest"],
    "request_corpus_digest": in_process["request_corpus_digest"],
    "schema_artifact_digest": in_process["schema_artifact_digest"],
    "case_count": len(vectors),
    "required_field_mutations": len(required),
    "adapters": ["in_process", "subprocess"],
    "network_required": False,
    "status": "passed",
}
if output_format == "json":
    print(json.dumps(summary, sort_keys=True, separators=(",", ":")))
else:
    print(
        "Protocol conformance passed: "
        f"version={summary['protocol_version']} "
        f"cases={summary['case_count']} "
        f"required_field_mutations={summary['required_field_mutations']} "
        f"corpus={summary['corpus_digest']} "
        "adapters=in_process,subprocess network=not_required"
    )
PY
