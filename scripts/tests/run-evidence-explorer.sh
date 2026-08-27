#!/usr/bin/env bash
# Rebuild and verify the self-contained offline evidence explorer.

set -euo pipefail
cd "$(dirname "$0")/../.."

if ! command -v bun >/dev/null 2>&1; then
  echo "Bun is required to verify web/explorer" >&2
  exit 1
fi
if [ ! -x web/explorer/node_modules/.bin/vite ] || [ ! -x web/explorer/node_modules/.bin/playwright ]; then
  echo "web/explorer dependencies are missing; run: cd web/explorer && bun install --frozen-lockfile" >&2
  exit 1
fi

temporary_root="$(mktemp -d "${TMPDIR:-/tmp}/evalwitness-evidence-explorer.XXXXXX")"
cleanup() {
  rm -rf "${temporary_root}"
}
trap cleanup EXIT

echo "==> explorer source gates"
(
  cd web/explorer
  bun run check
  bun scripts/sync-assets.ts --check
)

echo "==> explorer binary and frozen reliance capsule family"
evalwitness_binary="${temporary_root}/evalwitness"
go build -o "${evalwitness_binary}" ./cmd/evalwitness
base_capsule="eval/results/evidence-reliance-base-capsule-v1"
base_ledger="eval/results/evidence-reliance-base-claims-v1.json"
reliance_capsule="${temporary_root}/reliance-capsule"
"${evalwitness_binary}" capsule build-reliance \
  --base-capsule "${base_capsule}" \
  --map eval/results/evidence-reliance-map-v1.json \
  --destination "${reliance_capsule}" \
  > "${temporary_root}/reliance-capsule-build.json"
cmp "${reliance_capsule}.claims.json" eval/results/evidence-reliance-claims-v1.json
"${evalwitness_binary}" capsule verify-reliance \
  --base-capsule "${base_capsule}" \
  --source "${reliance_capsule}" \
  --map eval/results/evidence-reliance-map-v1.json \
  --ledger eval/results/evidence-reliance-claims-v1.json \
  --profile eval/results/evidence-reliance-profile-v1.json \
  --paper eval/results/evidence-reliance-paper-v1.json \
  --explorer eval/results/evidence-reliance-explorer-v1.json \
  > "${temporary_root}/reliance-capsule-verification.json"

echo "==> canonical explorer report"
"${evalwitness_binary}" claim report \
  --capsule "${base_capsule}" \
  --ledger "${base_ledger}" \
  --reliance-capsule "${reliance_capsule}" \
  --reliance-ledger eval/results/evidence-reliance-claims-v1.json \
  --repository-root . \
  > "${temporary_root}/report.json"

echo "==> immutable self-contained render"
explorer_html="${temporary_root}/evidence-explorer.html"
"${evalwitness_binary}" claim render \
  --capsule "${base_capsule}" \
  --ledger "${base_ledger}" \
  --reliance-capsule "${reliance_capsule}" \
  --reliance-ledger eval/results/evidence-reliance-claims-v1.json \
  --repository-root . \
  --destination "${explorer_html}" \
  > "${temporary_root}/render-result.json"
"${evalwitness_binary}" artifact scan --class public --path "${explorer_html}" >/dev/null

python3 - "${temporary_root}/report.json" "${temporary_root}/render-result.json" "${explorer_html}" <<'PY'
import hashlib
import json
import pathlib
import sys

report_path, result_path, html_path = map(pathlib.Path, sys.argv[1:])
report = json.loads(report_path.read_text(encoding="utf-8"))
result = json.loads(result_path.read_text(encoding="utf-8"))
html = html_path.read_bytes()

if report["schema_version"] != "evalwitness.evidence-explorer-report.v2":
    raise SystemExit("unexpected evidence explorer report schema")
if report["scope"]["provider_calls"] != 0 or report["scope"]["empirical"]:
    raise SystemExit("explorer scope overstates provider or empirical evidence")
if report["release"]["files"] != 20 or report["release"]["files_verified"] != 20 or not report["release"]["all_files_verified"]:
    raise SystemExit("explorer release view is incomplete")
if report["challenge"]["claim_id"] != "CLM-011" or report["challenge"]["total_receipts"] != 189:
    raise SystemExit("explorer challenge view differs from the sealed pack")
owner = report["owner_inspection"]
if (
    owner["assessments"]["required"] != 66
    or owner["assessments"]["completed"] != 66
    or len(owner["dimensions"]) != 16
    or owner["outcomes"]["core_status"] != "passed"
    or owner["outcomes"]["overall_status"] != "revision_required"
    or owner["human_study_status"] != "not_run"
    or owner["external_action_status"] != "not_authorized"
):
    raise SystemExit("explorer owner-inspection lane differs from the public attestation")
availability = [entry["availability"] for entry in report["challenge"]["classes"]]
if availability.count("available") != 7 or availability.count("not_applicable") != 1:
    raise SystemExit("explorer challenge availability registry is incomplete")
stress = report["stress"]
if (
    stress["case_study_id"] != "first-listed-candidate-order-one-minimal"
    or stress["outcome"] != "violated"
    or stress["original_line_units"] != 32
    or stress["final_line_units"] != 2
    or stress["reduction_attempts"] != 53
    or stress["accepted_reductions"] != 30
    or stress["rejected_reductions"] != 23
    or stress["reduction_basis_points"] != 9375
    or stress["empirical_units"] != 0
    or stress["provider_calls"] != 0
    or stress["network_required"]
):
    raise SystemExit("explorer stress view differs from the provider-free development witness")
reliance = report["reliance"]
if (
    reliance["schema_version"] != "evalwitness.reliance-explorer-projection.v1"
    or reliance["source_tasks"] != 24
    or reliance["registered_cells"] != 1536
    or reliance["outcome_bearing_cells"] != 1536
    or reliance["excluded_cells"] != 0
    or len(reliance["outcomes"]) != 98
    or len(reliance["selectors"]) != 10
    or len(reliance["arm_contrasts"]) != 5
    or len(reliance["witnesses"]) != 1
    or reliance["witnesses"][0]["raw_trajectory_content_shown"]
    or not reliance["global_score_prohibited"]
    or reliance["provider_calls"] != 0
    or reliance["network_required"]
):
    raise SystemExit("explorer reliance view differs from the frozen capsule projection")
denominators = {
    entry["name"]: entry["value"]
    for generation in report["method"]["generations"]
    if generation["generation"] == "v3"
    for entry in generation["frozen_denominators"]
}
if {key: denominators.get(key) for key in ("natural_total_attempts", "scarcity_admitted", "scarcity_target", "scarcity_test_role")} != {
    "natural_total_attempts": 939,
    "scarcity_admitted": 3,
    "scarcity_target": 40,
    "scarcity_test_role": 0,
}:
    raise SystemExit("explorer v3 denominators differ from the sealed evidence")
if result["schema_version"] != "evalwitness.claim-render-result.v1" or result["network_required"] or result["provider_calls"] != 0:
    raise SystemExit("claim render result violates the offline contract")
if result["bytes"] != len(html) or result["html_sha256"] != hashlib.sha256(html).hexdigest():
    raise SystemExit("claim render metadata differs from the written HTML")
PY

echo "==> file-protocol browser matrix"
(
  cd web/explorer
  EVALWITNESS_EXPLORER_HTML="${explorer_html}" bun run test:e2e
)

echo "==> deterministic public presentation assets"
public_assets="${temporary_root}/public-assets"
(
  cd web/explorer
  bun run capture:assets -- --html "${explorer_html}" --destination "${public_assets}"
)

python3 - "${temporary_root}/report.json" "${temporary_root}/render-result.json" "${explorer_html}" "${public_assets}" "assets/stress-witness.png" <<'PY'
import hashlib
import json
import pathlib
import struct
import sys
import xml.etree.ElementTree as ET

report_path, result_path, html_path, assets_path, tracked_stress_path = map(pathlib.Path, sys.argv[1:])
report = json.loads(report_path.read_text(encoding="utf-8"))
result = json.loads(result_path.read_text(encoding="utf-8"))
manifest = json.loads((assets_path / "manifest.json").read_text(encoding="utf-8"))
expected_names = [
    "claim-autopsy-desktop.png",
    "claim-autopsy-mobile.png",
    "stress-lab-desktop.png",
    "evidence-reliance-desktop.png",
    "owner-inspection-desktop.png",
    "architecture.svg",
]

if manifest["schema_version"] != "evalwitness.evidence-explorer-public-assets.v2":
    raise SystemExit("unexpected public presentation asset schema")
if manifest["source_html_sha256"] != hashlib.sha256(html_path.read_bytes()).hexdigest():
    raise SystemExit("public presentation assets are not bound to the rendered HTML")
if manifest["report_digest"] != report["digest"] or manifest["renderer_digest"] != result["renderer_digest"]:
    raise SystemExit("public presentation assets are not bound to the report and renderer")
if manifest["reference_browser"] != "playwright-chromium-1.62.1":
    raise SystemExit("public presentation assets use an unexpected reference browser")
if [entry["path"] for entry in manifest["files"]] != expected_names:
    raise SystemExit("public presentation asset inventory is incomplete or unordered")

for entry in manifest["files"]:
    raw = (assets_path / entry["path"]).read_bytes()
    if entry["bytes"] != len(raw) or entry["sha256"] != hashlib.sha256(raw).hexdigest():
        raise SystemExit(f"public presentation asset identity differs: {entry['path']}")

for name, expected_dimensions in {
    "claim-autopsy-desktop.png": (1440, 1000),
    "claim-autopsy-mobile.png": (390, 844),
    "stress-lab-desktop.png": (1440, 1000),
    "evidence-reliance-desktop.png": (1440, 1000),
    "owner-inspection-desktop.png": (1440, 1000),
}.items():
    raw = (assets_path / name).read_bytes()
    if raw[:8] != b"\x89PNG\r\n\x1a\n" or struct.unpack(">II", raw[16:24]) != expected_dimensions:
        raise SystemExit(f"public presentation asset dimensions differ: {name}")

generated_stress = (assets_path / "stress-lab-desktop.png").read_bytes()
if tracked_stress_path.read_bytes() != generated_stress:
    raise SystemExit("tracked README stress witness differs from the deterministic explorer capture")

architecture = ET.parse(assets_path / "architecture.svg").getroot()
if architecture.attrib.get("viewBox") != "0 0 1600 900":
    raise SystemExit("public architecture asset has an unexpected viewBox")
PY

echo "==> bounded real-output terminal proof"
terminal_cast="${temporary_root}/evidence-explorer-proof.cast"
terminal_html="${temporary_root}/evidence-explorer-demo.html"
go run ./scripts/demos/record-terminal-demo.go \
  --destination "${terminal_cast}" -- \
  ./scripts/demos/run-evidence-explorer-demo.sh \
  --binary "${evalwitness_binary}" \
  --capsule "${base_capsule}" \
  --ledger "${base_ledger}" \
  --repository-root . \
  --destination "${terminal_html}" \
  --maximum-seconds 90 \
  > "${temporary_root}/terminal-demo.txt"
"${evalwitness_binary}" artifact scan --class public --path "${terminal_cast}" >/dev/null

python3 - "${terminal_cast}" "${terminal_html}" <<'PY'
import json
import pathlib
import sys

cast_path, private_html_path = map(pathlib.Path, sys.argv[1:])
lines = cast_path.read_text(encoding="utf-8").splitlines()
header = json.loads(lines[0])
events = [json.loads(line) for line in lines[1:]]
if header != {
    "version": 2,
    "width": 120,
    "height": 36,
    "title": "EvalWitness offline claim proof",
    "idle_time_limit": 1.5,
    "env": {"SHELL": "/bin/bash", "TERM": "xterm-256color"},
}:
    raise SystemExit("terminal proof has an unexpected asciicast header")
if not events or any(len(event) != 3 or event[1] != "o" for event in events):
    raise SystemExit("terminal proof has an invalid event stream")
times = [event[0] for event in events]
if times != sorted(times) or times[-1] > 90:
    raise SystemExit("terminal proof exceeds its bounded monotonic timeline")
output = "".join(event[2] for event in events)
if str(private_html_path) in output or '"destination":"NEW_HTML"' not in output:
    raise SystemExit("terminal proof exposes its private render destination")
for required in (
    '"claim_id":"CLM-011"',
    '"observed_guard":"expression-denominator-missing"',
    '"network_required":false',
    '"provider_calls":0',
    '"findings": []',
    "Proof complete: success verified, challenge withdrawn, capsule offline, HTML public-safe.",
):
    if required not in output:
        raise SystemExit(f"terminal proof is missing real output: {required}")
PY

echo "Evidence explorer checks passed."
