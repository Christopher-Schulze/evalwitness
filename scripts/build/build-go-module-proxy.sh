#!/usr/bin/env bash
# Materialize the exact Go module graph as a no-overwrite file:// proxy.

set -euo pipefail
cd "$(dirname "$0")/../.."

usage() {
  echo "Usage: scripts/build/build-go-module-proxy.sh --destination NEW_DIRECTORY" >&2
}

if [ "$#" -ne 2 ] || [ "$1" != "--destination" ] || [ -z "$2" ]; then
  usage
  exit 2
fi

destination="$2"
destination_parent="$(dirname "${destination}")"
if [ ! -d "${destination_parent}" ]; then
  echo "Go module proxy parent does not exist: ${destination_parent}" >&2
  exit 1
fi
if [ -e "${destination}" ] || [ -L "${destination}" ]; then
  echo "Go module proxy destination already exists: ${destination}" >&2
  exit 1
fi

module_graph_stage="$(mktemp -d "${destination_parent}/.evalwitness-module-graph.XXXXXX")"
download_inventory="${module_graph_stage}/downloads.json"
cp go.mod "${module_graph_stage}/graph.mod"
cp go.sum "${module_graph_stage}/graph.sum"
proxy_created=false
cleanup() {
	rm -rf "${module_graph_stage}"
  if [ "${proxy_created}" != true ] && [ -d "${destination}" ]; then
    rm -rf "${destination}"
  fi
}
trap cleanup EXIT

go mod download -modfile="${module_graph_stage}/graph.mod" -json all > "${download_inventory}"
module_cache_download="$(go env GOMODCACHE)/cache/download"
mkdir "${destination}"

python3 - "${download_inventory}" "${module_cache_download}" "${destination}" <<'PY'
import hashlib
import json
import os
import pathlib
import shutil
import sys

inventory_path = pathlib.Path(sys.argv[1])
cache_root = pathlib.Path(sys.argv[2]).resolve(strict=True)
destination = pathlib.Path(sys.argv[3]).resolve(strict=True)
raw = inventory_path.read_text(encoding="utf-8")
decoder = json.JSONDecoder()
offset = 0
downloads = []
while offset < len(raw):
    while offset < len(raw) and raw[offset].isspace():
        offset += 1
    if offset == len(raw):
        break
    value, offset = decoder.raw_decode(raw, offset)
    if not isinstance(value, dict):
        raise SystemExit("Go module download inventory contains a non-object")
    downloads.append(value)

if not downloads:
    raise SystemExit("Go module download inventory is empty")

seen = set()
modules = []
file_records = []
versions_by_directory = {}
for download in sorted(downloads, key=lambda item: (item.get("Path", ""), item.get("Version", ""))):
    required = ("Path", "Version", "Info", "GoMod", "Zip", "Sum", "GoModSum")
    if any(not isinstance(download.get(field), str) or not download[field] for field in required):
        raise SystemExit("Go module download inventory is incomplete")
    identity = (download["Path"], download["Version"])
    if identity in seen:
        raise SystemExit(f"duplicate Go module download: {identity[0]}@{identity[1]}")
    seen.add(identity)
    module_files = []
    version_directory = None
    for field, suffix in (("Info", ".info"), ("GoMod", ".mod"), ("Zip", ".zip")):
        source = pathlib.Path(download[field]).resolve(strict=True)
        try:
            relative = source.relative_to(cache_root)
        except ValueError as error:
            raise SystemExit(f"Go module cache path escapes the download root: {source}") from error
        if source.name != download["Version"] + suffix or source.stat().st_size < 1:
            raise SystemExit(f"Go module cache file has an invalid identity: {source}")
        if version_directory is None:
            version_directory = relative.parent
        elif relative.parent != version_directory:
            raise SystemExit("Go module cache files disagree on their version directory")
        target = destination / relative
        target.parent.mkdir(parents=True, exist_ok=True, mode=0o755)
        if target.exists() or target.is_symlink():
            raise SystemExit(f"Go module proxy target already exists: {target}")
        shutil.copyfile(source, target)
        os.chmod(target, 0o644)
        digest = hashlib.sha256(target.read_bytes()).hexdigest()
        record = {"path": relative.as_posix(), "bytes": target.stat().st_size, "sha256": digest}
        file_records.append(record)
        module_files.append(record["path"])
    versions_by_directory.setdefault(version_directory, []).append(download["Version"])
    modules.append({
        "path": download["Path"],
        "version": download["Version"],
        "sum": download["Sum"],
        "go_mod_sum": download["GoModSum"],
        "files": module_files,
    })

for version_directory, versions in sorted(versions_by_directory.items(), key=lambda item: item[0].as_posix()):
    relative = version_directory / "list"
    target = destination / relative
    if target.exists() or target.is_symlink():
        raise SystemExit(f"Go module proxy list target already exists: {target}")
    target.write_text("".join(version + "\n" for version in sorted(versions)), encoding="utf-8")
    os.chmod(target, 0o644)
    file_records.append({
        "path": relative.as_posix(),
        "bytes": target.stat().st_size,
        "sha256": hashlib.sha256(target.read_bytes()).hexdigest(),
    })

file_records.sort(key=lambda item: item["path"])
index = {
    "schema_version": "evalwitness.go-module-proxy.v1",
    "module_count": len(modules),
    "file_count": len(file_records),
    "modules": modules,
    "files": file_records,
}
index_path = destination / "index.json"
index_path.write_text(json.dumps(index, sort_keys=True, separators=(",", ":")), encoding="utf-8")
os.chmod(index_path, 0o644)
PY

proxy_created=true
echo "Go module proxy built: destination=${destination} modules=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["module_count"])' "${destination}/index.json")"
