#!/usr/bin/env bash
# Keep the shipped notice inventory aligned with the exact Go module graph and
# installed production dependency closure used by the Evidence Explorer.

set -euo pipefail
cd "$(dirname "$0")/../.."

if [ "$#" -ne 0 ]; then
  echo "usage: scripts/tests/run-third-party-notices.sh" >&2
  exit 2
fi

notice="THIRD_PARTY_NOTICES.md"
if [ ! -f "${notice}" ]; then
  echo "third-party notice file is missing" >&2
  exit 1
fi
if ! command -v bun >/dev/null 2>&1; then
  echo "Bun is required to verify Explorer licenses" >&2
  exit 1
fi

inventory="$(mktemp "${TMPDIR:-/tmp}/evalwitness-license-inventory.XXXXXX")"
trap 'rm -f "${inventory}"' EXIT
(
  cd web/explorer
  bun pm licenses --json --prod
) > "${inventory}"

python3 - "${notice}" "${inventory}" <<'PY'
import json
import pathlib
import subprocess
import sys

notice = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
licenses = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
repository_root = pathlib.Path.cwd()

allowed = {"0BSD", "Apache-2.0", "ISC", "MIT"}
observed = set(licenses)
if observed != allowed:
    raise SystemExit(f"Explorer license set changed: {sorted(observed)}")

packages = []
for declared, entries in licenses.items():
    for entry in entries:
        if not entry.get("license") or entry["license"] != declared:
            raise SystemExit(f"invalid license metadata for {entry.get('name', '<unknown>')}")
        for version in entry.get("versions", []):
            packages.append(f"{entry['name']}@{version}")

module_output = subprocess.run(
    ["go", "list", "-m", "-f", "{{if not .Main}}{{.Path}}@{{.Version}}{{end}}", "all"],
    check=True,
    stdout=subprocess.PIPE,
    text=True,
).stdout.splitlines()
expected = sorted(set(packages + [value for value in module_output if value]))
go_directive = next(
    line.split()[1]
    for line in (repository_root / "go.mod").read_text(encoding="utf-8").splitlines()
    if line.startswith("go ")
)
expected.extend([f"Go standard library go{go_directive}", "tailwindcss@4.3.3"])
missing = [value for value in expected if value not in notice]
if missing:
    raise SystemExit("third-party notices missing: " + ", ".join(missing))

for heading in ("BSD-3-Clause license", "MIT license", "0BSD license", "ISC license", "Apache License 2.0"):
    if f"## {heading}" not in notice:
        raise SystemExit(f"third-party license text missing: {heading}")

for relative_path in (
    "web/explorer/node_modules/class-variance-authority/LICENSE",
    "web/explorer/node_modules/lucide-react/LICENSE",
):
    license_text = (repository_root / relative_path).read_text(encoding="utf-8")
    if license_text not in notice:
        raise SystemExit(f"third-party license text differs from installed source: {relative_path}")

print(f"Third-party notices passed: go_modules={len([v for v in module_output if v])} explorer_packages={len(packages)}")
PY
