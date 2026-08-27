#!/usr/bin/env bash
# Fail closed when repository-owned public reports or documentation contain
# credentials, workstation paths, environment dumps, symlinks, or unsafe modes.

set -euo pipefail
cd "$(dirname "$0")/../.."

if ! command -v rg >/dev/null 2>&1; then
  echo "artifact safety: ripgrep (rg) is required" >&2
  exit 1
fi

if ! rg -qx '/private/' .gitignore; then
  echo "artifact safety: owner-only /private/ study material is not gitignored" >&2
  exit 1
fi

go run ./cmd/evalwitness artifact scan \
  --class public \
  --reviewed-findings config/tracked-public-reviewed-findings.json \
  --path README.md \
  --path THIRD_PARTY_NOTICES.md \
  --path assets \
  --path docs/documentation.md \
  --path docs/releasing.md \
  --path docs/spec.md \
  --path .agents/skills/evalwitness-audit/SKILL.md \
  --path config/eval-data-reviewed-findings.json \
  --path config/tracked-public-reviewed-findings.json \
  --path eval/README.md \
  --path eval/fixtures/controlled-corruption \
  --path eval/governance \
  --path scripts/build/prepare-outcome-pilot.sh \
  --path scripts/build/prepare-relation-pilot.sh \
  --path scripts/build/pack-response-bundle.sh \
  --path scripts/build/build-go-module-proxy.sh \
  --path scripts/build/build-release-candidate.sh \
  --path scripts/evals/reproduce-public-evidence.sh \
  --path scripts/audits/verify-relation-pilot-package.sh \
  --path scripts/audits/run-relation-construct.sh \
  --path scripts/tests/run-race.sh \
  --path scripts/tests/run-response-bundle.sh \
  --path scripts/tests/run-release-roundtrip.sh \
  --path scripts/tests/response-bundle-policy-draft.json \
  --path eval/results
