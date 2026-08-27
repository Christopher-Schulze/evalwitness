#!/usr/bin/env bash
# Cross-platform release build with no-overwrite publication.
# Output: DESTINATION/evalwitness-{os}-{arch}[.exe]

set -euo pipefail
cd "$(dirname "$0")/../.."

usage() {
  echo "Usage: scripts/build/build.sh [--destination NEW_DIRECTORY]" >&2
}

destination="dist"
if [ "$#" -gt 0 ]; then
  if [ "$#" -ne 2 ] || [ "$1" != "--destination" ] || [ -z "$2" ]; then
    usage
    exit 2
  fi
  destination="$2"
fi

# Keep the stripped cross-platform release artifacts under the 20 MiB contract
# without changing development or test builds. Standard-library and external
# dependency inlining remain enabled.
release_gcflags=(-gcflags='github.com/Christopher-Schulze/evalwitness/...=-l')

destination_parent="$(dirname "${destination}")"
destination_name="$(basename "${destination}")"
if [ "${destination_name}" = "." ] || [ "${destination_name}" = ".." ] || [ "${destination_name}" = "/" ]; then
  echo "Refusing unsafe release destination: ${destination}" >&2
  exit 1
fi
if [ ! -d "${destination_parent}" ]; then
  echo "Release destination parent does not exist: ${destination_parent}" >&2
  exit 1
fi
if [ -e "${destination}" ] || [ -L "${destination}" ]; then
  echo "Release destination already exists: ${destination}" >&2
  exit 1
fi
mkdir "${destination}"

build() {
  local os="$1"
  local arch="$2"
  local name="evalwitness-${os}-${arch}"
  if [ "${os}" = "windows" ]; then
    name="${name}.exe"
  fi
  echo "==> ${name}"
  CGO_ENABLED=0 GOOS="${os}" GOARCH="${arch}" \
    go build -buildvcs=false -ldflags="-s -w -buildid= -funcalign=8" -trimpath "${release_gcflags[@]}" -o "${destination}/${name}" ./cmd/evalwitness
}

for os in darwin linux windows; do
  for arch in amd64 arm64; do
    if [ "${os}" = "windows" ] && [ "${arch}" = "arm64" ]; then
      continue
    fi
    build "${os}" "${arch}"
  done
done

echo
echo "Built artifacts:"
ls -lh "${destination}/"

echo
echo "Checksums:"
(cd "${destination}" && shasum -a 256 evalwitness-* > SHA256SUMS)
cat "${destination}/SHA256SUMS"
