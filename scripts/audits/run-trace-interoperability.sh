#!/usr/bin/env bash
# Offline trace-interoperability gate for pinned OTLP/JSON and Agent Trace fixtures.

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)
fixture_root="$repo_root/internal/preprocess/testdata/trace"
tmp_dir=$(mktemp -d)
tmp_bin=$(mktemp /tmp/evalwitness-trace-XXXXXX)
trap 'rm -rf "${tmp_dir}"; rm -f "${tmp_bin}"' EXIT

cd "$repo_root"
go build -o "$tmp_bin" ./cmd/evalwitness

"$tmp_bin" trace inspect --source "$fixture_root/otlp-genai-1.41.0.json" --output json > "$tmp_dir/otlp-metadata.json"
"$tmp_bin" trace inspect --source "$fixture_root/agent-trace-0.1.0.json" --privacy-class attribution_authorized --output json > "$tmp_dir/agent-trace.json"
"$tmp_bin" trace export --source "$fixture_root/otlp-genai-1.41.0.json" --privacy-class content_authorized --target otlp --output artifact > "$tmp_dir/otlp-roundtrip.json"
"$tmp_bin" trace inspect --source "$tmp_dir/otlp-roundtrip.json" --privacy-class content_authorized --output json > "$tmp_dir/otlp-roundtrip-inspection.json"

python3 - "$tmp_dir" "$fixture_root/manifest.json" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
manifest_path = pathlib.Path(sys.argv[2])

def load(path):
    with pathlib.Path(path).open(encoding="utf-8") as handle:
        return json.load(handle)

metadata = load(root / "otlp-metadata.json")
agent = load(root / "agent-trace.json")
roundtrip = load(root / "otlp-roundtrip-inspection.json")
manifest = load(manifest_path)

if metadata["envelope"]["source"]["format"] != "otlp_json":
    raise SystemExit("OTLP fixture was not detected as OTLP JSON")
if metadata["envelope"]["source"]["schema_version"] != "otlp-json-1.8.0+gen-ai-1.41.0":
    raise SystemExit("OTLP fixture did not bind the pinned versions")
if metadata["envelope"]["privacy_class"] != "metadata_only" or metadata["mapping"]["totals"]["redacted"] < 3:
    raise SystemExit("metadata-first OTLP privacy accounting is incomplete")
if metadata["root_events"] != 1 or metadata["maximum_depth"] < 2:
    raise SystemExit("OTLP causal hierarchy was not retained")
if len(metadata.get("provider_usage", [])) != 1 or metadata["provider_usage"][0]["input_tokens"] != 12 or metadata["provider_usage"][0]["output_tokens"] != 7:
    raise SystemExit("missing OTLP usage was converted to zero or observed usage was not retained")
if agent["envelope"]["source"]["format"] != "agent_trace_json" or agent["envelope"]["privacy_class"] != "attribution_authorized":
    raise SystemExit("Agent Trace attribution fixture identity is wrong")
if roundtrip["envelope"]["source"]["format"] != "otlp_json" or roundtrip["canonical_events"] < 3:
    raise SystemExit("OTLP semantic round trip lost canonical events")
if len(roundtrip.get("provider_usage", [])) != 1 or roundtrip["provider_usage"][0]["input_tokens"] != 12 or roundtrip["provider_usage"][0]["output_tokens"] != 7:
    raise SystemExit("OTLP semantic round trip lost usage evidence")
if manifest.get("schema_version") != "evalwitness.trace-fixture-manifest.v1" or len(manifest.get("fixtures", [])) != 2:
    raise SystemExit("trace fixture provenance manifest is incomplete")
for fixture in manifest["fixtures"]:
    if not all(fixture.get(field) for field in ("path", "format", "version", "upstream_commit", "license", "derivation")):
        raise SystemExit(f"trace fixture provenance is incomplete: {fixture}")

print(
    "Trace interoperability passed: "
    f"otlp_events={metadata['canonical_events']} "
    f"otlp_redacted={metadata['mapping']['totals']['redacted']} "
    f"otlp_usage={len(metadata['provider_usage'])} "
    f"roundtrip_events={roundtrip['canonical_events']} "
    f"roundtrip_usage={len(roundtrip['provider_usage'])} "
    "providers=not_invoked network=not_required"
)
PY
