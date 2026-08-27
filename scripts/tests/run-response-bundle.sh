#!/usr/bin/env bash
# Hard-network-offline conformance gate for public exact-response bundles.

set -euo pipefail
cd "$(dirname "$0")/../.."

temporary_root="$(mktemp -d "${TMPDIR:-/tmp}/evalwitness-response-bundle-test.XXXXXX")"
temporary_binary=""
network_probe_pid=""
cleanup() {
  if [ -n "${network_probe_pid}" ]; then
    kill "${network_probe_pid}" >/dev/null 2>&1 || true
    wait "${network_probe_pid}" 2>/dev/null || true
  fi
  rm -rf "${temporary_root}"
}
trap cleanup EXIT

provided_evalwitness_binary="${EVALWITNESS_BIN:-}"
evalwitness_binary=""

isolated_home="${temporary_root}/home"
isolated_tmp="${temporary_root}/tmp"
mkdir -p "${isolated_home}" "${isolated_tmp}"
network_guard=()
network_guard_inherited="${EVALWITNESS_NETWORK_GUARD_ACTIVE:-0}"
if [ "${network_guard_inherited}" = "1" ]; then
  # Bash 3.2 with nounset rejects expansion of a declared empty array.
  network_guard=(/usr/bin/env)
elif command -v sandbox-exec > /dev/null 2>&1; then
  network_guard=(sandbox-exec -p '(version 1) (allow default) (deny network*)')
elif command -v unshare > /dev/null 2>&1 && unshare --user --map-root-user --net true > /dev/null 2>&1; then
  network_guard=(unshare --user --map-root-user --net)
else
  echo "hard network isolation is unavailable on this host" >&2
  exit 1
fi
go_module_cache="$(go env GOMODCACHE)"
go_build_cache="$(go env GOCACHE)"
go_root="$(go env GOROOT)"

guarded() {
  "${network_guard[@]}" env -i \
    PATH="${PATH}" HOME="${isolated_home}" TMPDIR="${isolated_tmp}" \
    GOMODCACHE="${go_module_cache}" GOCACHE="${go_build_cache}" GOROOT="${go_root}" GOENV=off \
    HTTP_PROXY="http://127.0.0.1:1" HTTPS_PROXY="http://127.0.0.1:1" ALL_PROXY="http://127.0.0.1:1" NO_PROXY="" \
    EVALWITNESS_NETWORK_GUARD_ACTIVE=1 \
    "$@"
}

if [ "${network_guard_inherited}" != "1" ]; then
  network_probe_port_file="${temporary_root}/network-probe.port"
  python3 - "${network_probe_port_file}" <<'PY' &
import pathlib
import socket
import sys

server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
server.bind(("127.0.0.1", 0))
server.listen()
pathlib.Path(sys.argv[1]).write_text(str(server.getsockname()[1]), encoding="ascii")
while True:
    connection, _ = server.accept()
    connection.close()
PY
  network_probe_pid="$!"
  for _ in {1..100}; do
    if [ -s "${network_probe_port_file}" ]; then
      break
    fi
    if ! kill -0 "${network_probe_pid}" 2>/dev/null; then
      echo "network-denial control server exited before readiness" >&2
      exit 1
    fi
    sleep 0.02
  done
  if [ ! -s "${network_probe_port_file}" ]; then
    echo "network-denial control server did not become ready" >&2
    exit 1
  fi
  network_probe_port="$(<"${network_probe_port_file}")"
  python3 -c 'import socket,sys; socket.create_connection(("127.0.0.1", int(sys.argv[1])), 1).close()' "${network_probe_port}"
  if guarded python3 -c 'import socket,sys; socket.create_connection(("127.0.0.1", int(sys.argv[1])), 1).close()' "${network_probe_port}" >/dev/null 2>&1; then
    echo "network-denial probe unexpectedly connected" >&2
    exit 1
  fi
  kill "${network_probe_pid}"
  wait "${network_probe_pid}" 2>/dev/null || true
  network_probe_pid=""
fi

if [ -n "${provided_evalwitness_binary}" ]; then
  evalwitness_binary="${provided_evalwitness_binary}"
  if [[ "${evalwitness_binary}" != */* ]]; then
    if ! evalwitness_binary="$(command -v "${evalwitness_binary}")"; then
      echo "response bundle test binary is not available: ${provided_evalwitness_binary}" >&2
      exit 2
    fi
  elif [[ "${evalwitness_binary}" != /* ]]; then
    evalwitness_binary="$(cd "$(dirname "${evalwitness_binary}")" && pwd -P)/$(basename "${evalwitness_binary}")"
  fi
else
  temporary_binary="${temporary_root}/evalwitness"
  guarded go build -o "${temporary_binary}" ./cmd/evalwitness
  evalwitness_binary="${temporary_binary}"
fi
if [ ! -f "${evalwitness_binary}" ] || [ ! -x "${evalwitness_binary}" ]; then
  echo "response bundle test binary is not an executable regular file: ${evalwitness_binary}" >&2
  exit 2
fi

policy_draft="scripts/tests/response-bundle-policy-draft.json"
capture="golden-delta=scripts/tests/golden-delta-replay.jsonl"
guarded scripts/build/pack-response-bundle.sh \
  --policy-draft "${policy_draft}" \
  --redistribution-evidence LICENSE \
  --producer-binary "${evalwitness_binary}" \
  --destination "${temporary_root}/bundle-a" \
  --archive "${temporary_root}/bundle-a.tar.gz" \
  --capture "${capture}" > "${temporary_root}/build-a.json"
guarded scripts/build/pack-response-bundle.sh \
  --policy-draft "${policy_draft}" \
  --redistribution-evidence LICENSE \
  --producer-binary "${evalwitness_binary}" \
  --destination "${temporary_root}/bundle-b" \
  --archive "${temporary_root}/bundle-b.tar.gz" \
  --capture "${capture}" > "${temporary_root}/build-b.json"
cmp "${temporary_root}/bundle-a.tar.gz" "${temporary_root}/bundle-b.tar.gz"

guarded "${evalwitness_binary}" replay bundle verify \
  --source "${temporary_root}/bundle-a" > "${temporary_root}/verify-a.json"
guarded "${evalwitness_binary}" replay bundle verify \
  --source "${temporary_root}/bundle-b" > "${temporary_root}/verify-b.json"

capture_path="$(guarded python3 - "${temporary_root}/build-a.json" "${temporary_root}/build-b.json" "${temporary_root}/verify-a.json" "${temporary_root}/verify-b.json" <<'PY'
import json
import pathlib
import sys

build_a, build_b, verify_a, verify_b = [json.loads(pathlib.Path(path).read_text()) for path in sys.argv[1:]]
if build_a["capsule_id"] != build_b["capsule_id"] or build_a["archive"]["sha256"] != build_b["archive"]["sha256"]:
    raise SystemExit("identical exact captures produced different bundle identities")
for report in (build_a, build_b):
    if report["provider_calls"] != 0 or report["network_required"]:
        raise SystemExit("response bundle build crossed its offline provider boundary")
for report in (verify_a, verify_b):
    if not report["valid"] or not report["exact_replay"] or report["provider_calls"] != 0 or report["network_required"]:
        raise SystemExit("response bundle verification crossed its exact offline boundary")
    if report["lineage_policy"] != "exact_fixture":
        raise SystemExit("synthetic response bundle lost its explicit fixture-only lineage boundary")
    if report["evidence_ceiling"] != "mechanism_conformance":
        raise SystemExit("synthetic response bundle exceeded its mechanism-conformance evidence ceiling")
    if report["redistribution_evidence_sha256"] != "a241c1fcf73642ed28ebb55d1b6281efff38f3892fe36ef184dbdebb8101191d":
        raise SystemExit("response bundle did not embed the sealed redistribution evidence")
    if report["total_entries"] != 1 or len(report["captures"]) != 1:
        raise SystemExit("response bundle capture census drifted")
    if report["study_id"] != "synthetic-response-bundle-conformance" or report["cell_id"] != "golden-delta-exact-replay":
        raise SystemExit("response bundle study or cell identity drifted")
    if report["captures"][0]["complete_research_entries"] != 0:
        raise SystemExit("synthetic fixture was promoted to complete research evidence")
    if report["captures"][0]["payload_sha256"] != "c0accb36153ae04f47a6b24d43a846e0ebdf62d8fd1fb5a126cf02c0a9acb1ce":
        raise SystemExit("golden exact-response bytes drifted")
if (verify_a["policy_digest"], verify_a["index_digest"], verify_a["capture_set_digest"], verify_a["request_corpus_digest"]) != (verify_b["policy_digest"], verify_b["index_digest"], verify_b["capture_set_digest"], verify_b["request_corpus_digest"]):
    raise SystemExit("identical response bundles verified with different policy, index, capture-set, or request-corpus identities")
print(verify_a["captures"][0]["payload_path"])
PY
)"

empty_env="${temporary_root}/empty.env"
empty_config="${temporary_root}/empty.json"
: > "${empty_env}"
printf '{}\n' > "${empty_config}"
guarded env \
  EVALWITNESS_ENV_FILE="${empty_env}" \
  EVALWITNESS_CONFIG_FILE="${empty_config}" \
  EVALWITNESS_THINKING_MODE=disabled \
  EVALWITNESS_REPLAY_FROM="${temporary_root}/bundle-a/${capture_path}" \
  EVALWITNESS_CACHE_DIR="${temporary_root}/empty-cache" \
  "${evalwitness_binary}" verify \
    --provider replay \
    --model golden-delta \
    --base-url https://replay.invalid/v1 \
    --mode delta \
    --task @scripts/tests/sample-task.txt \
    --trajectory @scripts/tests/sample-traj-a.txt \
    --trajectory @scripts/tests/sample-traj-b.txt \
    --criteria generic \
    --n-reps 1 \
    --no-bias-mit \
    --no-cache \
    --output json > "${temporary_root}/replay.json"
rg -q '"winner": "A"' "${temporary_root}/replay.json"
rg -q '"conditional_score_a": 0.997' "${temporary_root}/replay.json"
if [ -e "${temporary_root}/empty-cache" ]; then
  echo "response bundle replay wrote to an empty cache boundary" >&2
  exit 1
fi

if guarded scripts/build/pack-response-bundle.sh \
  --policy-draft "${policy_draft}" \
  --redistribution-evidence LICENSE \
  --producer-binary "${evalwitness_binary}" \
  --destination "${temporary_root}/legacy-bundle" \
  --capture golden-delta=scripts/tests/golden-delta-replay.legacy.jsonl \
  > "${temporary_root}/legacy.stdout" 2> "${temporary_root}/legacy.stderr"; then
  echo "response bundle accepted a legacy schema-1 capture" >&2
  exit 1
fi
rg -q 'legacy or unsupported schema' "${temporary_root}/legacy.stderr"

guarded cp -R "${temporary_root}/bundle-a" "${temporary_root}/corrupted-bundle"
guarded truncate -s 1 "${temporary_root}/corrupted-bundle/${capture_path}"
if guarded "${evalwitness_binary}" replay bundle verify --source "${temporary_root}/corrupted-bundle" \
  > "${temporary_root}/corrupted.stdout" 2> "${temporary_root}/corrupted.stderr"; then
  echo "response bundle verifier accepted byte corruption" >&2
  exit 1
fi

printf '{"schema_version":"evalwitness.response-bundle-conformance.v1","network":"denied","provider_calls":0,"status":"passed"}\n'
