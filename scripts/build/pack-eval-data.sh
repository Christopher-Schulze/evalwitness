#!/usr/bin/env bash
# Pack eval/trajectories into release-asset tarballs with checksums.
# Output: dist-eval-data/{terminal_trajs,swebench_verified_trajs}.tar.gz
#         + SHA256SUMS-eval-data
#
# The checksum file is named apart from the binary release's SHA256SUMS on
# purpose: both sets attach to the same GitHub release, and a shared name means
# one silently replaces the other and every verification afterwards checks the
# wrong list.
# Upload with: gh release upload <tag> dist-eval-data/*

set -euo pipefail
cd "$(dirname "$0")/../.."

src_root="eval/trajectories"
out_dir="dist-eval-data"
review_manifest="config/eval-data-reviewed-findings.json"

if [ ! -d "${src_root}" ]; then
  echo "error: ${src_root} not present; nothing to pack" >&2
  exit 1
fi
if [ ! -f "${review_manifest}" ]; then
  echo "error: ${review_manifest} not present; release findings are not reviewed" >&2
  exit 1
fi

echo "==> scanning release corpus"
go run ./cmd/evalwitness artifact scan \
  --class public \
  --path "${src_root}" \
  --reviewed-findings "${review_manifest}" \
  >/dev/null

rm -rf "${out_dir}"
mkdir -p "${out_dir}"
chmod 0755 "${out_dir}"

pack() {
  local name="$1"
  if [ ! -d "${src_root}/${name}" ]; then
    echo "error: ${src_root}/${name} missing" >&2
    exit 1
  fi
  echo "==> packing ${name}"
  # Sorted file list + gzip -n keeps archives as reproducible as BSD/GNU tar allows.
  (cd "${src_root}" && find "${name}" -type f | LC_ALL=C sort | COPYFILE_DISABLE=1 tar --no-xattrs --no-acls --no-fflags -cf - -T -) \
    | gzip -n > "${out_dir}/${name}.tar.gz"
  chmod 0644 "${out_dir}/${name}.tar.gz"
}

pack terminal_trajs
pack swebench_verified_trajs

echo "==> validating archive safety"
go run ./cmd/evalwitness archive inspect \
  --source "${out_dir}/terminal_trajs.tar.gz" \
  --source "${out_dir}/swebench_verified_trajs.tar.gz" \
  --expected-root terminal_trajs \
  --expected-root swebench_verified_trajs

(cd "${out_dir}" && shasum -a 256 ./*.tar.gz > SHA256SUMS-eval-data)
chmod 0644 "${out_dir}/SHA256SUMS-eval-data"

echo "==> done"
ls -lh "${out_dir}"
echo
echo "Upload: gh release upload <tag> ${out_dir}/*"
