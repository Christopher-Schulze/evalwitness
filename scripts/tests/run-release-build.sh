#!/usr/bin/env bash
# Reproduce the complete binary set twice and prove no-overwrite behavior.

set -euo pipefail
cd "$(dirname "$0")/../.."

work_root="$(mktemp -d "${TMPDIR:-/tmp}/evalwitness-release-build.XXXXXX")"
trap 'rm -rf "${work_root}"' EXIT

first="${work_root}/first"
second="${work_root}/second"

scripts/build/build.sh --destination "${first}"
scripts/build/build.sh --destination "${second}"

expected_assets="$(printf '%s\n' \
  SHA256SUMS \
  evalwitness-darwin-amd64 \
  evalwitness-darwin-arm64 \
  evalwitness-linux-amd64 \
  evalwitness-linux-arm64 \
  evalwitness-windows-amd64.exe)"

actual_assets="$(
  for asset in "${first}"/*; do
    basename "${asset}"
  done | LC_ALL=C sort
)"
if [ "${actual_assets}" != "${expected_assets}" ]; then
  echo "Release asset inventory mismatch" >&2
  diff -u <(printf '%s\n' "${expected_assets}") <(printf '%s\n' "${actual_assets}") >&2 || true
  exit 1
fi

maximum_binary_bytes=$((20 * 1024 * 1024))
for binary in "${first}"/evalwitness-*; do
  binary_bytes="$(wc -c <"${binary}" | tr -d ' ')"
  if [ "${binary_bytes}" -gt "${maximum_binary_bytes}" ]; then
    echo "Release binary exceeds 20 MiB: $(basename "${binary}") (${binary_bytes} bytes)" >&2
    exit 1
  fi
done

(cd "${first}" && shasum -a 256 -c SHA256SUMS)
(cd "${second}" && shasum -a 256 -c SHA256SUMS)
if ! cmp -s "${first}/SHA256SUMS" "${second}/SHA256SUMS"; then
  echo "Two release builds produced different checksums" >&2
  diff -u "${first}/SHA256SUMS" "${second}/SHA256SUMS" >&2 || true
  exit 1
fi

host_binary="${first}/evalwitness-$(go env GOOS)-$(go env GOARCH)"
if [ "$(go env GOOS)" = "windows" ]; then
  host_binary="${host_binary}.exe"
fi
expected_version="$(go run ./cmd/evalwitness version)"
if [ "$("${host_binary}" version)" != "${expected_version}" ]; then
  echo "Release binary does not expose the canonical product version" >&2
  exit 1
fi

before_manifest="$(shasum -a 256 "${first}/SHA256SUMS")"
if scripts/build/build.sh --destination "${first}" >/dev/null 2>&1; then
  echo "Release build overwrote an existing destination" >&2
  exit 1
fi
after_manifest="$(shasum -a 256 "${first}/SHA256SUMS")"
if [ "${before_manifest}" != "${after_manifest}" ]; then
  echo "Rejected release build mutated the existing destination" >&2
  exit 1
fi

echo "Release build conformance passed: assets=6 version=${expected_version} maximum_binary_mib=20 reproducible=true overwrite=false"
