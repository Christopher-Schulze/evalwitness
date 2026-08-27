#!/usr/bin/env bash
# Reproduce the committed TASK-070 v5 empirical capsule from a fresh clone.
#
# The clone receives only the declared non-Git trajectory fixture. Go module and
# build caches start empty; the exact module graph is supplied through a local
# file proxy. Every verification runs under an operating-system network deny.

set -euo pipefail

if [ "$#" -ne 0 ]; then
  printf 'usage: %s\n' "$0" >&2
  exit 2
fi

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
repo_root=$(cd "${script_dir}/../.." && pwd -P)
cd "${repo_root}"

if [ -n "$(git status --porcelain=v1 --untracked-files=all)" ]; then
  echo "clean-clone reproduction requires a clean source worktree" >&2
  exit 1
fi

fixture="${repo_root}/eval/trajectories/terminal_trajs/forge_gpt54"
if [ ! -d "${fixture}" ] || [ -L "${fixture}" ]; then
  echo "declared trajectory fixture is missing or linked: ${fixture}" >&2
  exit 1
fi

temporary_root=$(mktemp -d "${TMPDIR:-/tmp}/evalwitness-identical-response-v5.XXXXXX")
network_probe_pid=""
cleanup() {
	if [ -n "${network_probe_pid}" ]; then
		kill "${network_probe_pid}" >/dev/null 2>&1 || true
		wait "${network_probe_pid}" 2>/dev/null || true
	fi
	chmod -R u+w "${temporary_root}" 2>/dev/null || true
	rm -rf "${temporary_root}"
}
trap cleanup EXIT

clone_root="${temporary_root}/repo"
clone_tmp="${temporary_root}/tmp"
empty_home="${temporary_root}/home"
empty_go_cache="${temporary_root}/go-cache"
empty_module_cache="${temporary_root}/module-cache"
mkdir -p "${clone_tmp}" "${empty_home}" "${empty_go_cache}" "${empty_module_cache}"
chmod 0700 "${clone_tmp}" "${empty_home}" "${empty_go_cache}" "${empty_module_cache}"

git clone --quiet --no-local --no-hardlinks "${repo_root}" "${clone_root}"
mkdir -p "${clone_root}/eval/trajectories/terminal_trajs"
cp -R "${fixture}" "${clone_root}/eval/trajectories/terminal_trajs/"

if ! git -C "${clone_root}" diff --quiet --exit-code; then
  echo "fresh clone has a tracked-file diff before reproduction" >&2
  exit 1
fi
source_commit=$(git rev-parse HEAD)
clone_commit=$(git -C "${clone_root}" rev-parse HEAD)
if [ "${source_commit}" != "${clone_commit}" ]; then
  echo "fresh clone commit differs from source HEAD" >&2
  exit 1
fi

if command -v sandbox-exec >/dev/null 2>&1; then
  network_guard=(sandbox-exec -p '(version 1) (allow default) (deny network*)')
elif command -v unshare >/dev/null 2>&1 && unshare --user --map-root-user --net true >/dev/null 2>&1; then
  network_guard=(unshare --user --map-root-user --net)
else
  echo "hard network isolation is unavailable on this host" >&2
  exit 1
fi

host_goroot=$(go env GOROOT)
guarded() {
  "${network_guard[@]}" env -i \
    PATH="${PATH}" HOME="${empty_home}" TMPDIR="${clone_tmp}" \
    GOCACHE="${empty_go_cache}" GOMODCACHE="${empty_module_cache}" \
    GOROOT="${host_goroot}" GOENV=off GOTOOLCHAIN=local \
    GOPROXY="${module_proxy_uri:-off}" GOSUMDB=off GOFLAGS=-mod=readonly \
    EVALWITNESS_HOME="${temporary_root}/evalwitness-home" \
    EVALWITNESS_ENV_FILE="${temporary_root}/empty.env" \
    EVALWITNESS_CONFIG_FILE="${temporary_root}/empty.json" \
    EVALWITNESS_NETWORK_GUARD_ACTIVE=1 \
    HTTP_PROXY=http://127.0.0.1:1 HTTPS_PROXY=http://127.0.0.1:1 \
    ALL_PROXY=http://127.0.0.1:1 NO_PROXY= "$@"
}
: > "${temporary_root}/empty.env"
printf '{}\n' > "${temporary_root}/empty.json"

network_probe_port_file="${temporary_root}/network-probe.port"
python3 - "${network_probe_port_file}" <<'PY' &
import pathlib
import socket
import sys

path = pathlib.Path(sys.argv[1])
server = socket.socket()
server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
server.bind(("127.0.0.1", 0))
server.listen()
path.write_text(str(server.getsockname()[1]), encoding="ascii")
while True:
    connection, _ = server.accept()
    connection.close()
PY
network_probe_pid="$!"
for _ in {1..100}; do
  if [ -s "${network_probe_port_file}" ]; then
    break
  fi
  sleep 0.02
done
if [ ! -s "${network_probe_port_file}" ]; then
  echo "network-denial control server did not become ready" >&2
  exit 1
fi
network_probe_port=$(<"${network_probe_port_file}")
python3 -c 'import socket,sys; socket.create_connection(("127.0.0.1", int(sys.argv[1])), 1).close()' "${network_probe_port}"
if guarded python3 -c 'import socket,sys; socket.create_connection(("127.0.0.1", int(sys.argv[1])), 1).close()' "${network_probe_port}" >/dev/null 2>&1; then
  echo "network-denial probe unexpectedly connected" >&2
  exit 1
fi
kill "${network_probe_pid}" >/dev/null 2>&1 || true
wait "${network_probe_pid}" 2>/dev/null || true
network_probe_pid=""

# Materialize the exact module graph without network access. The clone build
# below starts with empty module/build caches and can use only this file proxy.
module_proxy="${temporary_root}/module-proxy"
(
  cd "${clone_root}"
  GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local \
    scripts/build/build-go-module-proxy.sh --destination "${module_proxy}" >/dev/null
)
module_proxy_uri=$(python3 - "${module_proxy}" <<'PY'
import pathlib
import sys

print(pathlib.Path(sys.argv[1]).resolve(strict=True).as_uri())
PY
)

clone_binary="${temporary_root}/evalwitness"
(
  cd "${clone_root}"
  guarded go build -buildvcs=false -trimpath -o "${clone_binary}" ./cmd/evalwitness
)

base_extract="${temporary_root}/base"
mkdir -p "${base_extract}"
tar -xzf "${clone_root}/eval/governance/identical-response-capsule-v5.tar.gz" -C "${base_extract}"
base_capsule=$(find "${base_extract}" -mindepth 1 -maxdepth 1 -type d -name 'evalwitness-capsule-*' -print -quit)
outer_capsule="${clone_root}/eval/governance/identical-response-capsule-v5-outer"
ledger="${clone_root}/eval/governance/identical-response-claim-ledger-v5.json"
challenge_pack="${clone_root}/eval/governance/identical-response-claim-challenge-pack-v5.json"
capture=$(find "${base_capsule}/components/sha256" -type f -name 'b314261d77ae8f027e731017d40eec3ccbfc909a1c67e011b8191addd673bd2e' -print -quit)

printf 'clean-clone: verifying outer capsule and claims\n' >&2
(
  cd "${clone_root}"
  guarded "${clone_binary}" replay study capsule verify \
    --base-capsule "${base_capsule}" --source "${outer_capsule}" \
    --claim-ledger "${ledger}" --challenge-pack "${challenge_pack}"
) > "${temporary_root}/capsule-verification.json"

printf 'clean-clone: verifying capture run and research admission\n' >&2
(
  cd "${clone_root}"
  guarded "${clone_binary}" replay capture-run verify \
    --capture "${capture}" \
    --attestation eval/governance/identical-response-capture-run-attestation-v5.json
) > "${temporary_root}/capture-run-verification.json"
(
  cd "${clone_root}"
  guarded "${clone_binary}" replay capture-run admit \
    --capture "${capture}" --authorized-calls 60
) > "${temporary_root}/research-admission.json"

printf 'clean-clone: replaying registered analysis\n' >&2
(
  cd "${clone_root}"
  guarded "${clone_binary}" replay study analyze-identical-response \
    --capture "${capture}" \
    --study-record eval/governance/identical-response-study-record-v5.json \
    --root eval/trajectories/terminal_trajs --trajs forge_gpt54 --epsilon 0.02 \
    --output "${temporary_root}/replayed-analysis.json"
) > "${temporary_root}/analysis-output.json"
cmp "${temporary_root}/replayed-analysis.json" \
  "${clone_root}/eval/governance/identical-response-offline-analysis-v5.json"

printf 'clean-clone: inspecting deterministic outer archive\n' >&2
(
  cd "${clone_root}"
  guarded "${clone_binary}" archive inspect \
    --source eval/governance/identical-response-capsule-v5-outer.tar.gz \
    --expected-root evalwitness-capsule-2ba8ac4686bd1e9f5bb1f9ce530565ed02060447494b452abff8a0850ed8461b
) > "${temporary_root}/archive-inspection.json"

python3 - "${repo_root}" "${clone_root}" "${temporary_root}" <<'PY'
import hashlib
import json
import pathlib
import subprocess
import sys

source = pathlib.Path(sys.argv[1])
clone = pathlib.Path(sys.argv[2])
run = pathlib.Path(sys.argv[3])

def digest(path: pathlib.Path) -> str:
    value = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            value.update(block)
    return value.hexdigest()

def fixture_digest(root: pathlib.Path) -> tuple[int, int, str]:
    value = hashlib.sha256()
    files = sorted(path for path in root.rglob("*") if path.is_file())
    total = 0
    for path in files:
        relative = path.relative_to(root).as_posix().encode("utf-8")
        payload = path.read_bytes()
        total += len(payload)
        value.update(len(relative).to_bytes(8, "big"))
        value.update(relative)
        value.update(len(payload).to_bytes(8, "big"))
        value.update(hashlib.sha256(payload).digest())
    return len(files), total, value.hexdigest()

registered = [
    "eval/governance/identical-response-capture-bai-flash-v5.jsonl",
    "eval/governance/identical-response-route-attestation-v5.json",
    "eval/governance/identical-response-study-manifest-v5.json",
    "eval/governance/identical-response-study-record-v5.json",
    "eval/governance/identical-response-live-authorization-v5.json",
    "eval/governance/identical-response-capture-run-attestation-v5.json",
    "eval/governance/identical-response-research-lineage-admission-v5.json",
    "eval/governance/identical-response-offline-analysis-v5.json",
    "eval/governance/identical-response-bundle-policy-v5.json",
    "eval/governance/identical-response-reviewed-findings-v5.json",
    "eval/governance/identical-response-capsule-v5.tar.gz",
    "eval/governance/identical-response-capsule-v5-outer.tar.gz",
    "eval/governance/identical-response-claim-ledger-v5.json",
    "eval/governance/identical-response-claim-challenge-pack-v5.json",
]
identical = []
for relative in registered:
    left = source / relative
    right = clone / relative
    if not left.is_file() or not right.is_file() or digest(left) != digest(right):
        raise SystemExit(f"registered v5 artifact differs: {relative}")
    identical.append({"path": relative, "sha256": digest(left), "bytes": left.stat().st_size})

verification = json.loads((run / "capsule-verification.json").read_text(encoding="utf-8"))
if not verification["offline"] or verification["provider_calls"] != 0:
    raise SystemExit("outer capsule verifier was not offline/provider-free")
if not verification["family"]["valid"] or verification["family"]["components"] != 8:
    raise SystemExit("outer capsule family did not verify")
if not verification["claims"]["valid"]:
    raise SystemExit("v5 claim ledger did not verify")
receipts = verification["challenge_pack"]["receipts"]
if len(receipts) != 34 or not all(receipt["passed"] for receipt in receipts):
    raise SystemExit("v5 challenge pack did not pass all receipts")

admission = json.loads((run / "research-admission.json").read_text(encoding="utf-8"))
expected_admission = json.loads(
    (clone / "eval/governance/identical-response-research-lineage-admission-v5.json").read_text(encoding="utf-8")
)
if (
    admission["admission"] != "admitted"
    or admission["authorized_calls"] != 60
    or admission["observed_calls"] != 60
    or admission["complete_research_entries"] != 60
    or admission["digest"] != expected_admission["digest"]
):
    raise SystemExit("v5 research admission did not reproduce")

analysis_path = clone / "eval/governance/identical-response-offline-analysis-v5.json"
analysis = json.loads(analysis_path.read_text(encoding="utf-8"))
analysis_digest = digest(analysis_path)
if analysis_digest != "d4b881379b7d19144eae820a28f13fb7b7dd694e18b185eeac5adb5e17249847":
    raise SystemExit("v5 analysis digest differs from the locked identity")

archive = json.loads((run / "archive-inspection.json").read_text(encoding="utf-8"))
archive_source = clone / "eval/governance/identical-response-capsule-v5-outer.tar.gz"
if (
    archive["files"] != 11
    or archive["directories"] != 3
    or archive["expanded_bytes"] != 399491
    or archive["sources"][0]["sha256"] != digest(archive_source)
):
    raise SystemExit("v5 outer archive inspection differs from the committed archive")

fixture_files, fixture_bytes, fixture_identity = fixture_digest(
    clone / "eval/trajectories/terminal_trajs/forge_gpt54"
)
proxy_index = json.loads((run / "module-proxy/index.json").read_text(encoding="utf-8"))
source_commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=source, text=True).strip()
clone_commit = subprocess.check_output(["git", "-C", str(clone), "rev-parse", "HEAD"], text=True).strip()
if source_commit != clone_commit:
    raise SystemExit("clean-clone report commit mismatch")

report = {
    "schema_version": "evalwitness.identical-response-reproduction-report.v1",
    "profile": "identical-response-v5-clean-clone",
    "source_commit": clone_commit,
    "clone": {
        "method": "git clone --no-local --no-hardlinks",
        "tracked_worktree_clean": True,
        "fixture": {
            "path": "eval/trajectories/terminal_trajs/forge_gpt54",
            "provisioned": True,
            "files": fixture_files,
            "bytes": fixture_bytes,
            "digest": fixture_identity,
        },
    },
    "environment": {
        "evalwitness_state": "isolated_empty",
        "home": "empty",
        "provider_environment": "empty",
        "go_build_cache": "empty",
        "go_module_cache": "empty",
        "go_module_proxy": "file:// declared exact module graph",
        "network": "denied",
        "network_probe": "passed",
        "provider_calls": 0,
    },
    "registered_artifacts": {
        "count": len(identical),
        "byte_hashes_identical": True,
        "artifacts": identical,
    },
    "capsule": {
        "capsule_id": verification["family"]["capsule_id"],
        "manifest_digest": verification["family"]["manifest_digest"],
        "registry_digest": verification["family"]["registry_digest"],
        "components": verification["family"]["components"],
        "valid": verification["family"]["valid"],
        "evidence_ceiling": "record_complete_external_parents_resolved",
    },
    "capture": {
        "payload_sha256": "b314261d77ae8f027e731017d40eec3ccbfc909a1c67e011b8191addd673bd2e",
        "authorized_calls": 60,
        "observed_calls": 60,
        "complete_research_entries": 60,
        "capture_run_verified": True,
        "research_admission": "admitted",
    },
    "analysis": {
        "sha256": analysis_digest,
        "byte_identical": True,
        "task_groups": analysis["summary"]["task_groups"],
        "complete": analysis["summary"]["complete"],
        "agreements": analysis["summary"]["agreements"],
        "disagreements": analysis["summary"]["disagreements"],
        "unresolved": analysis["summary"]["unresolved"],
    },
    "claims": {
        "ledger_digest": verification["claims"]["ledger_digest"],
        "valid": verification["claims"]["valid"],
        "status_counts": verification["claims"]["status_counts"],
    },
    "challenges": {
        "pack_digest": verification["challenge_pack"]["digest"],
        "receipts": len(receipts),
        "all_passed": True,
    },
    "module_proxy": {"modules": proxy_index["module_count"], "files": proxy_index["file_count"]},
    "clean_clone_proof": True,
    "status": "passed",
    "limitations": [
        "The fixture is provisioned from the declared local non-Git asset and is not a provider response or independent replication.",
        "The empirical result remains descriptive for the locked development population and makes no correctness, quality, transfer, or release claim.",
    ],
}
print(json.dumps(report, sort_keys=True, separators=(",", ":")))
PY
