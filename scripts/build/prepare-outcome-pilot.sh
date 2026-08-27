#!/usr/bin/env bash
# Prepare the complete owner-only TASK 057 outcome pilot package without
# contacting reviewers or providers. The destination must not already exist.

set -euo pipefail
umask 077

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)

binary=""
key_file=""
key_id=""
package_root=""
bundle_created_at=""
protocol_created_at=""
readiness_prepared_at=""

usage() {
  cat >&2 <<'EOF'
usage: scripts/build/prepare-outcome-pilot.sh \
  --key-file PATH --key-id ID --package-root PATH \
  --bundle-created-at RFC3339 --protocol-created-at RFC3339 \
  --readiness-prepared-at RFC3339 [--binary PATH]

The command prepares restricted review items, sealed private materials, the
development review bundle, structural inspection, blinding protocol, and v3
readiness artifact below a new owner-only package root. It never generates,
prints, sends, or replaces a key or package.
EOF
}

while (($# > 0)); do
  case "$1" in
    --binary)
      binary=${2:-}
      shift 2
      ;;
    --key-file)
      key_file=${2:-}
      shift 2
      ;;
    --key-id)
      key_id=${2:-}
      shift 2
      ;;
    --package-root)
      package_root=${2:-}
      shift 2
      ;;
    --bundle-created-at)
      bundle_created_at=${2:-}
      shift 2
      ;;
    --protocol-created-at)
      protocol_created_at=${2:-}
      shift 2
      ;;
    --readiness-prepared-at)
      readiness_prepared_at=${2:-}
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if [[ -z "$key_file" || -z "$key_id" || -z "$package_root" || -z "$bundle_created_at" || -z "$protocol_created_at" || -z "$readiness_prepared_at" ]]; then
  echo "error: all key, package, and timestamp arguments are required" >&2
  usage
  exit 2
fi

cd "$repo_root"

if [[ ! -f "$key_file" || -L "$key_file" ]]; then
  echo "error: --key-file must name an existing regular non-symlink file" >&2
  exit 1
fi
if [[ -e "$package_root" || -L "$package_root" ]]; then
  echo "error: --package-root already exists; prepared pilot packages are immutable" >&2
  exit 1
fi
if [[ ! -d eval/trajectories/terminal_trajs/forge_gpt54 || ! -d eval/trajectories/swebench_verified_trajs ]]; then
  echo "error: fetched Terminal-Bench and SWE-bench trajectory assets are required" >&2
  exit 1
fi

package_parent=$(dirname "$package_root")
package_name=$(basename "$package_root")
if [[ "$package_name" == "." || "$package_name" == ".." || "$package_name" == "/" || -z "$package_name" ]]; then
  echo "error: --package-root must name a new specific directory" >&2
  exit 1
fi
mkdir -p "$package_parent"

staging_root=$(mktemp -d "$package_parent/.${package_name}.candidate.XXXXXXXX")
work_dir=$(mktemp -d /tmp/evalwitness-outcome-pilot-prepare-XXXXXXXX)
published=false

cleanup() {
  if [[ "$published" != true && -n "$staging_root" && -d "$staging_root" ]]; then
    rm -rf "$staging_root"
  fi
  if [[ -n "$work_dir" && -d "$work_dir" ]]; then
    rm -rf "$work_dir"
  fi
}
trap cleanup EXIT

if [[ -z "$binary" ]]; then
  binary="$work_dir/evalwitness"
  go build -o "$binary" ./cmd/evalwitness
elif [[ ! -f "$binary" || -L "$binary" || ! -x "$binary" ]]; then
  echo "error: --binary must name an executable regular non-symlink file" >&2
  exit 1
fi

pilot_digest=$(python3 - <<'PY'
import json
import pathlib

value = json.loads(pathlib.Path("eval/governance/outcome-pilot-sample-v2.json").read_text(encoding="utf-8"))
digest = value.get("digest", "")
if len(digest) != 64 or any(character not in "0123456789abcdef" for character in digest):
    raise SystemExit("governed pilot digest is invalid")
print(digest)
PY
)

items_path="$work_dir/restricted-review-items.json"
"$binary" outcome pilot-materials \
  --root . \
  --plan @eval/governance/outcome-adjudication-v1.json \
  --natural-request @eval/governance/outcome-natural-inventory-request-v1.json \
  --inventory @eval/governance/outcome-natural-inventory-v1.json \
  --pilot-sample @eval/governance/outcome-pilot-sample-v2.json \
  --key-file "$key_file" \
  --key-id "$key_id" \
  --private-root "$staging_root" > "$items_path"

private_materials="$staging_root/outcome-pilots/$pilot_digest.json"
"$binary" outcome validate --type pilot-private-materials --document @"$private_materials" > /dev/null

cp "$items_path" "$staging_root/restricted-review-items.json"
cmp "$items_path" "$staging_root/restricted-review-items.json" > /dev/null

"$binary" outcome review bundle \
  --plan @eval/governance/outcome-adjudication-v1.json \
  --qualification-set @eval/governance/outcome-qualification-v1.json \
  --handbook @eval/governance/outcome-reviewer-handbook-v1.json \
  --items @"$staging_root/restricted-review-items.json" \
  --data-role development \
  --visibility restricted \
  --created-at "$bundle_created_at" > "$staging_root/review-bundle.json"

"$binary" outcome pilot-inspect \
  --pilot-sample @eval/governance/outcome-pilot-sample-v2.json \
  --bundle @"$staging_root/review-bundle.json" \
  --private-materials @"$private_materials" > "$staging_root/pilot-inspection.json"

"$binary" outcome review blinding-protocol \
  --bundle @"$staging_root/review-bundle.json" \
  --private-materials @"$private_materials" \
  --created-at "$protocol_created_at" > "$staging_root/blinding-protocol.json"

"$binary" outcome review pilot-readiness \
  --pilot-sample @eval/governance/outcome-pilot-sample-v2.json \
  --bundle @"$staging_root/review-bundle.json" \
  --qualification-set @eval/governance/outcome-qualification-v1.json \
  --handbook @eval/governance/outcome-reviewer-handbook-v1.json \
  --protocol @"$staging_root/blinding-protocol.json" \
  --inspection @"$staging_root/pilot-inspection.json" \
  --private-materials @"$private_materials" \
  --prepared-at "$readiness_prepared_at" > "$staging_root/pilot-readiness.json"

"$binary" outcome validate --type review-bundle --document @"$staging_root/review-bundle.json" > /dev/null
"$binary" outcome validate --type pilot-inspection --document @"$staging_root/pilot-inspection.json" > /dev/null
"$binary" outcome validate --type blinding-protocol --document @"$staging_root/blinding-protocol.json" > /dev/null
"$binary" outcome validate --type pilot-readiness --document @"$staging_root/pilot-readiness.json" > /dev/null

find "$staging_root" -type d -exec chmod 0700 {} +
find "$staging_root" -type f -exec chmod 0600 {} +

if [[ -e "$package_root" || -L "$package_root" ]]; then
  echo "error: --package-root appeared during preparation; refusing to replace it" >&2
  exit 1
fi
mv "$staging_root" "$package_root"
published=true

echo "Outcome pilot package prepared without provider or reviewer action."
echo "Package root: $package_root"
echo "Pilot digest: $pilot_digest"
echo "Readiness: ready_for_authorization; external_action_status=not_authorized"
