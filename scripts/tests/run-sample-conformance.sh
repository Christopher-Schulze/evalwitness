#!/usr/bin/env bash
# TASK 059 sample-repository conformance: the isolated consumer repo under
# docs/sample-consumer/ must stay byte-aligned with the shipped composite
# action, workflow, policy, and reference profile so the copy-paste
# integration cannot silently rot.
set -euo pipefail
cd "$(dirname "$0")/../.."

cmp .github/actions/evalwitness-audit/action.yml \
    docs/sample-consumer/.github/actions/evalwitness-audit/action.yml
cmp config/policies/minimal-public.json \
    docs/sample-consumer/config/minimal-public.json
cmp scripts/tests/reference-profile-for-policies.json \
    docs/sample-consumer/reference-profile-for-policies.json

python3 - << 'PYEOF'
import yaml
for path in (".github/workflows/offline-audit.yml",
             "docs/sample-consumer/.github/workflows/offline-audit.yml"):
    doc = yaml.safe_load(open(path))
    assert doc["jobs"], path
print("sample workflows parse OK")
PYEOF

echo "Sample consumer conformance passed."
