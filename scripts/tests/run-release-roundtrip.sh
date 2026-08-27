#!/usr/bin/env bash
# Rebuild and verify a candidate from only its manifest-bound asset graph and exact Go toolchain.

set -euo pipefail

usage() {
  echo "Usage: scripts/tests/run-release-roundtrip.sh --candidate RELEASE_CANDIDATE_DIRECTORY" >&2
}

if [ "$#" -ne 2 ] || [ "$1" != "--candidate" ] || [ -z "$2" ]; then
  usage
  exit 2
fi

candidate_input="$2"
if [ ! -d "${candidate_input}" ] || [ -L "${candidate_input}" ]; then
  echo "Release round trip requires a real candidate directory: ${candidate_input}" >&2
  exit 1
fi
candidate="$(cd "${candidate_input}" && pwd -P)"

host_os="$(go env GOOS)"
host_arch="$(go env GOARCH)"
host_name="evalwitness-${host_os}-${host_arch}"
if [ "${host_os}" = "windows" ]; then
  host_name="${host_name}.exe"
fi
release_gcflags=(-gcflags='github.com/Christopher-Schulze/evalwitness/...=-l')
packaged_binary="${candidate}/assets/binary/${host_name}"
manifest="${candidate}/release-manifest.json"
sbom="${candidate}/evalwitness.spdx.json"
statement="${candidate}/release.intoto.json"
source_index="${candidate}/assets/source/source-tree-provenance.json"
for required in "${packaged_binary}" "${manifest}" "${sbom}" "${statement}" "${candidate}/assets/source/go-proxy/index.json" "${source_index}"; do
  if [ ! -f "${required}" ] || [ -L "${required}" ]; then
    echo "Release round trip input is missing or linked: ${required}" >&2
    exit 1
  fi
done

IFS=$'\t' read -r version commit source_archive_sha256 < <(
  python3 - "${manifest}" <<'PY'
import json
import sys

manifest = json.load(open(sys.argv[1], encoding="utf-8"))
values = (manifest.get("product_version"), manifest.get("git_commit"), manifest.get("source_archive_sha256"))
if any(not isinstance(value, str) or not value for value in values):
    raise SystemExit("release manifest source identity is incomplete")
print("\t".join(values))
PY
)
source_root_name="evalwitness-${version}"
source_archive="${candidate}/assets/source/${source_root_name}-source.tar.gz"
if [ ! -f "${source_archive}" ] || [ -L "${source_archive}" ]; then
  echo "Release source archive is missing or linked: ${source_archive}" >&2
  exit 1
fi
actual_source_archive_sha256="$(shasum -a 256 "${source_archive}" | awk '{print $1}')"
if [ "${actual_source_archive_sha256}" != "${source_archive_sha256}" ]; then
  echo "Release source archive digest differs from the manifest" >&2
  exit 1
fi
if [ "$("${packaged_binary}" version)" != "${version}" ]; then
  echo "Packaged binary version differs from the release manifest" >&2
  exit 1
fi

temporary_root="$(mktemp -d /tmp/evalwitness-release-roundtrip.XXXXXX)"
cleanup() {
  chmod -R u+w "${temporary_root}" 2>/dev/null || true
  rm -rf "${temporary_root}"
}
trap cleanup EXIT
mkdir -m 0700 "${temporary_root}/home" "${temporary_root}/go-cache" "${temporary_root}/module-cache"
extracted="${temporary_root}/source"
"${packaged_binary}" archive extract \
  --source "${source_archive}" \
  --expected-root "${source_root_name}" \
  --destination "${extracted}" \
  > "${temporary_root}/archive-extraction.json"
source_root="${extracted}/${source_root_name}"
if [ ! -f "${source_root}/go.mod" ] || [ -L "${source_root}/go.mod" ]; then
  echo "Extracted release source has no real go.mod" >&2
  exit 1
fi
trajectory_data_status="absent"
if [ -e "${source_root}/eval/trajectories" ]; then
  if [ -d "${source_root}/eval/trajectories/terminal_trajs/forge_gpt54" ] &&
    [ -d "${source_root}/eval/trajectories/swebench_verified_trajs" ]; then
    trajectory_data_status="present"
  else
    echo "Extracted release source contains an incomplete fetched trajectory cache" >&2
    exit 1
  fi
fi

# Some unit tests exercise worktree-only behavior. This synthetic repository is
# an ephemeral harness, not release provenance; capsule/build provenance remains
# bound to the manifest-bound portable source index below.
git -C "${source_root}" init --quiet
git -C "${source_root}" config user.name "EvalWitness Archive Round Trip"
git -C "${source_root}" config user.email "archive-roundtrip@evalwitness.invalid"
git -C "${source_root}" add --all
git -C "${source_root}" commit --quiet -m "synthetic archive test harness"
if [ -n "$(git -C "${source_root}" status --porcelain=v1 --untracked-files=all)" ]; then
  echo "Synthetic archive test harness is not clean" >&2
  exit 1
fi

proxy_uri="$(python3 - "${candidate}/assets/source/go-proxy" <<'PY'
import pathlib
import sys
print(pathlib.Path(sys.argv[1]).resolve(strict=True).as_uri())
PY
)"
offline_environment=(
  env
  "HOME=${temporary_root}/home"
  "EVALWITNESS_HOME=${temporary_root}/evalwitness-home"
  "EVALWITNESS_REPOSITORY_ROOT=${source_root}"
  "EVALWITNESS_SOURCE_TREE_PROVENANCE=${source_index}"
  "GOCACHE=${temporary_root}/go-cache"
  "GOMODCACHE=${temporary_root}/module-cache"
  "GONOPROXY=none"
  "GOPROXY=${proxy_uri}"
  "GOSUMDB=off"
  "GOTOOLCHAIN=local"
)

project_packages=()
while IFS= read -r package; do
  project_packages[${#project_packages[@]}]="${package}"
done < <(
  cd "${source_root}"
  "${offline_environment[@]}" go list -mod=readonly ./... | awk '$0 !~ /\/internal\/stress$/'
)
if [ "${#project_packages[@]}" -eq 0 ]; then
  echo "Release round trip found no default-test project packages" >&2
  exit 1
fi
if ! (
  cd "${source_root}"
  "${offline_environment[@]}" go test -mod=readonly "${project_packages[@]}"
) > "${temporary_root}/go-test.log" 2>&1; then
  cat "${temporary_root}/go-test.log" >&2
  exit 1
fi
if ! (
  cd "${source_root}"
  "${offline_environment[@]}" bash scripts/tests/run-artifact-safety.sh
) > "${temporary_root}/artifact-safety.log" 2>&1; then
  cat "${temporary_root}/artifact-safety.log" >&2
  exit 1
fi
if [ -n "$(git -C "${source_root}" status --porcelain=v1 --untracked-files=all)" ]; then
  echo "Archive-only tests mutated the verified source tree" >&2
  exit 1
fi

rebuilt_binary="${temporary_root}/${host_name}"
(
  cd "${source_root}"
  "${offline_environment[@]}" CGO_ENABLED=0 GOOS="${host_os}" GOARCH="${host_arch}" \
    go build -mod=readonly -buildvcs=false -ldflags="-s -w -buildid= -funcalign=8" -trimpath "${release_gcflags[@]}" -o "${rebuilt_binary}" ./cmd/evalwitness
)
if [ "$("${rebuilt_binary}" version)" != "${version}" ]; then
  echo "Archive-only rebuilt binary version differs from the manifest" >&2
  exit 1
fi
if ! cmp "${packaged_binary}" "${rebuilt_binary}"; then
  echo "Archive-only rebuilt binary is not byte-identical to the packaged host binary" >&2
  exit 1
fi

signature_arguments=(--allow-unsigned-development)
if [ -d "${candidate}/signature" ] && [ ! -L "${candidate}/signature" ]; then
  signature_arguments=(--signature "${candidate}/signature")
elif [ -e "${candidate}/signature" ] || [ -L "${candidate}/signature" ]; then
  echo "Release signature input is not a real directory" >&2
  exit 1
fi
"${rebuilt_binary}" release verify \
  --assets "${candidate}/assets" \
  --manifest "${manifest}" \
  --sbom "${sbom}" \
  --statement "${statement}" \
  "${signature_arguments[@]}" \
  > "${temporary_root}/release-verification.json"

capsule="${candidate}/assets/capsule/reference-capsule"
ledger="${candidate}/assets/capsule/reference-capsule.claims.json"
"${rebuilt_binary}" capsule verify \
  --source "${capsule}" \
  --ledger "${ledger}" \
  --statement "${candidate}/assets/capsule/reference-capsule.intoto.json" \
  --projection "${candidate}/assets/capsule/reference-capsule.projection.json" \
  --autopsy "${candidate}/assets/capsule/reference-capsule.autopsy.json" \
  > "${temporary_root}/capsule-verification.json"
cmp "${temporary_root}/capsule-verification.json" "${candidate}/assets/evidence/capsule-verification.json"

"${rebuilt_binary}" claim verify --capsule "${capsule}" --ledger "${ledger}" \
  > "${temporary_root}/claim-verification.json"
cmp "${temporary_root}/claim-verification.json" "${candidate}/assets/evidence/claim-verification.json"
"${rebuilt_binary}" claim challenge --all --capsule "${capsule}" --ledger "${ledger}" \
  > "${temporary_root}/claim-challenge-pack.json"
cmp "${temporary_root}/claim-challenge-pack.json" "${candidate}/assets/evidence/claim-challenge-pack.json"
"${rebuilt_binary}" claim surface verify \
  --capsule "${capsule}" --ledger "${ledger}" --repository-root "${source_root}" \
  > "${temporary_root}/claim-surface-verification.json"
cmp "${temporary_root}/claim-surface-verification.json" "${candidate}/assets/evidence/claim-surface-verification.json"

identical_base_root="evalwitness-capsule-3b26fefb5174cc63d03f47f2be5543878c287e978155116b0da6dce85d9ebf19"
identical_outer_root="evalwitness-capsule-2ba8ac4686bd1e9f5bb1f9ce530565ed02060447494b452abff8a0850ed8461b"
identical_base_stage="${temporary_root}/identical-response-base"
identical_outer_stage="${temporary_root}/identical-response-outer"
"${rebuilt_binary}" archive extract \
  --source "${candidate}/assets/capsule/identical-response-capsule-v5.tar.gz" \
  --expected-root "${identical_base_root}" \
  --destination "${identical_base_stage}" \
  > "${temporary_root}/identical-response-base-extraction.json"
"${rebuilt_binary}" archive extract \
  --source "${candidate}/assets/capsule/identical-response-capsule-v5-outer.tar.gz" \
  --expected-root "${identical_outer_root}" \
  --destination "${identical_outer_stage}" \
  > "${temporary_root}/identical-response-outer-extraction.json"
identical_base="${identical_base_stage}/${identical_base_root}"
identical_outer="${identical_outer_stage}/${identical_outer_root}"
identical_ledger="${candidate}/assets/evidence/identical-response-claim-ledger-v5.json"
identical_challenges="${candidate}/assets/evidence/identical-response-claim-challenge-pack-v5.json"
identical_reproduction="${candidate}/assets/evidence/identical-response-reproduction-report-v5.json"
"${rebuilt_binary}" replay study capsule verify \
  --base-capsule "${identical_base}" \
  --source "${identical_outer}" \
  --claim-ledger "${identical_ledger}" \
  --challenge-pack "${identical_challenges}" \
  > "${temporary_root}/identical-response-capsule-verification.json"
cmp "${temporary_root}/identical-response-capsule-verification.json" \
  "${candidate}/assets/evidence/identical-response-capsule-verification.json"

reliance_base="${candidate}/assets/capsule/evidence-reliance-base-capsule-v1"
reliance_ledger="${candidate}/assets/capsule/evidence-reliance-base-claims-v1.json"
reliance_capsule="${candidate}/assets/capsule/evidence-reliance-capsule-v1"
reliance_claims="${candidate}/assets/evidence/evidence-reliance-claims-v1.json"
"${rebuilt_binary}" claim report \
  --capsule "${reliance_base}" --ledger "${reliance_ledger}" \
  --reliance-capsule "${reliance_capsule}" --reliance-ledger "${reliance_claims}" \
  --identical-response-base-capsule "${identical_base}" \
  --identical-response-capsule "${identical_outer}" \
  --identical-response-ledger "${identical_ledger}" \
  --identical-response-challenge-pack "${identical_challenges}" \
  --identical-response-reproduction-report "${identical_reproduction}" \
  --repository-root "${source_root}" \
  > "${temporary_root}/claim-autopsy-report.json"
cmp "${temporary_root}/claim-autopsy-report.json" "${candidate}/assets/evidence/claim-autopsy-report.json"
"${rebuilt_binary}" claim render \
  --capsule "${reliance_base}" --ledger "${reliance_ledger}" \
  --reliance-capsule "${reliance_capsule}" --reliance-ledger "${reliance_claims}" \
  --identical-response-base-capsule "${identical_base}" \
  --identical-response-capsule "${identical_outer}" \
  --identical-response-ledger "${identical_ledger}" \
  --identical-response-challenge-pack "${identical_challenges}" \
  --identical-response-reproduction-report "${identical_reproduction}" \
  --repository-root "${source_root}" \
  --destination "${temporary_root}/evidence-explorer.html" \
  > /dev/null
cmp "${temporary_root}/evidence-explorer.html" "${candidate}/assets/documentation/evidence-explorer.html"

packaged_binary_sha256="$(shasum -a 256 "${packaged_binary}" | awk '{print $1}')"
rebuilt_binary_sha256="$(shasum -a 256 "${rebuilt_binary}" | awk '{print $1}')"
python3 - \
  "${version}" "${commit}" "${source_archive_sha256}" \
  "${candidate}/assets/source/go-proxy/index.json" "$(go version)" \
  "${host_os}" "${host_arch}" "${packaged_binary_sha256}" "${rebuilt_binary_sha256}" \
  "${trajectory_data_status}" <<'PY'
import json
import sys

proxy = json.load(open(sys.argv[4], encoding="utf-8"))
report = {
    "schema_version": "evalwitness.release-roundtrip-verification.v1",
    "product": "evalwitness",
    "product_version": sys.argv[1],
    "git_commit": sys.argv[2],
    "source_archive_sha256": sys.argv[3],
    "go_module_proxy": {
        "schema_version": proxy["schema_version"],
        "modules": proxy["module_count"],
        "files": proxy["file_count"],
        "network_required": False,
    },
    "go_version": sys.argv[5],
    "host_target": {"os": sys.argv[6], "arch": sys.argv[7]},
    "packaged_binary_sha256": sys.argv[8],
    "rebuilt_binary_sha256": sys.argv[9],
    "byte_identical": sys.argv[8] == sys.argv[9],
    "fetched_trajectory_data": sys.argv[10],
    "tests": "passed",
    "artifact_safety": "passed",
    "release_assets_verified": True,
    "capsule_verified": True,
    "claims_verified": True,
    "claim_challenges_reproduced": True,
    "claim_surfaces_reproduced": True,
    "claim_autopsy_reproduced": True,
    "explorer_reproduced": True,
    "identical_response_capsule_verified": True,
    "identical_response_explorer_reproduced": True,
    "provider_calls": 0,
    "valid": True,
}
print(json.dumps(report, sort_keys=True, separators=(",", ":")))
PY
