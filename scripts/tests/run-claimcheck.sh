#!/usr/bin/env bash
# No-provider claim gate for all locally provable README/eval claims.

set -euo pipefail
cd "$(dirname "$0")/../.."

tmp_dir="$(mktemp -d)"
tmp_bin=""
trap 'rm -rf "${tmp_dir}"; if [ -n "${tmp_bin}" ]; then rm -f "${tmp_bin}"; fi' EXIT

if [ -z "${EVALWITNESS_BIN:-}" ] && [ -n "${LOGPROBE_BIN:-}" ]; then
  EVALWITNESS_BIN="${LOGPROBE_BIN}"
  echo "warning: legacy LOGPROBE_BIN consumed; migrate to EVALWITNESS_BIN; value was not logged" >&2
fi

if [ -n "${EVALWITNESS_BIN:-}" ]; then
  evalwitness_bin="${EVALWITNESS_BIN}"
else
  tmp_bin="$(mktemp /tmp/evalwitness-claimcheck-XXXXXX)"
  go build -o "${tmp_bin}" ./cmd/evalwitness
  evalwitness_bin="${tmp_bin}"
fi
if [[ "${evalwitness_bin}" != */* ]]; then
  evalwitness_bin="$(command -v "${evalwitness_bin}")"
fi
if [[ "${evalwitness_bin}" != /* ]]; then
  evalwitness_bin="$(cd "$(dirname "${evalwitness_bin}")" && pwd -P)/$(basename "${evalwitness_bin}")"
fi

empty_env="${tmp_dir}/empty.env"
: > "${empty_env}"

echo "==> immutable reference capsule and canonical claim ledger"
"${evalwitness_bin}" capsule build \
  --repository-root . \
  --destination "${tmp_dir}/reference-capsule" \
  > "${tmp_dir}/capsule-build.json"
"${evalwitness_bin}" capsule verify \
  --source "${tmp_dir}/reference-capsule" \
  --ledger "${tmp_dir}/reference-capsule.claims.json" \
  --statement "${tmp_dir}/reference-capsule.intoto.json" \
  --projection "${tmp_dir}/reference-capsule.projection.json" \
  --autopsy "${tmp_dir}/reference-capsule.autopsy.json" \
  > "${tmp_dir}/capsule-verification.json"
"${evalwitness_bin}" claim verify \
  --capsule "${tmp_dir}/reference-capsule" \
  --ledger "${tmp_dir}/reference-capsule.claims.json" \
  > "${tmp_dir}/claim-verification.json"
"${evalwitness_bin}" claim autopsy \
  --capsule "${tmp_dir}/reference-capsule" \
  --ledger "${tmp_dir}/reference-capsule.claims.json" \
  > "${tmp_dir}/claim-autopsy.json"
"${evalwitness_bin}" claim challenge --all \
  --capsule "${tmp_dir}/reference-capsule" \
  --ledger "${tmp_dir}/reference-capsule.claims.json" \
  > "${tmp_dir}/claim-challenge-pack.json"
"${evalwitness_bin}" claim surface verify \
  --capsule "${tmp_dir}/reference-capsule" \
  --ledger "${tmp_dir}/reference-capsule.claims.json" \
  --repository-root . \
  > "${tmp_dir}/claim-surface-verification.json"
"${evalwitness_bin}" artifact scan --class public \
  --path "${tmp_dir}/reference-capsule" \
  --path "${tmp_dir}/reference-capsule.claims.json" \
  --path "${tmp_dir}/reference-capsule.intoto.json" \
  --path "${tmp_dir}/reference-capsule.projection.json" \
  --path "${tmp_dir}/reference-capsule.autopsy.json" \
  > "${tmp_dir}/reference-capsule-public-scan.json"

echo "==> frozen evidence-reliance capsule and canonical projections"
go test ./internal/reliance \
  -run '^TestEvidenceReliancePublicArtifactsMatchCanonicalProjection$' \
  -count=1 -timeout 15m
"${evalwitness_bin}" capsule build-reliance \
  --base-capsule eval/results/evidence-reliance-base-capsule-v1 \
  --map eval/results/evidence-reliance-map-v1.json \
  --destination "${tmp_dir}/reliance-capsule" \
  > "${tmp_dir}/reliance-capsule-build.json"
cmp "${tmp_dir}/reliance-capsule.claims.json" eval/results/evidence-reliance-claims-v1.json
"${evalwitness_bin}" capsule verify-reliance \
  --base-capsule eval/results/evidence-reliance-base-capsule-v1 \
  --source "${tmp_dir}/reliance-capsule" \
  --map eval/results/evidence-reliance-map-v1.json \
  --ledger eval/results/evidence-reliance-claims-v1.json \
  --profile eval/results/evidence-reliance-profile-v1.json \
  --paper eval/results/evidence-reliance-paper-v1.json \
  --explorer eval/results/evidence-reliance-explorer-v1.json \
  > "${tmp_dir}/reliance-capsule-verification.json"
"${evalwitness_bin}" artifact scan --class public \
  --path eval/results/evidence-reliance-base-capsule-v1 \
  --path eval/results/evidence-reliance-base-claims-v1.json \
  --path eval/results/evidence-reliance-map-v1.json \
  --path eval/results/evidence-reliance-claims-v1.json \
  --path eval/results/evidence-reliance-profile-v1.json \
  --path eval/results/evidence-reliance-paper-v1.json \
  --path eval/results/evidence-reliance-explorer-v1.json \
  > "${tmp_dir}/reliance-public-scan.json"

echo "==> public narrative scope and identity policy"
scripts/tests/run-name-residue.sh > "${tmp_dir}/name-residue.txt"
python3 <<'PY'
import json
import pathlib
import re

surfaces = {
    "README.md": (
        "EvalWitness is a provider-portable, reproducible verifier audit lab for coding-agent trajectories.",
        "The bounded identical-response study is closed and reproducible.",
        "## Prove one claim in five minutes",
    ),
    "docs/documentation.md": (
        "EvalWitness is a provider-portable, reproducible verifier audit lab for coding-agent trajectories.",
        "It does not mean that every provider",
        "## Closest Work and Contribution Boundary",
    ),
    "docs/findings.md": (
        "Current provider-free capsule",
        "Legacy development artifacts",
        "## Current finding A: the method preserves its own falsification history",
    ),
    "docs/spec.md": (
        "EvalWitness is a provider-portable, reproducible verifier audit lab for coding-agent trajectories.",
        "Provider portability is a versioned contract, not universal route capability.",
        "No-key claim gate:",
    ),
    "eval/results/README.md": (
        "EvalWitness is a provider-portable, reproducible verifier audit lab for coding-agent trajectories.",
        "## Current provider-free package",
        "They do not record a provider-issued served checkpoint identity.",
    ),
    "docs/releasing.md": (
        "EvalWitness is a provider-portable, reproducible verifier audit lab for coding-agent trajectories.",
        "## Public claim set",
        "This runbook remains pre-publication evidence",
    ),
    ".agents/skills/evalwitness-audit/SKILL.md": (
        "EvalWitness is a provider-portable, reproducible verifier audit lab for coding-agent trajectories.",
        "Portability names versioned contracts, not universal route support.",
        "Local claim gate:",
    ),
}
generated_block = re.compile(
    r"<!-- evalwitness:claim-surface:[^:]+:begin -->.*?"
    r"<!-- evalwitness:claim-surface:[^:]+:end -->",
    re.DOTALL,
)
for path_text, required in surfaces.items():
    path = pathlib.Path(path_text)
    text = path.read_text(encoding="utf-8")
    normalized = " ".join(text.split())
    for fragment in required:
        if fragment not in normalized:
            raise SystemExit(f"{path}: required bounded narrative fragment is missing: {fragment!r}")
    manual = generated_block.sub("", text)
    lowered = manual.casefold()
    forbidden = (
        ("legacy selector-first heading", "## verifier vs. judge"),
        ("legacy metric-wall heading", "## results"),
        ("unsupported exclusivity", "what is missing across the tooling"),
        ("unsupported exclusivity", "nothing else ships this combination"),
        ("universal provider scope", "works with any openai-compatible endpoint"),
        ("universal provider scope", "all providers/models"),
        ("unsupported validation scope", "human validated"),
        ("unsupported replication scope", "independently replicated"),
        ("unsupported certification scope", "community-validated"),
    )
    for reason, phrase in forbidden:
        if phrase in lowered:
            raise SystemExit(f"{path}: {reason}: {phrase!r}")
    for match in re.finditer(
        r"\bthe first (?:judge|verifier|trajectory|trace|audit|evaluation|evaluator|provider)",
        lowered,
    ):
        prefix = lowered[max(0, match.start() - 120):match.start()]
        if not re.search(r"\b(?:not|no|never|without)\b[^.!?\n]{0,120}$", prefix):
            raise SystemExit(f"{path}: unsupported novelty language near {match.group(0)!r}")

scarcity = json.loads(
    pathlib.Path("eval/results/relation-scarcity-negative-evidence.json").read_text(encoding="utf-8")
)
natural_audit = json.loads(
    pathlib.Path("eval/governance/controlled-corruption-v3-natural-audit.json").read_text(encoding="utf-8")
)
owner = json.loads(
    pathlib.Path("eval/results/relation-owner-inspection-attestation.json").read_text(encoding="utf-8")
)
admitted = scarcity["availability"]["admitted"]
target = scarcity["availability"]["target"]
test_cases = scarcity["study_roles"]["test"]
attempts = natural_audit["total_attempts"]
completed = owner["assessments"]["completed"]
dimension_count = len(owner["dimensions"])
core_status = owner["outcomes"]["core_status"]
scarcity_status = owner["outcomes"]["scarcity_status"]

derived_fragments = {
    "docs/findings.md": (
        f"v3: {attempts} attempts",
        f"scarcity availability is {admitted} of {target} with zero test-role cases",
        f"all {completed} required assessments completed",
        f"core status is `{core_status}`; scarcity and overall status are `{scarcity_status}`",
    ),
}
if test_cases != 0:
    raise SystemExit("the current public narrative requires zero test-role scarcity cases")
for path_text, required in derived_fragments.items():
    normalized = " ".join(pathlib.Path(path_text).read_text(encoding="utf-8").split())
    for fragment in required:
        if fragment not in normalized:
            raise SystemExit(
                f"{path_text}: canonical current-evidence fragment is stale or missing: {fragment!r}"
            )
PY

echo "==> capsule corruption rejection"
cp -R "${tmp_dir}/reference-capsule" "${tmp_dir}/corrupted-capsule"
corruption_target="$(rg --files "${tmp_dir}/corrupted-capsule/components" | LC_ALL=C sort | sed -n '1p')"
truncate -s 1 "${corruption_target}"
if "${evalwitness_bin}" capsule verify --source "${tmp_dir}/corrupted-capsule" \
  > "${tmp_dir}/corrupted-capsule.stdout" 2> "${tmp_dir}/corrupted-capsule.stderr"; then
  echo "capsule verifier accepted a byte-corrupted component" >&2
  exit 1
fi
cp -R "${tmp_dir}/reference-capsule" "${tmp_dir}/incomplete-capsule"
missing_target="$(rg --files "${tmp_dir}/incomplete-capsule/components" | LC_ALL=C sort | sed -n '1p')"
rm "${missing_target}"
if "${evalwitness_bin}" capsule verify --source "${tmp_dir}/incomplete-capsule" \
  > "${tmp_dir}/incomplete-capsule.stdout" 2> "${tmp_dir}/incomplete-capsule.stderr"; then
  echo "capsule verifier accepted a missing component" >&2
  exit 1
fi
cp -R "${tmp_dir}/reference-capsule" "${tmp_dir}/schema-mismatch-capsule"
python3 - "${tmp_dir}/schema-mismatch-capsule/registry.json" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
document = json.loads(path.read_text())
document["schema_version"] = "evalwitness.capsule-registry.invalid"
path.write_text(json.dumps(document, ensure_ascii=False, separators=(",", ":"), sort_keys=True))
PY
if "${evalwitness_bin}" capsule verify --source "${tmp_dir}/schema-mismatch-capsule" \
  > "${tmp_dir}/schema-mismatch-capsule.stdout" 2> "${tmp_dir}/schema-mismatch-capsule.stderr"; then
  echo "capsule verifier accepted a registry schema mismatch" >&2
  exit 1
fi
if ! rg -q "capsule registry identity is invalid" "${tmp_dir}/schema-mismatch-capsule.stderr"; then
  echo "capsule schema-mismatch test failed at an unintended guard" >&2
  exit 1
fi

echo "==> isolated offline capsule verification"
isolated_root="${tmp_dir}/isolated"
mkdir -p "${isolated_root}/home" "${isolated_root}/tmp"
cp -R "${tmp_dir}/reference-capsule" "${isolated_root}/capsule"
cp "${tmp_dir}/reference-capsule.claims.json" "${isolated_root}/claims.json"
cp "${tmp_dir}/reference-capsule.intoto.json" "${isolated_root}/statement.json"
cp "${tmp_dir}/reference-capsule.projection.json" "${isolated_root}/projection.json"
cp "${tmp_dir}/reference-capsule.autopsy.json" "${isolated_root}/autopsy.json"
network_guard=()
if [ "${EVALWITNESS_NETWORK_GUARD_ACTIVE:-0}" = "1" ]; then
  # Bash 3.2 with nounset rejects expansion of a declared empty array.
  network_guard=(/usr/bin/env)
elif command -v sandbox-exec > /dev/null 2>&1; then
  network_guard=(sandbox-exec -p '(version 1) (allow default) (deny network*)')
elif command -v unshare > /dev/null 2>&1 && unshare --user --map-root-user --net true > /dev/null 2>&1; then
  network_guard=(unshare --user --map-root-user --net)
else
  echo "hard network isolation is unavailable on this host" >&2
  exit 1
fi
(
  cd "${isolated_root}"
  "${network_guard[@]}" env -i \
    PATH="${PATH}" HOME="${isolated_root}/home" TMPDIR="${isolated_root}/tmp" \
    HTTP_PROXY="http://127.0.0.1:1" HTTPS_PROXY="http://127.0.0.1:1" ALL_PROXY="http://127.0.0.1:1" \
    NO_PROXY="" \
    "${evalwitness_bin}" capsule verify \
      --source capsule \
      --ledger claims.json \
      --statement statement.json \
      --projection projection.json \
      --autopsy autopsy.json \
      > isolated-verification.json
)

echo "==> default route claim"
EVALWITNESS_ENV_FILE="${empty_env}" \
BAI_API_KEY=claimcheck \
EVALWITNESS_CACHE_DIR="${tmp_dir}/cache" \
  "${evalwitness_bin}" doctor --output json > "${tmp_dir}/doctor.json"

echo "==> preset metadata claim"
"${evalwitness_bin}" presets --output json > "${tmp_dir}/presets.json"

echo "==> study governance and historical-data role claims"
"${evalwitness_bin}" study schema --type manifest > "${tmp_dir}/study-schema.json"
"${evalwitness_bin}" study inventory > "${tmp_dir}/study-inventory.json"

echo "==> verification-lineage preregistration claim"
"${evalwitness_bin}" trace lineage plan > "${tmp_dir}/verification-lineage-plan.json"
"${evalwitness_bin}" trace lineage schema-inventory > "${tmp_dir}/verification-lineage-schema-inventory.json"
"${evalwitness_bin}" trace lineage source-inventory > "${tmp_dir}/verification-lineage-source-inventory.json"
"${evalwitness_bin}" trace lineage source-specifications > "${tmp_dir}/trace-source-specifications.json"
"${evalwitness_bin}" trace lineage fixture-witnesses > "${tmp_dir}/synthetic-execution-witness-fixtures.json"
"${evalwitness_bin}" trace lineage golden-vectors > "${tmp_dir}/verification-lineage-golden-vectors.json"
"${evalwitness_bin}" trace lineage adapter-conformance > "${tmp_dir}/verification-lineage-adapter-conformance.json"
"${evalwitness_bin}" trace lineage parser-lock --repository-root . > "${tmp_dir}/verification-lineage-parser-lock.json"
"${evalwitness_bin}" trace lineage parser-lock-verify --repository-root . \
  --document @eval/governance/verification-lineage-parser-lock-v1.json \
  > "${tmp_dir}/verification-lineage-parser-lock-validation.json"
"${evalwitness_bin}" trace lineage source-readiness --repository-root . > "${tmp_dir}/verification-lineage-source-readiness-audit.json"
"${evalwitness_bin}" trace lineage source-readiness-verify --repository-root . \
  --document @eval/governance/verification-lineage-source-readiness-audit-v1.json \
  > "${tmp_dir}/verification-lineage-source-readiness-audit-validation.json"
"${evalwitness_bin}" trace lineage holdout-readiness --repository-root . > "${tmp_dir}/verification-lineage-holdout-readiness-audit.json"
"${evalwitness_bin}" trace lineage holdout-readiness-verify --repository-root . \
  --document @eval/governance/verification-lineage-holdout-readiness-audit-v1.json \
  > "${tmp_dir}/verification-lineage-holdout-readiness-audit-validation.json"
"${evalwitness_bin}" trace lineage corpus-feasibility --repository-root . > "${tmp_dir}/verification-lineage-corpus-feasibility.json"
"${evalwitness_bin}" trace lineage corpus-feasibility-verify --repository-root . \
  --document @eval/governance/verification-lineage-corpus-feasibility-v1.json \
  > "${tmp_dir}/verification-lineage-corpus-feasibility-validation.json"
"${evalwitness_bin}" trace lineage capability-matrix > "${tmp_dir}/verification-lineage-capability-matrix.json"
"${evalwitness_bin}" trace lineage capability-matrix-verify \
  --document @eval/governance/verification-lineage-capability-matrix-v1.json \
  > "${tmp_dir}/verification-lineage-capability-matrix-validation.json"
"${evalwitness_bin}" trace lineage offline-proof --repository-root . > "${tmp_dir}/verification-lineage-offline-proof.json"
"${evalwitness_bin}" trace lineage offline-proof-verify --repository-root . \
  --document @eval/governance/verification-lineage-offline-proof-v1.json \
  > "${tmp_dir}/verification-lineage-offline-proof-validation.json"
"${evalwitness_bin}" trace lineage loss-certificate --repository-root . > "${tmp_dir}/verification-lineage-loss-certificate.json"
"${evalwitness_bin}" trace lineage loss-certificate-verify --repository-root . \
  --document @eval/results/verification-lineage-same-path-loss-certificate-v1.json \
  > "${tmp_dir}/verification-lineage-loss-certificate-validation.json"
"${evalwitness_bin}" trace lineage lineage-graph --repository-root . --format json > "${tmp_dir}/verification-lineage-graph.json"
"${evalwitness_bin}" trace lineage lineage-graph --repository-root . --format svg > "${tmp_dir}/verification-lineage-graph.svg"
"${evalwitness_bin}" trace lineage lineage-graph-verify --repository-root . \
  --document @eval/results/verification-lineage-offline-graph-v1.json \
  > "${tmp_dir}/verification-lineage-graph-validation.json"
"${evalwitness_bin}" trace lineage offline-bom --repository-root . > "${tmp_dir}/verification-lineage-offline-bom.json"
"${evalwitness_bin}" trace lineage offline-bom-verify --repository-root . \
  --document @eval/results/verification-lineage-offline-bom-example-v1.json \
  > "${tmp_dir}/verification-lineage-offline-bom-validation.json"
"${evalwitness_bin}" trace lineage offline-audit --repository-root . > "${tmp_dir}/verification-lineage-offline-audit.json"
"${evalwitness_bin}" trace lineage offline-audit-verify --repository-root . \
  --document @eval/results/verification-lineage-offline-audit-v1.json \
  > "${tmp_dir}/verification-lineage-offline-audit-validation.json"
"${evalwitness_bin}" trace lineage development-dataset-card --repository-root . > "${tmp_dir}/verification-lineage-development-dataset-card.json"
"${evalwitness_bin}" trace lineage development-dataset-card-verify --repository-root . \
  --document @eval/results/verification-lineage-development-dataset-card-v1.json \
  > "${tmp_dir}/verification-lineage-development-dataset-card-validation.json"
"${evalwitness_bin}" trace lineage limitations --repository-root . > "${tmp_dir}/verification-lineage-limitations.json"
"${evalwitness_bin}" trace lineage limitations-verify --repository-root . \
  --document @eval/results/verification-lineage-limitations-v1.json \
  > "${tmp_dir}/verification-lineage-limitations-validation.json"
"${evalwitness_bin}" trace lineage development-release --repository-root . > "${tmp_dir}/verification-lineage-development-release.json"
"${evalwitness_bin}" trace lineage development-release-verify --repository-root . \
  --document @eval/results/verification-lineage-development-release-v1.json \
  > "${tmp_dir}/verification-lineage-development-release-validation.json"
"${evalwitness_bin}" trace lineage schema --type plan > "${tmp_dir}/verification-lineage-plan.schema.json"
"${evalwitness_bin}" trace lineage validate --type plan \
  --document @eval/governance/verification-lineage-plan-v1.json \
  > "${tmp_dir}/verification-lineage-plan-validation.json"
cmp "${tmp_dir}/verification-lineage-plan.json" eval/governance/verification-lineage-plan-v1.json
cmp "${tmp_dir}/verification-lineage-schema-inventory.json" eval/governance/verification-lineage-schema-inventory-v1.json
cmp "${tmp_dir}/verification-lineage-source-inventory.json" eval/governance/verification-lineage-source-inventory-v1.json
cmp "${tmp_dir}/trace-source-specifications.json" eval/governance/trace-source-specifications-v1.json
cmp "${tmp_dir}/synthetic-execution-witness-fixtures.json" eval/governance/synthetic-execution-witness-fixtures-v1.json
cmp "${tmp_dir}/verification-lineage-golden-vectors.json" eval/governance/verification-lineage-golden-vectors-v1.json
cmp "${tmp_dir}/verification-lineage-adapter-conformance.json" eval/governance/verification-lineage-adapter-conformance-v1.json
cmp "${tmp_dir}/verification-lineage-parser-lock.json" eval/governance/verification-lineage-parser-lock-v1.json
cmp "${tmp_dir}/verification-lineage-source-readiness-audit.json" eval/governance/verification-lineage-source-readiness-audit-v1.json
cmp "${tmp_dir}/verification-lineage-holdout-readiness-audit.json" eval/governance/verification-lineage-holdout-readiness-audit-v1.json
cmp "${tmp_dir}/verification-lineage-corpus-feasibility.json" eval/governance/verification-lineage-corpus-feasibility-v1.json
cmp "${tmp_dir}/verification-lineage-capability-matrix.json" eval/governance/verification-lineage-capability-matrix-v1.json
cmp "${tmp_dir}/verification-lineage-offline-proof.json" eval/governance/verification-lineage-offline-proof-v1.json
cmp "${tmp_dir}/verification-lineage-loss-certificate.json" eval/results/verification-lineage-same-path-loss-certificate-v1.json
cmp "${tmp_dir}/verification-lineage-graph.json" eval/results/verification-lineage-offline-graph-v1.json
cmp "${tmp_dir}/verification-lineage-graph.svg" eval/results/verification-lineage-offline-graph-v1.svg
cmp "${tmp_dir}/verification-lineage-offline-bom.json" eval/results/verification-lineage-offline-bom-example-v1.json
cmp "${tmp_dir}/verification-lineage-offline-audit.json" eval/results/verification-lineage-offline-audit-v1.json
cmp "${tmp_dir}/verification-lineage-development-dataset-card.json" eval/results/verification-lineage-development-dataset-card-v1.json
cmp "${tmp_dir}/verification-lineage-limitations.json" eval/results/verification-lineage-limitations-v1.json
cmp "${tmp_dir}/verification-lineage-development-release.json" eval/results/verification-lineage-development-release-v1.json
lineage_plan_file_digest="$(shasum -a 256 eval/governance/verification-lineage-plan-v1.json | awk '{print $1}')"
if [ "${lineage_plan_file_digest}" != "e0765e67a28f96f68cc427df6a373504188c59d16e5f519776fd8cc6213f81cc" ]; then
  echo "verification-lineage preregistration bytes drifted" >&2
  exit 1
fi
lineage_inventory_file_digest="$(shasum -a 256 eval/governance/verification-lineage-schema-inventory-v1.json | awk '{print $1}')"
if [ "${lineage_inventory_file_digest}" != "08613396c861a83c8fa8b30d8533ae275b972f3d0e84898de2120bd1a4db5359" ]; then
  echo "verification-lineage schema inventory bytes drifted" >&2
  exit 1
fi
source_specifications_file_digest="$(shasum -a 256 eval/governance/trace-source-specifications-v1.json | awk '{print $1}')"
if [ "${source_specifications_file_digest}" != "4bf95b9ae576dbb0ee2759ad272bfa23c1cebd30c94de5972c2ba944a9e6c59a" ]; then
  echo "trace source specification registry bytes drifted" >&2
  exit 1
fi
source_inventory_file_digest="$(shasum -a 256 eval/governance/verification-lineage-source-inventory-v1.json | awk '{print $1}')"
if [ "${source_inventory_file_digest}" != "c0eac7c5a2bcf9226c32a61eb8a543be05179426598f017d9885e8b8e717098b" ]; then
  echo "verification-lineage source inventory bytes drifted" >&2
  exit 1
fi
synthetic_witness_fixtures_file_digest="$(shasum -a 256 eval/governance/synthetic-execution-witness-fixtures-v1.json | awk '{print $1}')"
if [ "${synthetic_witness_fixtures_file_digest}" != "f857a668ab46ca440a6e1cba7dcf2a65fb63de062767ee531a6e6f810550a422" ]; then
  echo "synthetic execution-witness fixture bytes drifted" >&2
  exit 1
fi
golden_vectors_file_digest="$(shasum -a 256 eval/governance/verification-lineage-golden-vectors-v1.json | awk '{print $1}')"
if [ "${golden_vectors_file_digest}" != "85f450d4adb927fba91c2f822c535c260643fce3026ef2d5ffc20d4ed3cb3602" ]; then
  echo "verification-lineage golden-vector bytes drifted" >&2
  exit 1
fi
adapter_conformance_file_digest="$(shasum -a 256 eval/governance/verification-lineage-adapter-conformance-v1.json | awk '{print $1}')"
if [ "${adapter_conformance_file_digest}" != "4cd6e9968466c7be22e8442ac5c360758e87b892b6269d2959b65f2304d481b3" ]; then
  echo "verification-lineage adapter-conformance bytes drifted" >&2
  exit 1
fi
parser_lock_file_digest="$(shasum -a 256 eval/governance/verification-lineage-parser-lock-v1.json | awk '{print $1}')"
if [ "${parser_lock_file_digest}" != "17d8e84f23945960cb4599580ca4591202778a09f45c296852714ed392de6a19" ]; then
  echo "verification-lineage parser lock bytes drifted" >&2
  exit 1
fi
source_readiness_file_digest="$(shasum -a 256 eval/governance/verification-lineage-source-readiness-audit-v1.json | awk '{print $1}')"
if [ "${source_readiness_file_digest}" != "763ebc6488e250af58b5e2be79f82d09900a4bb832cca810cdd21afeca4e075d" ]; then
  echo "verification-lineage source-readiness audit bytes drifted" >&2
  exit 1
fi
holdout_readiness_file_digest="$(shasum -a 256 eval/governance/verification-lineage-holdout-readiness-audit-v1.json | awk '{print $1}')"
if [ "${holdout_readiness_file_digest}" != "563a8945a5f53d51d764c74bccc0d8c5a12b19c4bb50e808282776fcc97f4b4f" ]; then
  echo "verification-lineage holdout-readiness audit bytes drifted" >&2
  exit 1
fi
corpus_feasibility_file_digest="$(shasum -a 256 eval/governance/verification-lineage-corpus-feasibility-v1.json | awk '{print $1}')"
if [ "${corpus_feasibility_file_digest}" != "d033491fbf9aace7a2b9eb08c4314c1323c7473a315bab450fa5658cca834d3d" ]; then
  echo "verification-lineage corpus-feasibility bytes drifted" >&2
  exit 1
fi
capability_matrix_file_digest="$(shasum -a 256 eval/governance/verification-lineage-capability-matrix-v1.json | awk '{print $1}')"
if [ "${capability_matrix_file_digest}" != "637ce1b38d226c5fbd33a48049c44b7d6be7e67475a57f15391189fd53d2a96a" ]; then
  echo "verification-lineage capability-matrix bytes drifted" >&2
  exit 1
fi
offline_proof_file_digest="$(shasum -a 256 eval/governance/verification-lineage-offline-proof-v1.json | awk '{print $1}')"
if [ "${offline_proof_file_digest}" != "4ec7ff69f9213dfe3e7809106ab5623e5eee14208a010d1b97c5d7ec7f891f11" ]; then
  echo "verification-lineage offline-proof bytes drifted" >&2
  exit 1
fi
loss_certificate_file_digest="$(shasum -a 256 eval/results/verification-lineage-same-path-loss-certificate-v1.json | awk '{print $1}')"
if [ "${loss_certificate_file_digest}" != "b0e31c68e9c92493231fe401686a051437f3da934ed5f1c8116f9ee421b71045" ]; then
  echo "verification-lineage loss-certificate bytes drifted" >&2
  exit 1
fi
lineage_graph_file_digest="$(shasum -a 256 eval/results/verification-lineage-offline-graph-v1.json | awk '{print $1}')"
if [ "${lineage_graph_file_digest}" != "a5e9bdb3caa1e67bdf9115e530b85361b5e53fe1ae5ea4511c643918e371c319" ]; then
  echo "verification-lineage graph bytes drifted" >&2
  exit 1
fi
lineage_graph_svg_file_digest="$(shasum -a 256 eval/results/verification-lineage-offline-graph-v1.svg | awk '{print $1}')"
if [ "${lineage_graph_svg_file_digest}" != "9744b8ffb6af12f3332becf67aaa2de9ae8c2ef586aaa01861d59d79b87ec4cd" ]; then
  echo "verification-lineage SVG bytes drifted" >&2
  exit 1
fi
offline_bom_file_digest="$(shasum -a 256 eval/results/verification-lineage-offline-bom-example-v1.json | awk '{print $1}')"
if [ "${offline_bom_file_digest}" != "2553d32f3f5eb72be492f2431f17cb663e8194d00f45a6f978fb92757b55ad67" ]; then
  echo "verification-lineage offline BOM bytes drifted" >&2
  exit 1
fi
offline_audit_file_digest="$(shasum -a 256 eval/results/verification-lineage-offline-audit-v1.json | awk '{print $1}')"
if [ "${offline_audit_file_digest}" != "aa68587b4f90cb05de3d4f5d67a15d720f167ac1a9858c3df625ebcb62b3af46" ]; then
  echo "verification-lineage offline audit bytes drifted" >&2
  exit 1
fi
development_dataset_card_file_digest="$(shasum -a 256 eval/results/verification-lineage-development-dataset-card-v1.json | awk '{print $1}')"
if [ "${development_dataset_card_file_digest}" != "079cb4b0e504f2caf48e92966ba29c1f7bd90750a1868dfb142d157d64fdab22" ]; then
  echo "verification-lineage development dataset-card bytes drifted" >&2
  exit 1
fi
limitations_file_digest="$(shasum -a 256 eval/results/verification-lineage-limitations-v1.json | awk '{print $1}')"
if [ "${limitations_file_digest}" != "51fef433281525ad298fd7f25e807c3ce0b8e2d2a8c339cfc1de8a970464c72d" ]; then
  echo "verification-lineage limitations bytes drifted" >&2
  exit 1
fi
development_release_file_digest="$(shasum -a 256 eval/results/verification-lineage-development-release-v1.json | awk '{print $1}')"
if [ "${development_release_file_digest}" != "50aeeb372dfd93ab55a41b0fe246d9b26772f3bbc39e96588883a0dbbf27951a" ]; then
  echo "verification-lineage development release bytes drifted" >&2
  exit 1
fi
for lineage_type in assessment audit bom candidate capability-vector dataset-card execution-witness plan release source; do
  "${evalwitness_bin}" trace lineage schema --type "${lineage_type}" \
    > "${tmp_dir}/verification-lineage-${lineage_type}.schema.json"
done
"${evalwitness_bin}" artifact scan --class public \
  --path eval/governance/verification-lineage-plan-v1.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/governance/verification-lineage-schema-inventory-v1.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/governance/trace-source-specifications-v1.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/governance/verification-lineage-source-inventory-v1.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/governance/synthetic-execution-witness-fixtures-v1.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/governance/verification-lineage-golden-vectors-v1.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/governance/verification-lineage-adapter-conformance-v1.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/governance/verification-lineage-parser-lock-v1.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/governance/verification-lineage-source-readiness-audit-v1.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/governance/verification-lineage-holdout-readiness-audit-v1.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/governance/verification-lineage-corpus-feasibility-v1.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/governance/verification-lineage-capability-matrix-v1.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/governance/verification-lineage-offline-proof-v1.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/results/verification-lineage-same-path-loss-certificate-v1.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/results/verification-lineage-offline-graph-v1.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/results/verification-lineage-offline-graph-v1.svg > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/results/verification-lineage-offline-bom-example-v1.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/results/verification-lineage-offline-audit-v1.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/results/verification-lineage-development-dataset-card-v1.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/results/verification-lineage-limitations-v1.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/results/verification-lineage-development-release-v1.json > /dev/null
"${evalwitness_bin}" trace lineage intake \
  --source internal/preprocess/testdata/golden/codex-rollout.jsonl \
  > "${tmp_dir}/verification-lineage-intake.json"

have_eval_data=1
if [ ! -d eval/trajectories/terminal_trajs ] || [ ! -d eval/trajectories/swebench_verified_trajs ]; then
  have_eval_data=0
  echo "==> SKIP dataset claims (eval/trajectories not fetched; run eval/fetch-eval-data.sh)"
fi

if [ "${have_eval_data}" = "1" ]; then
  echo "==> Terminal-Bench dry-run claim"
  "${evalwitness_bin}" eval-terminal --dry-run --output json > "${tmp_dir}/terminal.json"

  echo "==> SWE-bench dry-run claim"
  "${evalwitness_bin}" eval-swebench --dry-run --output json > "${tmp_dir}/swebench.json"
fi

echo "==> deterministic clustered-design claim"
"${evalwitness_bin}" design simulate \
  --spec @cmd/evalwitness/testdata/design-simulation.json \
  --output json > "${tmp_dir}/design-simulation.json"

echo "==> paired effect-resolution claim"
scripts/audits/run-paired-analysis.sh \
  --question equivalence --margin 0.05 \
  eval/results/terminal-bench-verifier.json \
  eval/results/terminal-bench-judge.json > "${tmp_dir}/paired-resolution.txt"

echo "==> replay verifier claim"
EVALWITNESS_BIN="${evalwitness_bin}" scripts/tests/run-replay-smoke.sh > "${tmp_dir}/replay.txt"

echo "==> public negative-evidence contract and brief"
"${evalwitness_bin}" relation render-scarcity-public-brief \
  --format json \
  --plan @eval/governance/relation-audit-plan-v3.json \
  --primary-sample @eval/governance/relation-primary-sample-v3.json \
  --scarcity-sentinel @eval/governance/relation-scarcity-sentinel-v3.json \
  --corpus-plan @eval/governance/controlled-corruption-v3-plan.json \
  --corpus-audit @eval/governance/controlled-corruption-v3-natural-audit.json \
  --release @eval/governance/controlled-corruption-v3-release.json \
  > "${tmp_dir}/relation-scarcity-negative-evidence.json"
"${evalwitness_bin}" relation render-scarcity-public-brief \
  --format markdown \
  --plan @eval/governance/relation-audit-plan-v3.json \
  --primary-sample @eval/governance/relation-primary-sample-v3.json \
  --scarcity-sentinel @eval/governance/relation-scarcity-sentinel-v3.json \
  --corpus-plan @eval/governance/controlled-corruption-v3-plan.json \
  --corpus-audit @eval/governance/controlled-corruption-v3-natural-audit.json \
  --release @eval/governance/controlled-corruption-v3-release.json \
  > "${tmp_dir}/relation-scarcity-negative-evidence.md"
"${evalwitness_bin}" relation schema --type scarcity-public-evidence \
  > "${tmp_dir}/relation-scarcity-public-evidence.schema.json"
"${evalwitness_bin}" relation validate --type scarcity-public-evidence \
  --document @eval/results/relation-scarcity-negative-evidence.json \
  > "${tmp_dir}/relation-scarcity-public-evidence-validation.json"
cmp "${tmp_dir}/relation-scarcity-negative-evidence.json" \
  eval/results/relation-scarcity-negative-evidence.json
cmp "${tmp_dir}/relation-scarcity-negative-evidence.md" \
  eval/results/relation-scarcity-negative-evidence.md
evidence_digest="$(shasum -a 256 eval/results/relation-scarcity-negative-evidence.json | awk '{print $1}')"
if [ "${evidence_digest}" != "af3037ae113d1dcd5aaada2ae0e77d4d9c7d666006bdf28d38307a835bb54821" ]; then
  echo "public negative-evidence contract digest drifted" >&2
  exit 1
fi
brief_digest="$(shasum -a 256 eval/results/relation-scarcity-negative-evidence.md | awk '{print $1}')"
if [ "${brief_digest}" != "e3d008ca0ec56755e5cc1784b1b29094b90ad42f69245323a962fa7c65a85a89" ]; then
  echo "public negative-evidence brief digest drifted" >&2
  exit 1
fi
"${evalwitness_bin}" artifact scan --class public \
  --path eval/results/relation-scarcity-negative-evidence.json > /dev/null
"${evalwitness_bin}" artifact scan --class public \
  --path eval/results/relation-scarcity-negative-evidence.md > /dev/null

echo "==> public owner-inspection attestation"
"${evalwitness_bin}" relation schema --type owner-inspection-public-attestation \
  > "${tmp_dir}/relation-owner-inspection-public-attestation.schema.json"
"${evalwitness_bin}" relation validate --type owner-inspection-public-attestation \
  --document @eval/results/relation-owner-inspection-attestation.json \
  > "${tmp_dir}/relation-owner-inspection-public-attestation-validation.json"
owner_attestation_digest="$(shasum -a 256 eval/results/relation-owner-inspection-attestation.json | awk '{print $1}')"
if [ "${owner_attestation_digest}" != "7304efbce27d68746f75180b4296c7b76fddfef0f9a4c53a9522db36d5d13fe8" ]; then
  echo "public owner-inspection attestation digest drifted" >&2
  exit 1
fi
"${evalwitness_bin}" artifact scan --class public \
  --path eval/results/relation-owner-inspection-attestation.json > /dev/null

echo "==> calibration uncertainty and error-strata claims"
scripts/audits/run-calibration-analysis.sh \
  eval/results/terminal-bench-verifier.json \
  eval/results/terminal-bench-judge.json \
  eval/results/terminal-bench-absolute.json \
  eval/results/swebench-verifier.json \
  eval/results/swebench-judge.json > "${tmp_dir}/calibration.txt"
scripts/audits/run-calibration-analysis.sh --format json \
  eval/results/terminal-bench-verifier.json \
  eval/results/terminal-bench-judge.json \
  eval/results/terminal-bench-absolute.json \
  eval/results/swebench-verifier.json \
  eval/results/swebench-judge.json > "${tmp_dir}/calibration.json"

python3 - "${tmp_dir}" <<'PY'
import json
import hashlib
import math
import pathlib
import sys

root = pathlib.Path(sys.argv[1])

def load(name):
    return json.loads((root / name).read_text())

def close(got, want, eps=1e-9):
    if not math.isclose(got, want, rel_tol=0, abs_tol=eps):
        raise SystemExit(f"{got!r} != {want!r}")

capsule_build = load("capsule-build.json")
if not capsule_build["offline"] or capsule_build["provider_calls"] != 0 or not capsule_build["archive"]["deterministic"]:
    raise SystemExit("reference capsule build crossed its offline deterministic boundary")
if capsule_build["archive"]["files"] != 74 or not capsule_build["autopsy_digest"]:
    raise SystemExit("reference capsule build omitted canonical files or Claim Autopsy")
capsule_verification = load("capsule-verification.json")
if not capsule_verification["capsule"]["valid"] or not capsule_verification["capsule"]["offline"]:
    raise SystemExit("reference capsule did not verify offline")
if (capsule_verification["capsule"]["components"], capsule_verification["capsule"]["scientific_components"], capsule_verification["capsule"]["presentation_components"]) != (71, 69, 2):
    raise SystemExit("reference capsule component census drifted")
if not all(capsule_verification[name] for name in ("statement_verified", "projection_verified", "autopsy_verified")):
    raise SystemExit("reference capsule sidecar verification is incomplete")

claim_verification = load("claim-verification.json")
if not claim_verification["valid"] or not claim_verification["offline"] or len(claim_verification["claims"]) != 34:
    raise SystemExit("canonical claim ledger did not verify all 34 claims offline")
if claim_verification["status_counts"] != {"exploratory": 2, "superseded": 1, "supported": 13, "unsupported": 18}:
    raise SystemExit("canonical claim lifecycle counts drifted")

autopsy = load("claim-autopsy.json")
method = autopsy["method_integrity"]
if [(item["generation"], item["state"]) for item in method["generations"]] != [
    ("v1", "falsified"), ("v2", "superseded"), ("v3", "admitted_development"),
]:
    raise SystemExit("Claim Autopsy method generations drifted")
v3_counts = {item["name"]: item["value"] for item in method["generations"][2]["frozen_denominators"]}
if v3_counts != {
    "inferential_core_cases": 280, "natural_applied_attempts": 689,
    "natural_rejected_attempts": 250, "natural_selected_cases": 283,
    "natural_source_tasks": 100, "natural_sources": 200,
    "natural_total_attempts": 939, "release_cases": 283,
    "scarcity_admitted": 3, "scarcity_attempted": 198,
    "scarcity_rejected": 195, "scarcity_shortfall": 37,
    "scarcity_target": 40, "scarcity_test_role": 0,
}:
    raise SystemExit("Claim Autopsy v3 frozen denominators drifted")
if [item["layer"] for item in autopsy["claim_transport"]["layers"]] != [
    "runtime_witness", "native_export", "canonical_graph", "retained_bundle", "verifier_request",
]:
    raise SystemExit("Claim Autopsy five-layer transport drifted")
if autopsy["claim_transport"]["provider_calls"] != 0 or len(autopsy["current_claim_ids"]) != 15 or len(autopsy["historical_claim_ids"]) != 19:
    raise SystemExit("Claim Autopsy crossed its provider or lifecycle boundary")

challenge_pack = load("claim-challenge-pack.json")
if len(challenge_pack["receipts"]) != 189 or sum(challenge_pack["class_counts"].values()) != 189:
    raise SystemExit("claim challenge pack no longer proves every applicable guard")
surface_verification = load("claim-surface-verification.json")
if surface_verification != {
    "claims": 34, "offline": True, "provider_calls": 0,
    "schema_version": "evalwitness.claim-surface-verification.v1", "surfaces": 6, "valid": True,
}:
    raise SystemExit("tracked claim surfaces differ from the canonical ledger projection")

doctor = load("doctor.json")
if doctor["provider"] != "bai":
    raise SystemExit("default provider is not the live-probed free route")
if doctor["wire_format"] != "openai":
    raise SystemExit("wire format is not openai")
if doctor["base_url"] != "https://api.b.ai/v1":
    raise SystemExit("default base URL is not the live-probed free route")
if doctor["model"] != "deepseek-v4-flash":
    raise SystemExit("default model is not deepseek-v4-flash")
if doctor["thinking_mode"] != "disabled":
    raise SystemExit("default thinking mode is not disabled; logprobs need the final answer")
if doctor["capability_status"] != "configured":
    raise SystemExit("default no-live route must remain configured, not self-qualified")
if doctor["full_verifier"]:
    raise SystemExit("default no-live route cannot claim full-verifier status without a current attestation")
if doctor.get("attestation_id"):
    raise SystemExit("isolated claimcheck unexpectedly loaded an attestation")

presets = {p["name"]: p for p in load("presets.json")}
if not presets["bai-deepseek-v4-flash"]["default"]:
    raise SystemExit("the live-probed free route is not the default preset")
default_preset = presets["bai-deepseek-v4-flash"]
if default_preset["capability_state"] != "configured":
    raise SystemExit("preset metadata self-qualified the default route")
if "full_verifier" in default_preset:
    raise SystemExit("preset metadata restored a timeless full_verifier badge")
# The registry is deliberately one model family. Every route stays configured
# until a fresh exact attestation qualifies it; pinned OpenRouter routes must
# also retain their upstream identity.
expected_presets = {
    "bai-deepseek-v4-flash",
    "deepseek-v4-flash",
    "deepseek-v4-pro",
    "fireworks-deepseek-v4-flash-0731",
    "opencode-go-deepseek-v4-flash-0731",
    "openrouter-ambient-deepseek-v4-flash-0731",
    "openrouter-morph-deepseek-v4-flash-0731",
}
if set(presets) != expected_presets:
    raise SystemExit(f"DeepSeek preset registry drifted: {sorted(presets)}")
for name, preset in presets.items():
    if preset["capability_state"] != "configured" or "full_verifier" in preset:
        raise SystemExit(f"preset metadata self-qualified route {name}")
for name, upstream in {
    "openrouter-ambient-deepseek-v4-flash-0731": "ambient",
    "openrouter-morph-deepseek-v4-flash-0731": "morph",
}.items():
    if presets[name].get("upstream_provider") != upstream:
        raise SystemExit(f"preset {name} lost its pinned upstream")

study_schema = load("study-schema.json")
if study_schema["$schema"] != "https://json-schema.org/draft/2020-12/schema":
    raise SystemExit("study manifest schema is not Draft 2020-12")
if study_schema.get("additionalProperties") is not False:
    raise SystemExit("study manifest schema permits undeclared fields")
required_study_sections = {
    "identity", "hypotheses", "data", "arms", "outcomes", "inference", "failures",
    "controls", "providers", "budget", "execution", "publication", "reliability_contracts", "adjudication",
}
if not required_study_sections.issubset(study_schema.get("properties", {})):
    raise SystemExit("study manifest schema lost a governance section")
inventory = load("study-inventory.json")
if inventory != {"valid": True, "datasets": 2}:
    raise SystemExit("historical development-data inventory is not verified")

lineage_intake = load("verification-lineage-intake.json")
if lineage_intake["capability_plan"] != {
    "format": "codex_rollout_jsonl", "format_version": "unversioned-export",
    "status": "development_vector_available",
    "capability_vector_digest": "513ce19684ca845c4300e40ff4a88ef7b0cb995fbd688f698844d085e520de37",
    "specification_digest": "a1c8dd84c784df98fb0aa768fa34d18469f41d3ccd3514da0e29e9e283b5c3aa",
    "observed_fields": 9, "not_observed_fields": 1, "format_wide_inference": False,
}:
    raise SystemExit("verification-lineage intake capability plan drifted")
if lineage_intake["conformance_plan"] != {
    "status": "development_conformance_available", "development_vectors": 21,
    "checks": 174, "passed_checks": 174, "failed_checks": 0, "input_conformance": False,
}:
    raise SystemExit("verification-lineage intake conformance plan drifted")
loss_plan = lineage_intake["loss_plan"]
if loss_plan["diagnosis_code"] != "independent_execution_witness_required" or loss_plan["can_classify_terminal_state"] or loss_plan["can_build_bom"]:
    raise SystemExit("verification-lineage intake crossed its missing-witness boundary")
if lineage_intake["claim_boundary"]["analysis_performed"] or lineage_intake["claim_boundary"]["provider_calls"] != 0 or lineage_intake["claim_boundary"]["agent_launches"] != 0:
    raise SystemExit("verification-lineage intake promoted a provider-free preflight")

loss_certificate = load("verification-lineage-loss-certificate.json")
loss_certificate_validation = load("verification-lineage-loss-certificate-validation.json")
if loss_certificate_validation != {
    "certificate_id": loss_certificate["certificate_id"], "digest": loss_certificate["digest"],
    "valid": True, "version": loss_certificate["version"],
}:
    raise SystemExit("verification-lineage loss certificate validation drifted")
if loss_certificate["layers"] != [
    {"layer": "runtime_witness", "status": "survived"},
    {"layer": "native_export", "status": "survived"},
    {"layer": "canonical_graph", "status": "survived"},
    {"layer": "retained_bundle", "status": "first_loss"},
    {"layer": "verifier_request", "status": "not_reached"},
] or not loss_certificate["public_safe"] or loss_certificate["restricted_content"]:
    raise SystemExit("verification-lineage loss certificate semantics drifted")
lineage_graph = load("verification-lineage-graph.json")
lineage_graph_validation = load("verification-lineage-graph-validation.json")
if lineage_graph_validation != {
    "digest": lineage_graph["digest"], "graph_id": lineage_graph["graph_id"],
    "valid": True, "version": lineage_graph["version"],
}:
    raise SystemExit("verification-lineage graph validation drifted")
if lineage_graph["development_fixtures"] != 2 or lineage_graph["empirical"] or lineage_graph["provider_ranking"]:
    raise SystemExit("verification-lineage graph crossed its development-only claim boundary")
if lineage_graph["loss_certificate_digest"] != loss_certificate["digest"] or lineage_graph["paths"][1]["evidence_digest"] != loss_certificate["digest"]:
    raise SystemExit("verification-lineage graph detached from its loss certificate")
lineage_svg = (root / "verification-lineage-graph.svg").read_text()
for fragment in ("No empirical claim", "PASS  survived", "STOP  first_loss", "WAIT  not_reached"):
    if fragment not in lineage_svg:
        raise SystemExit(f"verification-lineage SVG lost {fragment!r}")

offline_bom = load("verification-lineage-offline-bom.json")
offline_bom_validation = load("verification-lineage-offline-bom-validation.json")
if offline_bom_validation != {
    "digest": offline_bom["header"]["digest"], "object_id": offline_bom["header"]["object_id"],
    "valid": True, "version": offline_bom["header"]["schema_version"],
}:
    raise SystemExit("verification-lineage offline BOM validation drifted")
if not offline_bom["accepted"] or offline_bom["retention"]["truncated_required_fields"] is not None:
    raise SystemExit("verification-lineage offline BOM lost its accepted complete-retention state")
if offline_bom["retention"]["surviving_channels"] != offline_bom["evidence"]["decisive_channels"]:
    raise SystemExit("verification-lineage offline BOM lost a decisive channel")

offline_audit = load("verification-lineage-offline-audit.json")
offline_audit_validation = load("verification-lineage-offline-audit-validation.json")
if offline_audit_validation != {
    "digest": offline_audit["header"]["digest"], "object_id": offline_audit["header"]["object_id"],
    "valid": True, "version": offline_audit["header"]["schema_version"],
}:
    raise SystemExit("verification-lineage offline audit validation drifted")
offline_terminal_counts = {item["state"]: item["count"] for item in offline_audit["formats"][0]["terminal_counts"]}
if offline_audit["included_task_groups"] != 1 or offline_terminal_counts["direct_verification_invocation"] != 1 or sum(offline_terminal_counts.values()) != 1:
    raise SystemExit("verification-lineage offline audit conservation drifted")

development_card = load("verification-lineage-development-dataset-card.json")
development_card_validation = load("verification-lineage-development-dataset-card-validation.json")
if development_card_validation != {
    "digest": development_card["header"]["digest"], "object_id": development_card["header"]["object_id"],
    "valid": True, "version": development_card["header"]["schema_version"],
}:
    raise SystemExit("verification-lineage development dataset-card validation drifted")
development_counts = {item["name"]: item["value"] for item in development_card["counts"]}
if development_counts != {
    "accepted_boms": 1, "adapter_conformance_checks": 504, "development_fixtures": 2,
    "empirical_task_groups": 0, "golden_vectors": 63, "research_admitted_sources": 0,
} or development_card["provider_calls_required"] != 0:
    raise SystemExit("verification-lineage development dataset-card promoted empirical evidence")

limitations = load("verification-lineage-limitations.json")
limitations_validation = load("verification-lineage-limitations-validation.json")
if limitations_validation != {
    "digest": limitations["digest"], "object_id": limitations["ledger_id"],
    "valid": True, "version": limitations["version"],
}:
    raise SystemExit("verification-lineage limitations validation drifted")
if len(limitations["limitations"]) != 7 or limitations["corpus_feasibility"] != "not_feasible_current_generation":
    raise SystemExit("verification-lineage limitations ledger weakened")

development_release = load("verification-lineage-development-release.json")
development_release_validation = load("verification-lineage-development-release-validation.json")
if development_release_validation != {
    "digest": development_release["header"]["digest"], "object_id": development_release["header"]["object_id"],
    "valid": True, "version": development_release["header"]["schema_version"],
}:
    raise SystemExit("verification-lineage development release validation drifted")
if len(development_release["files"]) != 20 or development_release["provider_calls_required"] != 0 or not development_release["restricted_material_excluded"]:
    raise SystemExit("verification-lineage development release boundary drifted")
manifest = {item["path"]: item["digest"] for item in development_release["files"]}
for path, digest in manifest.items():
    if hashlib.sha256(pathlib.Path(path).read_bytes()).hexdigest() != digest:
        raise SystemExit(f"verification-lineage release file drifted: {path}")
for narrative_path in (pathlib.Path("eval/results/README.md"),):
    narrative = " ".join(narrative_path.read_text().split())
    for fragment in (
        "20 files", "two development fixtures", "zero empirical task groups",
        "zero research-admitted sources", "not a provider", "limitations ledger",
    ):
        if fragment not in narrative:
            raise SystemExit(f"{narrative_path}: verification-lineage narrative lost {fragment!r}")

v3_plan_path = pathlib.Path("eval/governance/controlled-corruption-v3-plan.json")
v3_audit_path = pathlib.Path("eval/governance/controlled-corruption-v3-natural-audit.json")
v3_release_path = pathlib.Path("eval/governance/controlled-corruption-v3-release.json")
v3_plan = json.loads(v3_plan_path.read_text())
v3_audit = json.loads(v3_audit_path.read_text())
v3_release = json.loads(v3_release_path.read_text())
if v3_plan["digest"] != "5a7f4f55aafabd03ef9c802a9c48cf198a778228b40ade35e8f074df8d060c85":
    raise SystemExit("v3 natural-corpus plan claim drifted")
if hashlib.sha256(v3_plan_path.read_bytes()).hexdigest() != "35414f50cef31e2047959b477566de3119acfd43f78724163e760590f64514e6":
    raise SystemExit("v3 natural-corpus plan bytes drifted")
if v3_audit["digest"] != "af0c0fd56fb498586096a8776e0d40794ee93acf5afda67cc000e576bfcef4d2":
    raise SystemExit("v3 natural-corpus audit claim drifted")
if hashlib.sha256(v3_audit_path.read_bytes()).hexdigest() != "e207ea5ebf3404cf5f89c943d5dba4e645b034c8dc57c7488d643afd34c956d2":
    raise SystemExit("v3 natural-corpus audit bytes drifted")
if (v3_audit["total_attempts"], v3_audit["applied_attempts"], v3_audit["rejected_attempts"], v3_audit["selected_cases"], v3_audit["quotas_satisfied"]) != (939, 689, 250, 283, False):
    raise SystemExit("v3 natural-corpus aggregate claim drifted")
if v3_audit["quota_shortfalls"] != [{"id": "omitted_test_evidence", "count": 37}]:
    raise SystemExit("v3 omitted-evidence scarcity claim drifted")
if v3_release["digest"] != "9b4999dafe2d37ea04c298b80a7aba0a1769755fdfd650cd01bf3a9cc31a2e42":
    raise SystemExit("v3 controlled release claim drifted")
if hashlib.sha256(v3_release_path.read_bytes()).hexdigest() != "37c96112541ef6991e40e6cd8dbaf79d0a02fd84e61fd36f25222be787e5ef42":
    raise SystemExit("v3 controlled release bytes drifted")
policy = v3_release["policy"]
if (policy["core_cases"], policy["scarcity_sentinel_cases"], policy["sentinel_in_primary_estimand"], policy["held_out_sentinel_claim_available"], policy["balanced_eight_family_available"]) != (280, 3, False, False, False):
    raise SystemExit("v3 controlled release claim boundary drifted")
relation_v3_claims = {
    "relation-audit-plan-v3.json": ("6eac462cae0a5b626561d5cbea274a5c3a72c78b6cb9d50a8952be6ccbb6fa8c", "b8dab1afa0516204267ac48089fdbd48553e7571a5e35b9437006fe84fc1177b"),
    "relation-primary-sample-v3.json": ("6b721bcf0fb10e47923b46d92f3c14691cbd0dd98949ab0bf1dc016d8e1c1e43", "fd693e8c4bf582964b7f384d755cf78ff453b870538f597a27972efcc9d574e8"),
    "relation-scarcity-sentinel-v3.json": ("ec720a56394249a47eb4c0f7ef618471ce14c69ff8c2c13e1e851350c23e71fb", "cc314db56830e75a96bbc1bf28f65b2b4c9d0afed6d35ef71b2b3ded12f4923b"),
    "relation-pilot-sample-v3.json": ("3f0a70209575316c78b87f6cb9e7641ff1f4a023eaa86c2fe2598ee14b3c94b4", "4cfe697605898c95e5162ce04ebec0906ed5b784ce1e4b93718f65981ac9fea8"),
    "relation-study-amendment-v3.json": ("a9924c62cdb6cea4aa4b9310af9a31ca5ee3cb193645cb30fd2b1135aab0ed94", "f3fdba56a2d49c0416d1e30dd771808a0fa82e995060b85ba04dc77ad6798321"),
}
for name, (digest, file_hash) in relation_v3_claims.items():
    path = pathlib.Path("eval/governance") / name
    artifact = json.loads(path.read_text())
    if artifact["digest"] != digest or hashlib.sha256(path.read_bytes()).hexdigest() != file_hash:
        raise SystemExit(f"v3 relation governance claim drifted: {name}")
primary_v3 = json.loads((pathlib.Path("eval/governance") / "relation-primary-sample-v3.json").read_text())
sentinel_v3 = json.loads((pathlib.Path("eval/governance") / "relation-scarcity-sentinel-v3.json").read_text())
if (primary_v3["selected_cases"], primary_v3["unique_task_groups"], primary_v3["unique_lineage_clusters"]) != (28, 28, 28):
    raise SystemExit("v3 relation primary independence claim drifted")
if (sentinel_v3["selected_cases"], sentinel_v3["test_cases"], sentinel_v3["held_out_claim_available"]) != (3, 0, False):
    raise SystemExit("v3 relation scarcity claim drifted")

scarcity_evidence = load("relation-scarcity-negative-evidence.json")
scarcity_schema = load("relation-scarcity-public-evidence.schema.json")
scarcity_validation = load("relation-scarcity-public-evidence-validation.json")
if scarcity_evidence["schema_version"] != "evalwitness.relation-scarcity-public-evidence.v1":
    raise SystemExit("public scarcity evidence schema identity drifted")
if scarcity_evidence["digest"] != "f401b84549b013f20deb9c78366f28b609fa7ff83cad330ac7e01617cd466d4f":
    raise SystemExit("public scarcity evidence content digest drifted")
if scarcity_validation != {"valid": True, "type": "scarcity-public-evidence", "digest": scarcity_evidence["digest"]}:
    raise SystemExit("public scarcity evidence standalone validation drifted")
if scarcity_evidence["availability"] != {"target": 40, "attempted": 198, "admitted": 3, "rejected": 195, "selected": 3, "shortfall": 37, "exhaustive": True}:
    raise SystemExit("public scarcity evidence availability funnel drifted")
claim_states = {item["id"]: item["status"] for item in scarcity_evidence["claims"]}
if claim_states != {
    "frozen_corpus_availability": "supported", "held_out_omitted_evidence_validity": "unsupported",
    "human_construct_agreement": "not_run", "verifier_robustness": "not_measured",
    "provider_behavior": "not_measured", "population_prevalence": "unsupported",
    "external_action": "not_authorized",
}:
    raise SystemExit("public scarcity evidence claim boundary drifted")
schema_properties = scarcity_schema.get("properties", {})
if scarcity_schema.get("additionalProperties") is not False or schema_properties.get("schema_version", {}).get("const") != scarcity_evidence["schema_version"]:
    raise SystemExit("public scarcity evidence schema is open or unbound")

owner_attestation_path = pathlib.Path("eval/results/relation-owner-inspection-attestation.json")
owner_attestation = json.loads(owner_attestation_path.read_text())
owner_schema = load("relation-owner-inspection-public-attestation.schema.json")
owner_validation = load("relation-owner-inspection-public-attestation-validation.json")
if owner_attestation["schema_version"] != "evalwitness.relation-owner-inspection-public-attestation.v1":
    raise SystemExit("public owner-inspection attestation schema identity drifted")
if owner_attestation["digest"] != "fd2c364fee2d575120ae4fc29e07788fe5f4107c63f2828da5add916dc7e2a84":
    raise SystemExit("public owner-inspection attestation content digest drifted")
if hashlib.sha256(owner_attestation_path.read_bytes()).hexdigest() != "7304efbce27d68746f75180b4296c7b76fddfef0f9a4c53a9522db36d5d13fe8":
    raise SystemExit("public owner-inspection attestation bytes drifted")
if owner_validation != {"valid": True, "type": "owner-inspection-public-attestation", "digest": owner_attestation["digest"]}:
    raise SystemExit("public owner-inspection attestation standalone validation drifted")
if owner_attestation["assessments"] != {
    "required": 66, "journal_events": 66, "corrections": 0, "core": 50,
    "scarcity_cases": 12, "scarcity_boundary": 4, "completed": 66,
    "core_cases": 7, "scarcity_case_count": 3, "scarcity_test_cases": 0,
}:
    raise SystemExit("public owner-inspection assessment denominators drifted")
if owner_attestation["outcomes"] != {
    "core": {"accepted": 7, "revision_required": 0, "unresolved": 0},
    "scarcity_cases": {"accepted": 1, "revision_required": 2, "unresolved": 0},
    "scarcity_boundary": "accepted", "core_status": "passed",
    "scarcity_status": "revision_required", "overall_status": "revision_required",
}:
    raise SystemExit("public owner-inspection aggregate outcomes drifted")
owner_claim_states = {item["id"]: item["status"] for item in owner_attestation["claims"]}
if owner_claim_states != {
    "public_document_integrity": "supported", "private_owner_inspection": "owner_attested",
    "core_construct_status": "owner_attested", "scarcity_construct_status": "owner_attested",
    "formal_human_study": "not_run", "provider_or_verifier_performance": "not_run",
    "external_action": "not_authorized", "population_or_held_out_validity": "unsupported",
    "public_source_reproduction": "not_publicly_reproducible", "corrected_corpus_feasibility": "unsupported",
}:
    raise SystemExit("public owner-inspection claim boundary drifted")
owner_disclosure = owner_attestation["disclosure"]
if owner_disclosure["private_journal_identities_disclosed"] or owner_disclosure["restricted_evidence_disclosed"]:
    raise SystemExit("public owner-inspection attestation claims a restricted disclosure")
if owner_attestation["human_study_status"] != "not_run" or owner_attestation["external_action_status"] != "not_authorized":
    raise SystemExit("public owner-inspection attestation promoted human or external authority")
owner_schema_properties = owner_schema.get("properties", {})
if owner_schema.get("additionalProperties") is not False or owner_schema_properties.get("schema_version", {}).get("const") != owner_attestation["schema_version"]:
    raise SystemExit("public owner-inspection attestation schema is open or unbound")

if (root / "terminal.json").exists():
    terminal = load("terminal.json")
    if terminal["schema_version"] != "evalwitness.evaluation.v2":
        raise SystemExit("Terminal-Bench dry-run emitted the wrong evidence-era schema")
    if terminal["tasks"] != 89 or terminal["trials"] != 445:
        raise SystemExit("Terminal-Bench dataset shape mismatch")
    close(terminal["pass_at_1_score"], 72.8)
    close(terminal["oracle_score"], 80.0)
    if not terminal["dry_run"]:
        raise SystemExit("Terminal-Bench claimcheck must be dry_run")
    terminal_plan = terminal.get("plan") or {}
    terminal_design = terminal_plan.get("statistical_design") or {}
    if terminal_plan.get("swing_tasks") != 17:
        raise SystemExit("Terminal-Bench preflight did not plan exactly the 17 decidable tasks")
    if terminal_design.get("total_tasks") != 89 or terminal_design.get("decidable_tasks") != 17:
        raise SystemExit("Terminal-Bench preflight lost total/decidable task accounting")
    if [row["disagreement_rate"] for row in terminal_design.get("disagreement_sensitivity", [])] != [0, 0.02, 0.14, 0.31]:
        raise SystemExit("Terminal-Bench preflight disagreement sensitivity drifted")
    if any(row.get("minimum_detectable_paired_effect_adjusted") is not None for row in terminal_design["disagreement_sensitivity"]):
        raise SystemExit("Terminal-Bench preflight falsely claims 80% power under an observed disagreement scenario")

if (root / "swebench.json").exists():
    swe = load("swebench.json")
    if swe["schema_version"] != "evalwitness.evaluation.v2":
        raise SystemExit("SWE-bench dry-run emitted the wrong evidence-era schema")
    if swe["tasks"] != 500 or swe["trials"] != 1500:
        raise SystemExit("SWE-bench dataset shape mismatch")
    close(swe["pass_at_1_score"], 380.3333333333333, eps=1e-6)
    close(swe["oracle_score"], 422.0)
    if not swe["dry_run"]:
        raise SystemExit("SWE-bench claimcheck must be dry_run")
    swe_plan = swe.get("plan") or {}
    swe_design = swe_plan.get("statistical_design") or {}
    if swe_plan.get("swing_tasks") != 86:
        raise SystemExit("SWE-bench preflight did not plan exactly the 86 decidable tasks")
    if swe_design.get("total_tasks") != 500 or swe_design.get("decidable_tasks") != 86:
        raise SystemExit("SWE-bench preflight lost total/decidable task accounting")
    swe_rows = {row["disagreement_rate"]: row for row in swe_design.get("disagreement_sensitivity", [])}
    close(swe_rows[0.14]["minimum_detectable_paired_effect_adjusted"], 0.1135967896141978)
    close(swe_rows[0.31]["minimum_detectable_paired_effect_adjusted"], 0.17337030839351864)

def reject_posthoc_power(value, path="root"):
    if isinstance(value, dict):
        for key, child in value.items():
            if key in {"observed_power", "achieved_power", "post_hoc_power"}:
                raise SystemExit(f"forbidden retrospective power field at {path}.{key}")
            reject_posthoc_power(child, f"{path}.{key}")
    elif isinstance(value, list):
        for index, child in enumerate(value):
            reject_posthoc_power(child, f"{path}[{index}]")

for artifact_name in ("terminal.json", "swebench.json"):
    if (root / artifact_name).exists():
        reject_posthoc_power(load(artifact_name))

design_simulation = load("design-simulation.json")
if design_simulation["algorithm"] != "evalwitness.cluster-factorial.v1":
    raise SystemExit("clustered design algorithm identity drifted")
if design_simulation["seed"] != 20260809 or design_simulation["code_digest"] != "f" * 64:
    raise SystemExit("clustered design reproducibility identity drifted")
if design_simulation["source_tasks"] != 40 or design_simulation["aliasing"]["rank"] != 5:
    raise SystemExit("clustered design task count or estimability drifted")
if design_simulation["budget"]["required_calls"] != 164:
    raise SystemExit("clustered design hard-call accounting drifted")
simulation_terms = {row["term"]: row for row in design_simulation["operating_characteristics"]}
close(simulation_terms["executable_failure"]["power"], 0.98)
close(simulation_terms["irrelevant_formatting"]["power"], 0.04)

paired_resolution = (root / "paired-resolution.txt").read_text()
for fragment in [
    "tasks shared: 89    decidable (paired): 17",
    "discordant pairs       2",
    "paired effect A-B   +0.1176 tasks",
    "90.0% CI [-0.0201, +0.2753]",
    "McNemar exact, two-sided: p = 0.5000",
    "no split can reject at the adjusted alpha",
    "typed inference: equivalence margin=0.0500 -> equivalence_not_established",
]:
    if fragment not in paired_resolution:
        raise SystemExit(f"paired resolution output drifted: missing {fragment!r}")

replay = (root / "replay.txt").read_text()
if "Replay smoke passed." not in replay:
    raise SystemExit("replay smoke did not pass")

# Every statistical claim in the README is recomputed from the published detail
# artifacts, so the prose cannot drift away from the evidence it cites. These
# artifacts are committed, so this runs without a provider.
results_dir = pathlib.Path("eval/results")
expected_details = {
    "terminal-bench-verifier.json": ("verifier", 78.0, 0, 68),
    "terminal-bench-judge.json": ("judge", 76.0, 30, 68),
    "swebench-verifier.json": ("verifier", 385.0, 0, 172),
    "swebench-judge.json": ("judge", 387.0, 17, 172),
    # Configuration variants: same trajectories, so the README compares them
    # paired. Absolute selection scores trajectories in isolation and therefore
    # records no pairwise decisions at all.
    "terminal-bench-all-pairs.json": ("verifier", 78.0, 0, 170),
    "terminal-bench-evidence-16k.json": ("verifier", 75.0, 0, 68),
    "terminal-bench-absolute.json": ("verifier", 75.0, 0, 0),
    # Reference pipeline on SWE-bench: 4 reps x 3 pairs x 3 criteria over the
    # 86 selecting tasks, the highest-resolution observed comparison here. Parity
    # runs the reference all-pairs path, which records no per-pair decisions,
    # so the tie counters are structurally zero rather than measured.
    "swebench-paper-parity.json": ("verifier", 386.0, 0, 0),
    # The size-prior ablation: byte-identical criteria minus the two clauses
    # telling the model to disregard patch size. Six decidable tasks, and the
    # difference between failing and clearing the random baseline.
    "swebench-size-agnostic.json": ("verifier", 391.0, 0, 172),
}
for name, (want_mode, want_score, want_ties, want_pairs) in expected_details.items():
    path = results_dir / name
    if not path.exists():
        raise SystemExit(f"missing published artifact: {path}")
    doc = json.loads(path.read_text())
    if doc.get("schema_version") is not None:
        raise SystemExit(f"{name}: historical naked-score artifact must remain explicitly legacy/unversioned")
    usage = doc["usage"]
    if usage["extraction_mode"] != want_mode:
        raise SystemExit(f"{name}: extraction_mode {usage['extraction_mode']} != {want_mode}")
    if usage["unextracted_scores"] != 0:
        raise SystemExit(f"{name}: unextracted scores present, not publishable")
    close(doc["verifier_score"], want_score, eps=1e-6)
    ties = pairs = 0
    for row in doc["details"]:
        for decision in ((row.get("selection") or {}).get("pair_decisions") or []):
            pairs += 1
            if decision["mean_difference"] == 0.0:
                ties += 1
    if (ties, pairs) != (want_ties, want_pairs):
        raise SystemExit(f"{name}: tie rate {ties}/{pairs} != {want_ties}/{want_pairs}")

# Baselines cost nothing and are the comparison the README now leads the
# SWE-bench section with, so every figure it quotes is recomputed here.
expected_baselines = {
    "swebench-verifier.json": {
        "most patch hunks": (56, 14, 21),
        "first listed": (48, 20, 19),
    },
    "terminal-bench-verifier.json": {
        "fewest error words": (12, 4, 1),
        "fewest duration": (12, 4, 1),
    },
}
for name, wanted in expected_baselines.items():
    doc = json.loads((results_dir / name).read_text())
    got = {b["name"]: b for b in doc.get("baselines", [])}
    for label, (solved, subject_only, baseline_only) in wanted.items():
        b = got.get(label)
        if b is None:
            raise SystemExit(f"{name}: baseline {label!r} missing")
        actual = (b["decidable_solved"], b["subject_only"], b["baseline_only"])
        if actual != (solved, subject_only, baseline_only):
            raise SystemExit(f"{name}: {label} = {actual}, want {(solved, subject_only, baseline_only)}")

# The strongest SWE-bench baseline outscores the verifier. That is the least
# comfortable number in the README and therefore the one most worth pinning.
swe = json.loads((results_dir / "swebench-verifier.json").read_text())
strongest = max(swe["baselines"], key=lambda b: b["decidable_solved"])
if strongest["name"] != "most patch hunks":
    raise SystemExit(f"strongest SWE-bench baseline is {strongest['name']!r}, README says most patch hunks")
verifier_decidable = sum(
    1 for row in swe["details"]
    if row["rewards"] and 0 < sum(1 for r in row["rewards"] if r) < len(row["rewards"])
    and row.get("selected_reward")
)
if strongest["decidable_solved"] <= verifier_decidable:
    raise SystemExit(
        "the strongest baseline no longer outscores the verifier "
        f"({strongest['decidable_solved']} vs {verifier_decidable}); the README section must be rewritten"
    )

# Calibration is a mechanism measurement used to interpret the selection
# results, so the figures quoted in the README are recomputed rather than
# trusted. Tolerances are loose
# enough to survive a regeneration and tight enough to catch a real change.
expected_calibration = {
    "terminal-bench-verifier.json": (36, 0.2024, 0.8281, 0.7778),
    "terminal-bench-judge.json": (34, 0.1281, 0.8561, 0.7647),
    "swebench-verifier.json": (108, 0.3365, 0.6208, 0.5463),
    "swebench-judge.json": (105, 0.3216, 0.6255, 0.5714),
}
for name, (count, ece, auc, accuracy) in expected_calibration.items():
    cal = json.loads((results_dir / name).read_text()).get("calibration")
    if cal is None:
        raise SystemExit(f"{name}: no calibration block; regenerate the artifact")
    if cal["count"] != count:
        raise SystemExit(f"{name}: {cal['count']} decidable pairs, README says {count}")
    close(cal["ece"], ece, eps=1e-3)
    close(cal["auc"], auc, eps=1e-3)
    close(cal["accuracy"], accuracy, eps=1e-3)
    if cal["monotone"]:
        raise SystemExit(f"{name}: reliability curve is now monotone; the README says none is")

# Uncertainty and error-strata figures come from a second implementation over
# task-clustered source rows. Pin the uncomfortable intervals and large wrong-
# direction counts without converting overlap into equivalence.
calibration_text = (root / "calibration.txt").read_text()
for fragment in [
    "terminal-bench-verifier.json   schema=logprobe.reliability.v1   role=development",
    "AUC     0.828  95% [0.638, 0.980], tasks=17",
    "large (.20,1]             2",
    "terminal-bench-absolute.json   schema=logprobe.reliability.v1   role=development",
    "Spearman 0.377  95% [0.183, 0.564], tasks=17",
    "swebench-verifier.json   schema=logprobe.reliability.v1   role=development",
    "AUC     0.621  95% [0.510, 0.731], tasks=86",
    "large (.20,1]            15",
    "raw gate invalid, held-out recalibration remains untested",
]:
    if fragment not in calibration_text:
        raise SystemExit(f"calibration analysis no longer contains expected evidence: {fragment!r}")

calibration_manifest = load("calibration.json")
if calibration_manifest["schema_version"] != "logprobe.reliability.v1":
    raise SystemExit("calibration manifest schema drifted")
if calibration_manifest["data_role"] != "development":
    raise SystemExit("calibration manifest is not development-only")
if calibration_manifest["cluster_unit"] != "task_id":
    raise SystemExit("calibration manifest no longer clusters by task")
expected_error_rows = {
    "terminal-bench-verifier.json": {
        "errors": 8, "bands": [6, 0, 2],
        "ece": (0.202, 0.115, 0.345), "auc": (0.828, 0.638, 0.980),
        "accuracy": (0.778, 0.622, 0.921),
    },
    "terminal-bench-judge.json": {
        "errors": 3, "bands": [2, 0, 1],
        "ece": (0.128, 0.072, 0.255), "auc": (0.856, 0.681, 0.985),
        "accuracy": (0.765, 0.646, 0.892),
    },
    "swebench-verifier.json": {
        "errors": 49, "bands": [22, 12, 15],
        "ece": (0.337, 0.252, 0.433), "auc": (0.621, 0.510, 0.731),
        "accuracy": (0.546, 0.454, 0.643),
    },
    "swebench-judge.json": {
        "errors": 40, "bands": [13, 14, 13],
        "ece": (0.322, 0.249, 0.418), "auc": (0.625, 0.515, 0.734),
        "accuracy": (0.571, 0.479, 0.658),
    },
}
for report in calibration_manifest["reports"]:
    artifact_name = pathlib.Path(report["artifact"]).name
    if artifact_name == "terminal-bench-absolute.json":
        absolute = report.get("absolute")
        if absolute is None or len(absolute["source_rows"]) != 85:
            raise SystemExit("terminal-bench-absolute.json: missing 85 absolute source rows")
        spearman = absolute["spearman"]
        close(spearman["point"], 0.377, eps=1e-3)
        close(spearman["interval"]["lower"], 0.183, eps=1e-3)
        close(spearman["interval"]["upper"], 0.564, eps=1e-3)
        if spearman["count"] != 85 or spearman["low_sample"]:
            raise SystemExit("terminal-bench-absolute.json: invalid Spearman sample metadata")
        continue
    expected = expected_error_rows.get(artifact_name)
    if expected is None:
        raise SystemExit(f"unexpected calibration manifest artifact: {artifact_name}")
    rows = report["error_decomposition"]["source_rows"]
    if len(rows) != expected["errors"]:
        raise SystemExit(f"{artifact_name}: unexpected error-strata source-row count")
    band_counts = [band["count"] for band in report["error_decomposition"]["bands"]]
    if band_counts != expected["bands"]:
        raise SystemExit(f"{artifact_name}: error bands {band_counts} != {expected['bands']}")
    for metric_name in ("ece", "auc", "accuracy"):
        metric = report["metrics"][metric_name]
        interval = metric["interval"]
        want_point, want_lower, want_upper = expected[metric_name]
        close(metric["point"], want_point, eps=1e-3)
        close(interval["lower"], want_lower, eps=1e-3)
        close(interval["upper"], want_upper, eps=1e-3)
    for row in rows:
        required = {
            "source_row_id", "task_id", "pair", "extraction_mode", "predicted",
            "won", "outcome_id", "difference", "calls", "pair_call_limit",
            "order_policy", "first_order", "error_band",
        }
        missing = sorted(required - row.keys())
        if missing:
            raise SystemExit(f"{artifact_name}: error-strata row missing {missing}")
        if row["order_policy"] != "not-recorded-in-legacy-artifact":
            raise SystemExit(f"{artifact_name}: legacy order provenance was invented")

# The ablation crossing is a hypothesis-generating development result: the
# shipped verifier does not clear the random baseline on SWE-bench and the
# size-agnostic one does. Recompute the observation without upgrading it into a
# causal claim.
def decidable_solved(doc):
    return sum(
        1 for row in doc["details"]
        if row["rewards"] and 0 < sum(1 for r in row["rewards"] if r) < len(row["rewards"])
        and row.get("selected_reward")
    )

shipped = json.loads((results_dir / "swebench-verifier.json").read_text())
ablated = json.loads((results_dir / "swebench-size-agnostic.json").read_text())
if decidable_solved(shipped) != 49:
    raise SystemExit(f"shipped verifier solves {decidable_solved(shipped)} decidable tasks, README says 49")
if decidable_solved(ablated) != 55:
    raise SystemExit(f"size-agnostic verifier solves {decidable_solved(ablated)} decidable tasks, README says 55")

# The verifier ties zero times in either benchmark. That is the paper's stated
# mechanism and the one claim in the README that reproduces outright.
verifier_ties = sum(
    1
    for name, (mode, *_) in expected_details.items()
    if mode == "verifier"
    for row in json.loads((results_dir / name).read_text())["details"]
    for decision in ((row.get("selection") or {}).get("pair_decisions") or [])
    if decision["mean_difference"] == 0.0
)
if verifier_ties != 0:
    raise SystemExit(f"verifier tied {verifier_ties} pairs; README claims none in any run")

print("Claimcheck passed.")
PY
