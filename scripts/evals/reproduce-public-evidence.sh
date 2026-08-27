#!/usr/bin/env bash
# Reproduce public development evidence with isolated EvalWitness state and a
# hard network-denial boundary. Provisioned host Go caches remain build inputs.

set -euo pipefail
cd "$(dirname "$0")/../.."

profile="full"
if [ "${1:-}" = "--profile" ] && [ "$#" -eq 2 ]; then
  profile="$2"
elif [ "$#" -ne 0 ]; then
  echo "usage: $0 [--profile full|response-bundle]" >&2
  exit 2
fi
if [ "${profile}" != "full" ] && [ "${profile}" != "response-bundle" ]; then
  echo "unsupported reproduction profile: ${profile}" >&2
  exit 2
fi

temporary_root="$(mktemp -d "${TMPDIR:-/tmp}/evalwitness-reproduction.XXXXXX")"
network_probe_pid=""
cleanup_command=(rm -rf --)
cleanup() {
  if [ -n "${network_probe_pid}" ]; then
    kill "${network_probe_pid}" >/dev/null 2>&1 || true
    wait "${network_probe_pid}" 2>/dev/null || true
  fi
  "${cleanup_command[@]}" "${temporary_root}"
}
trap cleanup EXIT

provided_evalwitness_binary="${EVALWITNESS_BIN:-}"
evalwitness_binary=""

network_guard=()
if command -v sandbox-exec > /dev/null 2>&1; then
  network_guard=(sandbox-exec -p '(version 1) (allow default) (deny network*)')
elif command -v unshare > /dev/null 2>&1; then
  if unshare --user --map-root-user --net true > /dev/null 2>&1; then
    network_guard=(unshare --user --map-root-user --net)
  elif command -v sudo > /dev/null 2>&1 && sudo -n unshare --net true > /dev/null 2>&1; then
    # Some hosted Linux runners deny unprivileged network namespaces but allow
    # the same isolated namespace through their passwordless CI root.
    network_guard=(sudo -n unshare --net)
    cleanup_command=(sudo -n rm -rf --)
  else
    echo "hard network isolation is unavailable on this host" >&2
    exit 1
  fi
else
  echo "hard network isolation is unavailable on this host" >&2
  exit 1
fi

isolated_home="${temporary_root}/home"
isolated_tmp="${temporary_root}/tmp"
mkdir -p "${isolated_home}" "${isolated_tmp}"
empty_env="${temporary_root}/empty.env"
empty_config="${temporary_root}/empty.json"
: > "${empty_env}"
printf '{}\n' > "${empty_config}"
go_module_cache="$(go env GOMODCACHE)"
go_build_cache="$(go env GOCACHE)"
go_root="$(go env GOROOT)"

guarded() {
  "${network_guard[@]}" env -i \
    PATH="${PATH}" HOME="${isolated_home}" TMPDIR="${isolated_tmp}" \
    GOMODCACHE="${go_module_cache}" GOCACHE="${go_build_cache}" GOROOT="${go_root}" GOENV=off \
    HTTP_PROXY="http://127.0.0.1:1" HTTPS_PROXY="http://127.0.0.1:1" ALL_PROXY="http://127.0.0.1:1" NO_PROXY="" \
    EVALWITNESS_ENV_FILE="${empty_env}" EVALWITNESS_CONFIG_FILE="${empty_config}" \
    EVALWITNESS_BIN="${evalwitness_binary}" EVALWITNESS_NETWORK_GUARD_ACTIVE=1 \
    "$@"
}

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

if [ -n "${provided_evalwitness_binary}" ]; then
  evalwitness_binary="${provided_evalwitness_binary}"
  if [[ "${evalwitness_binary}" != */* ]]; then
    if ! evalwitness_binary="$(command -v "${evalwitness_binary}")"; then
      echo "reproduction binary is not available: ${provided_evalwitness_binary}" >&2
      exit 2
    fi
  elif [[ "${evalwitness_binary}" != /* ]]; then
    evalwitness_binary="$(cd "$(dirname "${evalwitness_binary}")" && pwd -P)/$(basename "${evalwitness_binary}")"
  fi
else
  evalwitness_binary="${temporary_root}/evalwitness"
  guarded go build -o "${evalwitness_binary}" ./cmd/evalwitness
fi
if [ ! -f "${evalwitness_binary}" ] || [ ! -x "${evalwitness_binary}" ]; then
  echo "reproduction binary is not an executable regular file: ${evalwitness_binary}" >&2
  exit 2
fi

guarded scripts/tests/run-response-bundle.sh
if [ "${profile}" = "full" ]; then
  guarded scripts/tests/run-claimcheck.sh
fi

printf '{"schema_version":"evalwitness.reproduction-report.v1","profile":"%s","evidence_scope":"local_mechanism_conformance","evalwitness_state":"isolated_empty","go_dependency_caches":"host_provisioned","clean_clone_proof":false,"network":"denied","provider_calls":0,"status":"passed"}\n' "${profile}"
