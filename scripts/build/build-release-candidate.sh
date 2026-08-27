#!/usr/bin/env bash
# Build and locally verify one complete, non-published EvalWitness release candidate.

set -euo pipefail
cd "$(dirname "$0")/../.."

usage() {
  echo "Usage: scripts/build/build-release-candidate.sh --destination NEW_DIRECTORY [--key-file MODE_0600_FILE] [--external-publication not_authorized|authorized_by_tag]" >&2
}

destination=""
key_file=""
external_publication="not_authorized"
while [ "$#" -gt 0 ]; do
  case "$1" in
    --destination)
      [ "$#" -ge 2 ] || { usage; exit 2; }
      destination="$2"
      shift 2
      ;;
    --key-file)
      [ "$#" -ge 2 ] || { usage; exit 2; }
      key_file="$2"
      shift 2
      ;;
    --external-publication)
      [ "$#" -ge 2 ] || { usage; exit 2; }
      external_publication="$2"
      shift 2
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done

if [ -z "${destination}" ]; then
  usage
  exit 2
fi
if [ "${external_publication}" != "not_authorized" ] && [ "${external_publication}" != "authorized_by_tag" ]; then
  echo "Invalid external publication state: ${external_publication}" >&2
  exit 2
fi
if [ -e "${destination}" ] || [ -L "${destination}" ]; then
  echo "Release candidate destination already exists: ${destination}" >&2
  exit 1
fi
destination_parent="$(dirname "${destination}")"
if [ ! -d "${destination_parent}" ]; then
  echo "Release candidate parent does not exist: ${destination_parent}" >&2
  exit 1
fi

repository_root="$(pwd -P)"
destination_absolute="$(cd "${destination_parent}" && pwd -P)/$(basename "${destination}")"
case "${destination_absolute}" in
  "${repository_root}"|"${repository_root}"/*)
    echo "Release candidate destination must be outside the source repository" >&2
    exit 1
    ;;
esac

if [ -n "$(git status --porcelain=v1 --untracked-files=all)" ]; then
  echo "Release candidate requires a completely clean Git worktree" >&2
  exit 1
fi
if ! command -v bun >/dev/null 2>&1; then
  echo "Bun is required to build the release explorer" >&2
  exit 1
fi
if [ ! -x web/explorer/node_modules/.bin/vite ] || [ ! -x web/explorer/node_modules/.bin/playwright ]; then
  echo "Explorer dependencies are missing; run: cd web/explorer && bun install --frozen-lockfile" >&2
  exit 1
fi

echo "==> tracked public-source safety gate"
scripts/tests/run-artifact-safety.sh

commit="$(git rev-parse --verify HEAD)"
version="$(go run ./cmd/evalwitness version)"
if [ "${external_publication}" = "authorized_by_tag" ]; then
  tag_commit="$(git rev-parse --verify "refs/tags/v${version}^{commit}" 2>/dev/null || true)"
  if [ "${tag_commit}" != "${commit}" ]; then
    echo "Tag-authorized publication requires exact tag v${version} at HEAD" >&2
    exit 1
  fi
fi
created_at="$(python3 - "$(git show -s --format=%ct HEAD)" <<'PY'
import datetime
import sys

timestamp = int(sys.argv[1])
print(datetime.datetime.fromtimestamp(timestamp, datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"))
PY
)"

candidate_created=false
checksum_stage=""
public_scan_stage=""
cleanup() {
  if [ -n "${checksum_stage}" ] && [ -f "${checksum_stage}" ]; then
    rm -f "${checksum_stage}"
  fi
  if [ -n "${public_scan_stage}" ] && [ -f "${public_scan_stage}" ]; then
    rm -f "${public_scan_stage}"
  fi
  if [ "${candidate_created}" != true ] && [ -d "${destination_absolute}" ]; then
    rm -rf "${destination_absolute}"
  fi
}
trap cleanup EXIT

mkdir "${destination_absolute}"
assets="${destination_absolute}/assets"
mkdir -p \
  "${assets}/binary" \
  "${assets}/capsule" \
  "${assets}/documentation" \
  "${assets}/evidence" \
  "${assets}/protocol" \
  "${assets}/replication" \
  "${assets}/source"

echo "==> deterministic source archive"
source_archive="${assets}/source/evalwitness-${version}-source.tar.gz"
source_archive_report="${assets}/evidence/source-archive-report.json"
go run ./cmd/evalwitness release source-archive \
  --repository-root "${repository_root}" \
  --commit "${commit}" \
  --destination "${source_archive}" \
  > "${source_archive_report}"
source_archive_sha256="$(python3 - "${source_archive_report}" "${commit}" "${version}" <<'PY'
import json
import sys

report_path, expected_commit, expected_version = sys.argv[1:]
with open(report_path, "r", encoding="utf-8") as handle:
    report = json.load(handle)
expected = {
    "schema_version": "evalwitness.source-archive-report.v1",
    "product": "evalwitness",
    "version": expected_version,
    "git_commit": expected_commit,
    "archive_root": f"evalwitness-{expected_version}",
    "format": "ustar+gzip",
    "deterministic": True,
}
for key, value in expected.items():
    if report.get(key) != value:
        raise SystemExit(f"source archive report field {key!r} is invalid")
if not isinstance(report.get("files"), int) or report["files"] < 1:
    raise SystemExit("source archive report file count is invalid")
if not isinstance(report.get("directories"), int) or report["directories"] < 1:
    raise SystemExit("source archive report directory count is invalid")
if not isinstance(report.get("bytes"), int) or report["bytes"] < 1:
    raise SystemExit("source archive report byte count is invalid")
if not isinstance(report.get("expanded_bytes"), int) or report["expanded_bytes"] < 1:
    raise SystemExit("source archive report expanded byte count is invalid")
source_tree_digest = report.get("source_tree_digest")
if not isinstance(source_tree_digest, str) or len(source_tree_digest) != 64 or any(character not in "0123456789abcdef" for character in source_tree_digest):
    raise SystemExit("source archive report source-tree digest is invalid")
digest = report.get("sha256")
if not isinstance(digest, str) or len(digest) != 64 or any(character not in "0123456789abcdef" for character in digest):
    raise SystemExit("source archive report digest is invalid")
print(digest)
PY
)"

echo "==> portable source-tree provenance"
go run ./cmd/evalwitness release source-index \
  --repository-root "${repository_root}" \
  --destination "${assets}/source/source-tree-provenance.json" \
  > /dev/null

echo "==> offline Go module proxy"
scripts/build/build-go-module-proxy.sh --destination "${assets}/source/go-proxy"

echo "==> supported-platform binaries"
binary_stage="${destination_absolute}/.binary-stage"
scripts/build/build.sh --destination "${binary_stage}"
mv "${binary_stage}"/evalwitness-* "${assets}/binary/"
mv "${binary_stage}/SHA256SUMS" "${assets}/evidence/binary-sha256sums.txt"
rmdir "${binary_stage}"
host_binary="${assets}/binary/evalwitness-$(go env GOOS)-$(go env GOARCH)"
if [ "$(go env GOOS)" = "windows" ]; then
  host_binary="${host_binary}.exe"
fi

echo "==> public capsule and claim evidence"
capsule_root="${assets}/capsule/reference-capsule"
"${host_binary}" capsule build \
  --repository-root "${repository_root}" \
  --destination "${capsule_root}" \
  > /dev/null
"${host_binary}" capsule verify \
  --source "${capsule_root}" \
  --ledger "${capsule_root}.claims.json" \
  --statement "${capsule_root}.intoto.json" \
  --projection "${capsule_root}.projection.json" \
  --autopsy "${capsule_root}.autopsy.json" \
  > "${assets}/evidence/capsule-verification.json"
"${host_binary}" claim verify \
  --capsule "${capsule_root}" \
  --ledger "${capsule_root}.claims.json" \
  > "${assets}/evidence/claim-verification.json"
"${host_binary}" claim challenge --all \
  --capsule "${capsule_root}" \
  --ledger "${capsule_root}.claims.json" \
  > "${assets}/evidence/claim-challenge-pack.json"
"${host_binary}" claim surface verify \
  --capsule "${capsule_root}" \
  --ledger "${capsule_root}.claims.json" \
  --repository-root "${repository_root}" \
  > "${assets}/evidence/claim-surface-verification.json"

echo "==> frozen evidence-reliance capsule family"
reliance_base="${assets}/capsule/evidence-reliance-base-capsule-v1"
reliance_base_ledger="${assets}/capsule/evidence-reliance-base-claims-v1.json"
reliance_capsule="${assets}/capsule/evidence-reliance-capsule-v1"
cp -R eval/results/evidence-reliance-base-capsule-v1 "${reliance_base}"
cp eval/results/evidence-reliance-base-claims-v1.json "${reliance_base_ledger}"
(
  cd "${destination_absolute}"
  "${host_binary}" capsule build-reliance \
    --base-capsule assets/capsule/evidence-reliance-base-capsule-v1 \
    --map "${repository_root}/eval/results/evidence-reliance-map-v1.json" \
    --destination assets/capsule/evidence-reliance-capsule-v1
) > "${assets}/evidence/reliance-capsule-build.json"
cmp "${reliance_capsule}.claims.json" eval/results/evidence-reliance-claims-v1.json
"${host_binary}" capsule verify-reliance \
  --base-capsule "${reliance_base}" \
  --source "${reliance_capsule}" \
  --map eval/results/evidence-reliance-map-v1.json \
  --ledger eval/results/evidence-reliance-claims-v1.json \
  --profile eval/results/evidence-reliance-profile-v1.json \
  --paper eval/results/evidence-reliance-paper-v1.json \
  --explorer eval/results/evidence-reliance-explorer-v1.json \
  > "${assets}/evidence/reliance-capsule-verification.json"

echo "==> admitted TASK 070 exact-response evidence"
identical_base_root="evalwitness-capsule-3b26fefb5174cc63d03f47f2be5543878c287e978155116b0da6dce85d9ebf19"
identical_outer_root="evalwitness-capsule-2ba8ac4686bd1e9f5bb1f9ce530565ed02060447494b452abff8a0850ed8461b"
identical_stage="${destination_absolute}/.identical-response-stage"
mkdir "${identical_stage}"
for source in \
  eval/governance/identical-response-route-attestation-v5.json \
  eval/governance/identical-response-study-manifest-v5.json \
  eval/governance/identical-response-study-record-v5.json \
  eval/governance/identical-response-live-authorization-v5.json \
  eval/governance/identical-response-capture-run-attestation-v5.json \
  eval/governance/identical-response-research-lineage-admission-v5.json \
  eval/governance/identical-response-offline-analysis-v5.json \
  eval/governance/identical-response-bundle-policy-v5.json \
  eval/governance/identical-response-reviewed-findings-v5.json \
  eval/governance/identical-response-capsule-v5.tar.gz \
  eval/governance/identical-response-capsule-v5-outer.tar.gz \
  eval/governance/identical-response-claim-ledger-v5.json \
  eval/governance/identical-response-claim-challenge-pack-v5.json; do
  case "$(basename "${source}")" in
    identical-response-capsule-*.tar.gz)
      cp "${source}" "${assets}/capsule/$(basename "${source}")"
      ;;
    *)
      cp "${source}" "${assets}/evidence/$(basename "${source}")"
      ;;
  esac
done
cp eval/governance/identical-response-reproduction-report-v5.json \
  "${assets}/evidence/identical-response-reproduction-report-v5.json"
chmod 0644 "${assets}/evidence/identical-response-"*"-v5."* \
  "${assets}/capsule/identical-response-capsule-"*.tar.gz
python3 - \
  eval/governance/identical-response-reproduction-report-v5.json \
  "${assets}" \
  eval/governance/identical-response-live-result-v5.json \
  eval/governance/identical-response-run-budget-v5.json <<'PY'
import hashlib
import json
import pathlib
import sys

report_path = pathlib.Path(sys.argv[1])
assets_root = pathlib.Path(sys.argv[2])
evidence_root = assets_root / "evidence"
report = json.loads(report_path.read_text(encoding="utf-8"))
entries = []
for source in report["registered_artifacts"]["artifacts"]:
    source_path = pathlib.Path(source["path"])
    if source_path.name == "identical-response-capture-bai-flash-v5.jsonl":
        body = source_path.read_bytes()
        release_path = "capsule/identical-response-capsule-v5.tar.gz"
        role = "capsule"
        omission_state = "included_in_response_bundle"
    else:
        role = "capsule" if source_path.name.startswith("identical-response-capsule-") else "evidence"
        packaged = assets_root / role / source_path.name
        body = packaged.read_bytes()
        release_path = f"{role}/{packaged.name}"
        omission_state = "included"
    if len(body) != source["bytes"] or hashlib.sha256(body).hexdigest() != source["sha256"]:
        raise SystemExit(f"registered TASK 070 asset drifted: {source['path']}")
    entries.append({
        "bytes": source["bytes"],
        "omission_state": omission_state,
        "path": source["path"],
        "release_path": release_path,
        "role": role,
        "sha256": source["sha256"],
    })
reproduction_body = report_path.read_bytes()
entries.append({
    "bytes": len(reproduction_body),
    "omission_state": "included",
    "path": str(report_path),
    "release_path": f"evidence/{report_path.name}",
    "role": "evidence",
    "sha256": hashlib.sha256(reproduction_body).hexdigest(),
})
omission_reasons = {
    "identical-response-live-result-v5.json": "derived live runner output; canonical offline analysis and exact response bundle ship",
    "identical-response-run-budget-v5.json": "operational preflight record; sealed live authorization and capture-run attestation ship",
}
for raw_path in sys.argv[3:]:
    path = pathlib.Path(raw_path)
    body = path.read_bytes()
    entries.append({
        "bytes": len(body),
        "omission_reason": omission_reasons[path.name],
        "omission_state": "omitted_direct_asset",
        "path": str(path),
        "release_path": None,
        "role": "evidence",
        "sha256": hashlib.sha256(body).hexdigest(),
    })
entries.sort(key=lambda entry: entry["path"])
inventory = {
    "schema_version": "evalwitness.identical-response-release-inventory.v1",
    "capsule_id": report["capsule"]["capsule_id"],
    "included": sum(entry["omission_state"].startswith("included") for entry in entries),
    "omitted": sum(not entry["omission_state"].startswith("included") for entry in entries),
    "assets": entries,
}
destination = evidence_root / "identical-response-release-inventory-v5.json"
destination.write_text(json.dumps(inventory, sort_keys=True, separators=(",", ":")) + "\n", encoding="utf-8")
PY
(
  cd "${destination_absolute}"
  "${host_binary}" archive extract \
    --source assets/capsule/identical-response-capsule-v5.tar.gz \
    --expected-root "${identical_base_root}" \
    --destination .identical-response-stage/base
) > "${identical_stage}/base-extraction.json"
(
  cd "${destination_absolute}"
  "${host_binary}" archive extract \
    --source assets/capsule/identical-response-capsule-v5-outer.tar.gz \
    --expected-root "${identical_outer_root}" \
    --destination .identical-response-stage/outer
) > "${identical_stage}/outer-extraction.json"
python3 - \
  "${identical_stage}/base-extraction.json" \
  "${identical_stage}/base" \
  "${assets}/evidence/identical-response-base-extraction.json" \
  .identical-response-stage/base \
  "${identical_stage}/outer-extraction.json" \
  "${identical_stage}/outer" \
  "${assets}/evidence/identical-response-outer-extraction.json" \
  .identical-response-stage/outer <<'PY'
import json
import pathlib
import sys

arguments = sys.argv[1:]
for index in range(0, len(arguments), 4):
    source_path = pathlib.Path(arguments[index])
    expected_destination = arguments[index + 1]
    public_path = pathlib.Path(arguments[index + 2])
    public_destination = arguments[index + 3]
    report = json.loads(source_path.read_text(encoding="utf-8"))
    if report.get("destination") != expected_destination:
        raise SystemExit(f"archive extraction reported an unexpected destination: {source_path}")
    report["destination"] = public_destination
    with public_path.open("x", encoding="utf-8") as handle:
        json.dump(report, handle, indent=2)
        handle.write("\n")
PY
identical_base="${identical_stage}/base/${identical_base_root}"
identical_outer="${identical_stage}/outer/${identical_outer_root}"
identical_ledger="${assets}/evidence/identical-response-claim-ledger-v5.json"
identical_challenges="${assets}/evidence/identical-response-claim-challenge-pack-v5.json"
identical_reproduction="${assets}/evidence/identical-response-reproduction-report-v5.json"
"${host_binary}" replay study capsule verify \
  --base-capsule "${identical_base}" \
  --source "${identical_outer}" \
  --claim-ledger "${identical_ledger}" \
  --challenge-pack "${identical_challenges}" \
  > "${assets}/evidence/identical-response-capsule-verification.json"
"${host_binary}" claim report \
  --capsule "${reliance_base}" \
  --ledger "${reliance_base_ledger}" \
  --reliance-capsule "${reliance_capsule}" \
  --reliance-ledger eval/results/evidence-reliance-claims-v1.json \
  --identical-response-base-capsule "${identical_base}" \
  --identical-response-capsule "${identical_outer}" \
  --identical-response-ledger "${identical_ledger}" \
  --identical-response-challenge-pack "${identical_challenges}" \
  --identical-response-reproduction-report "${identical_reproduction}" \
  --repository-root "${repository_root}" \
  > "${assets}/evidence/claim-autopsy-report.json"

echo "==> controlled falsification and verification-lineage evidence"
for source in \
  eval/governance/construct-firewall-challenge-v1.json \
  eval/governance/construct-repair-evidence-v1.json \
  eval/governance/agent-only-study-v1.json \
  eval/governance/agent-only-study-schema-v1.json \
  eval/governance/controlled-corruption-v3-natural-audit.json \
  eval/governance/controlled-corruption-v3-plan.json \
  eval/governance/controlled-corruption-v3-release.json \
  eval/governance/relation-audit-plan-v3.json \
  eval/governance/relation-scarcity-sentinel-v3.json \
  eval/governance/verification-evidence-challenge-v1.json \
  eval/results/relation-owner-inspection-attestation.json \
  eval/results/relation-scarcity-negative-evidence.json \
  eval/results/relation-scarcity-negative-evidence.md \
  eval/results/evidence-reliance-map-v1.json \
  eval/results/evidence-reliance-claims-v1.json \
  eval/results/evidence-reliance-profile-v1.json \
  eval/results/evidence-reliance-paper-v1.json \
  eval/results/evidence-reliance-explorer-v1.json \
  eval/results/verification-lineage-development-dataset-card-v1.json \
  eval/results/verification-lineage-development-release-v1.json \
  eval/results/verification-lineage-limitations-v1.json \
  eval/results/verification-lineage-offline-audit-v1.json \
  eval/results/verification-lineage-same-path-loss-certificate-v1.json; do
  cp "${source}" "${assets}/evidence/$(basename "${source}")"
done

for source in \
  eval/governance/verification-lineage-adapter-conformance-v1.json \
  eval/governance/verification-lineage-capability-matrix-v1.json \
  eval/governance/verification-lineage-golden-vectors-v1.json \
  eval/governance/verification-lineage-offline-proof-v1.json \
  eval/governance/verification-lineage-parser-lock-v1.json \
  eval/governance/verification-lineage-schema-inventory-v1.json \
  eval/governance/verification-lineage-source-inventory-v1.json \
  config/mcp/generic-mcp.json; do
  cp "${source}" "${assets}/protocol/$(basename "${source}")"
done

echo "==> documentation and replication starter"
for source in CITATION.cff CONTRIBUTING.md LICENSE README.md SECURITY.md THIRD_PARTY_NOTICES.md docs/documentation.md docs/findings.md docs/releasing.md docs/spec.md; do
  cp "${source}" "${assets}/documentation/$(basename "${source}")"
done
cp .agents/skills/evalwitness-audit/SKILL.md "${assets}/replication/evalwitness-audit-skill.md"
cp docs/releasing.md "${assets}/replication/releasing.md"
cp eval/README.md "${assets}/replication/evaluation-readme.md"
cp eval/governance/trace-source-specifications-v1.json "${assets}/replication/trace-source-specifications-v1.json"
cp scripts/evals/reproduce-identical-response-v5.sh "${assets}/replication/reproduce-identical-response-v5.sh"
cp scripts/audits/run-agent-only-study.sh "${assets}/replication/run-agent-only-study.sh"

echo "==> deterministic offline Claim Autopsy"
explorer_html="${assets}/documentation/evidence-explorer.html"
"${host_binary}" claim render \
  --capsule "${reliance_base}" \
  --ledger "${reliance_base_ledger}" \
  --reliance-capsule "${reliance_capsule}" \
  --reliance-ledger eval/results/evidence-reliance-claims-v1.json \
  --identical-response-base-capsule "${identical_base}" \
  --identical-response-capsule "${identical_outer}" \
  --identical-response-ledger "${identical_ledger}" \
  --identical-response-challenge-pack "${identical_challenges}" \
  --identical-response-reproduction-report "${identical_reproduction}" \
  --repository-root "${repository_root}" \
  --destination "${explorer_html}" \
  > /dev/null
"${host_binary}" artifact scan --class public --path "${explorer_html}" >/dev/null
(
  cd web/explorer
  bun run check
  bun scripts/sync-assets.ts --check
  EVALWITNESS_EXPLORER_HTML="${explorer_html}" bun run test:e2e
  bun run capture:assets -- --html "${explorer_html}" --destination "${assets}/documentation/explorer-media"
)
rm -rf "${identical_stage}"

go run ./scripts/demos/record-terminal-demo.go \
  --destination "${destination_absolute}/.terminal-proof.cast" -- \
  ./scripts/demos/run-evidence-explorer-demo.sh \
  --binary "${host_binary}" \
  --capsule "${capsule_root}" \
  --ledger "${capsule_root}.claims.json" \
  --repository-root "${repository_root}" \
  --destination "${destination_absolute}/.terminal-explorer.html" \
  --maximum-seconds 90 \
  > /dev/null
"${host_binary}" artifact scan --class public --path "${destination_absolute}/.terminal-proof.cast" >/dev/null
rm "${destination_absolute}/.terminal-explorer.html" "${destination_absolute}/.terminal-proof.cast"
cp scripts/demos/run-evidence-explorer-demo.sh "${assets}/replication/run-evidence-explorer-demo.sh"

echo "==> release manifest, SPDX SBOM, and in-toto statement"
manifest_path="${destination_absolute}/release-manifest.json"
sbom_path="${destination_absolute}/evalwitness.spdx.json"
statement_path="${destination_absolute}/release.intoto.json"
"${host_binary}" release manifest \
  --assets "${assets}" \
  --commit "${commit}" \
  --source-archive-sha256 "${source_archive_sha256}" \
  --created "${created_at}" \
  --external-publication "${external_publication}" \
  --destination "${manifest_path}" \
  > /dev/null
"${host_binary}" release sbom \
  --assets "${assets}" \
  --manifest "${manifest_path}" \
  --destination "${sbom_path}" \
  > /dev/null
"${host_binary}" release statement \
  --manifest "${manifest_path}" \
  --sbom "${sbom_path}" \
  --destination "${statement_path}" \
  > /dev/null

verify_signature_args=(--allow-unsigned-development)
if [ -n "${key_file}" ]; then
  signature_path="${destination_absolute}/signature"
  "${host_binary}" release sign \
    --manifest "${manifest_path}" \
    --sbom "${sbom_path}" \
    --statement "${statement_path}" \
    --key-file "${key_file}" \
    --destination "${signature_path}" \
    > /dev/null
  verify_signature_args=(--signature "${signature_path}")
fi

echo "==> complete local release verification"
"${host_binary}" release verify \
  --assets "${assets}" \
  --manifest "${manifest_path}" \
  --sbom "${sbom_path}" \
  --statement "${statement_path}" \
  "${verify_signature_args[@]}" \
  > "${destination_absolute}/verification.json"

echo "==> archive-only local-proxy round trip"
scripts/tests/run-release-roundtrip.sh --candidate "${destination_absolute}" \
  > "${destination_absolute}/roundtrip-verification.json"

public_scan_stage="$(mktemp "$(dirname "${destination_absolute}")/.evalwitness-public-scan.XXXXXX")"
if ! "${host_binary}" artifact scan --class public --path "${destination_absolute}" > "${public_scan_stage}"; then
  cat "${public_scan_stage}" >&2
  exit 1
fi
mv "${public_scan_stage}" "${destination_absolute}/public-scan.json"
public_scan_stage=""

checksum_stage="$(mktemp "$(dirname "${destination_absolute}")/.evalwitness-sha256sums.XXXXXX")"
(
  cd "${destination_absolute}"
  find . -type f ! -name SHA256SUMS -print | LC_ALL=C sort | while IFS= read -r path; do
    shasum -a 256 "${path}"
  done > "${checksum_stage}"
  mv "${checksum_stage}" SHA256SUMS
  shasum -a 256 -c SHA256SUMS >/dev/null
)
checksum_stage=""

if [ -n "$(git status --porcelain=v1 --untracked-files=all)" ]; then
  echo "Release build mutated the source worktree" >&2
  exit 1
fi

candidate_created=true
echo "Release candidate verified: destination=${destination_absolute} version=${version} commit=${commit} signed=$([ -n "${key_file}" ] && echo true || echo false) external_publication=${external_publication}"
