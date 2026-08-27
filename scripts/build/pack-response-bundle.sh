#!/usr/bin/env bash
# Seal and publish a public exact-response capsule from schema-3 captures.

set -euo pipefail

repository_root="$(cd "$(dirname "$0")/../.." && pwd -P)"
policy_draft=""
destination=""
archive=""
producer_binary="${EVALWITNESS_BIN:-}"
redistribution_evidence=""
reviewed_findings=""
captures=()

usage() {
  echo "usage: $0 --policy-draft PATH --redistribution-evidence PATH --destination PATH --capture name=exact.jsonl [--capture ...] [--archive PATH] [--producer-binary PATH] [--repository-root PATH] [--reviewed-findings PATH]" >&2
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --policy-draft|--redistribution-evidence|--destination|--archive|--producer-binary|--repository-root|--capture|--reviewed-findings)
      if [ "$#" -lt 2 ] || [ -z "$2" ]; then
        usage
        exit 2
      fi
      case "$1" in
        --policy-draft) policy_draft="$2" ;;
        --redistribution-evidence) redistribution_evidence="$2" ;;
        --destination) destination="$2" ;;
        --archive) archive="$2" ;;
        --producer-binary) producer_binary="$2" ;;
        --repository-root) repository_root="$2" ;;
        --capture) captures+=("$2") ;;
        --reviewed-findings) reviewed_findings="$2" ;;
      esac
      shift 2
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done

if [ -z "${policy_draft}" ] || [ -z "${redistribution_evidence}" ] || [ -z "${destination}" ] || [ "${#captures[@]}" -eq 0 ]; then
  usage
  exit 2
fi

temporary_root="$(mktemp -d "${TMPDIR:-/tmp}/evalwitness-response-bundle.XXXXXX")"
temporary_binary=""
cleanup() {
  rm -rf "${temporary_root}"
}
trap cleanup EXIT

if [ -z "${producer_binary}" ]; then
  temporary_binary="${temporary_root}/evalwitness"
  (cd "${repository_root}" && go build -o "${temporary_binary}" ./cmd/evalwitness)
  producer_binary="${temporary_binary}"
elif [[ "${producer_binary}" != */* ]]; then
  producer_binary="$(command -v "${producer_binary}")"
elif [[ "${producer_binary}" != /* ]]; then
  producer_binary="$(cd "$(dirname "${producer_binary}")" && pwd -P)/$(basename "${producer_binary}")"
fi

if [ ! -f "${producer_binary}" ] || [ ! -x "${producer_binary}" ]; then
  echo "response bundle producer is not an executable regular file: ${producer_binary}" >&2
  exit 2
fi

sealed_policy="${temporary_root}/policy.json"
seal_arguments=(
  replay bundle seal-policy
  --source "${policy_draft}"
  --repository-root "${repository_root}"
  --producer-binary "${producer_binary}"
  --redistribution-evidence "${redistribution_evidence}"
)
build_arguments=(
  replay bundle build
  --policy "${sealed_policy}"
  --repository-root "${repository_root}"
  --producer-binary "${producer_binary}"
  --redistribution-evidence "${redistribution_evidence}"
  --destination "${destination}"
)
for capture in "${captures[@]}"; do
  seal_arguments+=(--capture "${capture}")
  build_arguments+=(--capture "${capture}")
done
if [ -n "${archive}" ]; then
  build_arguments+=(--archive "${archive}")
fi
if [ -n "${reviewed_findings}" ]; then
  build_arguments+=(--reviewed-findings "${reviewed_findings}")
fi

"${producer_binary}" "${seal_arguments[@]}" > "${sealed_policy}"
"${producer_binary}" "${build_arguments[@]}"
