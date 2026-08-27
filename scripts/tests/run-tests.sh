#!/usr/bin/env bash
# Default local test pipeline: format, vet, test, and build.
# Expensive stress and race suites remain available through explicit opt-in.

set -euo pipefail
cd "$(dirname "$0")/../.."

go_source_output="$(git ls-files --cached --others --exclude-standard -- '*.go')"
go_sources=()
while IFS= read -r source_file; do
  if [ -n "${source_file}" ]; then
    go_sources+=("${source_file}")
  fi
done <<<"${go_source_output}"
if [ "${#go_sources[@]}" -eq 0 ]; then
  echo "Go source inventory is empty" >&2
  exit 1
fi

project_package_output="$(scripts/tests/list-project-go-packages.sh)"
project_packages=()
while IFS= read -r package; do
  if [ -n "${package}" ]; then
    project_packages+=("${package}")
  fi
done <<<"${project_package_output}"
if [ "${#project_packages[@]}" -eq 0 ]; then
  echo "Go package inventory is empty" >&2
  exit 1
fi

module_path="$(go list -m -f '{{.Path}}')"
run_stress_tests="${EVALWITNESS_ENABLE_STRESS_TESTS:-0}"
run_race_tests="${EVALWITNESS_ENABLE_RACE_TESTS:-0}"
for toggle in "${run_stress_tests}" "${run_race_tests}"; do
  if [ "${toggle}" != "0" ] && [ "${toggle}" != "1" ]; then
    echo "EVALWITNESS_ENABLE_STRESS_TESTS and EVALWITNESS_ENABLE_RACE_TESTS must be 0 or 1" >&2
    exit 2
  fi
done
stress_package="${module_path}/internal/stress"
heavy_test_packages=(
  "${module_path}/cmd/evalwitness"
  "${module_path}/internal/capsule"
  "${module_path}/internal/claim"
  "${module_path}/internal/explorer"
  "${module_path}/internal/reliance"
)
if [ "${run_stress_tests}" = "1" ]; then
  heavy_test_packages+=("${stress_package}")
fi
heavy_test_found=()
for _ in "${heavy_test_packages[@]}"; do
  heavy_test_found+=(0)
done
standard_test_packages=()
excluded_test_packages=()
for package in "${project_packages[@]}"; do
  if [ "${run_stress_tests}" = "0" ] && [ "${package}" = "${stress_package}" ]; then
    excluded_test_packages+=("${package}")
    continue
  fi
  matched_heavy=0
  for ((index = 0; index < ${#heavy_test_packages[@]}; index++)); do
    if [ "${package}" = "${heavy_test_packages[index]}" ]; then
      heavy_test_found[index]=$((heavy_test_found[index] + 1))
      matched_heavy=1
      break
    fi
  done
  if [ "${matched_heavy}" -eq 0 ]; then
    standard_test_packages+=("${package}")
  fi
done
if [ "${#standard_test_packages[@]}" -eq 0 ]; then
  echo "Go test package assignment is incomplete" >&2
  exit 1
fi
for ((index = 0; index < ${#heavy_test_packages[@]}; index++)); do
  if [ "${heavy_test_found[index]}" -ne 1 ]; then
    echo "Go test heavy package ${heavy_test_packages[index]} occurs ${heavy_test_found[index]} times" >&2
    exit 1
  fi
done
if [ "$(( ${#standard_test_packages[@]} + ${#heavy_test_packages[@]} + ${#excluded_test_packages[@]} ))" -ne "${#project_packages[@]}" ]; then
  echo "Go test package assignment does not cover the project inventory" >&2
  exit 1
fi

echo "==> gofmt"
unformatted=$(gofmt -l "${go_sources[@]}")
if [ -n "${unformatted}" ]; then
  echo "files need formatting:" >&2
  echo "${unformatted}" >&2
  exit 1
fi

echo "==> identity residue"
scripts/tests/run-name-residue.sh

echo "==> artifact safety"
scripts/tests/run-artifact-safety.sh

echo "==> third-party notices"
scripts/tests/run-third-party-notices.sh

echo "==> cross-language request fingerprint vectors"
python3 eval/python-reference/verify_request_fingerprints.py

echo "==> offline verifier-audit protocol conformance"
scripts/audits/run-protocol-conformance.sh

echo "==> trace interoperability"
scripts/audits/run-trace-interoperability.sh

echo "==> controlled trajectory corruption"
scripts/audits/run-controlled-corruption.sh

echo "==> controlled trajectory corruption v2 construct audit"
scripts/audits/run-controlled-corruption-v2.sh

echo "==> controlled trajectory corruption v3 natural-corpus audit"
scripts/audits/run-controlled-corruption-v3.sh

echo "==> coding-agent-only formal study"
scripts/audits/run-agent-only-study.sh

echo "==> controlled-relation v2 governance"
scripts/audits/run-relation-governance-v2.sh

echo "==> controlled-relation v3 governance"
scripts/audits/run-relation-governance-v3.sh

echo "==> controlled-relation construct validity"
scripts/audits/run-relation-construct.sh

echo "==> outcome validity and adjudication governance"
scripts/audits/run-outcome-validity.sh

echo "==> go vet"
go vet "${project_packages[@]}"

echo "==> staticcheck 2026.2.1"
go run honnef.co/go/tools/cmd/staticcheck@2026.2.1 "${project_packages[@]}"

echo "==> Go vulnerability analysis"
scripts/tests/run-vulnerability-scan.sh

echo "==> go test"
go test "${standard_test_packages[@]}"

for ((index = 0; index < ${#heavy_test_packages[@]}; index++)); do
  printf '==> go test: heavy package %d/%d (%s)\n' \
    "$((index + 1))" "${#heavy_test_packages[@]}" "${heavy_test_packages[index]}"
  go test -count=1 -timeout 30m "${heavy_test_packages[index]}"
done

echo "==> product-level MCP and Best-of-N conformance"
scripts/tests/run-product-conformance.sh

echo "==> exact fuzz-target inventory and bounded smoke"
EVALWITNESS_ENABLE_STRESS_TESTS="${run_stress_tests}" scripts/tests/run-fuzz-smoke.sh

echo "==> offline evidence explorer"
scripts/tests/run-evidence-explorer.sh

echo "==> hard-network-offline reproduction"
scripts/evals/reproduce-public-evidence.sh --profile full

if [ "${run_race_tests}" = "1" ]; then
  echo "==> go test -race (explicit opt-in)"
  scripts/tests/run-race.sh
else
  echo "==> SKIP go test -race (set EVALWITNESS_ENABLE_RACE_TESTS=1 to opt in)"
fi

echo "==> reproducible no-overwrite release build"
scripts/tests/run-release-build.sh

echo
echo "All checks passed."
