#!/usr/bin/env bash
# Run every repository fuzz target for a bounded deterministic gate interval.

set -euo pipefail
cd "$(dirname "$0")/../.."

if [ "$#" -ne 0 ]; then
  echo "usage: scripts/tests/run-fuzz-smoke.sh" >&2
  exit 2
fi

fuzz_time="2s"
targets=(
  "./protocol:FuzzCanonicalizeJSONNeverPanics"
  "./internal/mutation:FuzzMutationStrictDecoders"
  "./internal/outcome:FuzzOutcomeStrictPlanDecoder"
  "./internal/preprocess:FuzzImportTraceNeverPanics"
  "./internal/preprocess:FuzzIngestNeverPanics"
  "./internal/preprocess:FuzzEvidenceBudgetNeverPanics"
  "./internal/provider:FuzzRequestFingerprintDeterministic"
  "./internal/safety:FuzzArchiveHeaderName"
  "./internal/safety:FuzzRouteNamespace"
  "./internal/safety:FuzzPathContainment"
  "./internal/safety:FuzzSecretRedaction"
  "./internal/stress:FuzzStageRecordDigestBindsCanonicalMaterial"
)
run_stress_tests="${EVALWITNESS_ENABLE_STRESS_TESTS:-0}"
if [ "${run_stress_tests}" != "0" ] && [ "${run_stress_tests}" != "1" ]; then
  echo "EVALWITNESS_ENABLE_STRESS_TESTS must be 0 or 1" >&2
  exit 2
fi

discovered="$(git grep -n -E '^func Fuzz[A-Za-z0-9_]+\(' -- '*_test.go' || true)"
discovered_count=0
if [ -n "${discovered}" ]; then
  discovered_count="$(printf '%s\n' "${discovered}" | wc -l | tr -d ' ')"
fi
if [ "${discovered_count}" -ne "${#targets[@]}" ]; then
  echo "fuzz target inventory changed: discovered=${discovered_count} declared=${#targets[@]}" >&2
  exit 1
fi

executed=0
for target in "${targets[@]}"; do
  package="${target%%:*}"
  fuzz_name="${target#*:}"
  if [ "${run_stress_tests}" = "0" ] && [ "${package}" = "./internal/stress" ]; then
    printf '==> SKIP fuzz %s %s (set EVALWITNESS_ENABLE_STRESS_TESTS=1 to opt in)\n' "${package}" "${fuzz_name}"
    continue
  fi
  printf '==> fuzz %s %s\n' "${package}" "${fuzz_name}"
  go test -run '^$' -fuzz "^${fuzz_name}$" -fuzztime "${fuzz_time}" "${package}"
  executed=$((executed + 1))
done

printf 'Fuzz smoke passed: executed=%d inventoried=%d fuzz_time_each=%s\n' "${executed}" "${#targets[@]}" "${fuzz_time}"
