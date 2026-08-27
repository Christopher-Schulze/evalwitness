#!/usr/bin/env bash
# Reject unexplained references to the pre-EvalWitness identity.

set -euo pipefail
cd "$(dirname "$0")/../.."

allowlist="config/identity-residue-allowlist.tsv"
violations="$(mktemp)"
trap 'rm -f "${violations}"' EXIT

if [ ! -f "${allowlist}" ]; then
  echo "error: identity residue allowlist is missing" >&2
  exit 1
fi

is_allowed() {
  local file="$1"
  local text="$2"
  local path_regex content_regex owner removal
  while IFS=$'\t' read -r path_regex content_regex owner removal; do
    if [ -z "${path_regex}" ] || [[ "${path_regex}" == \#* ]]; then
      continue
    fi
    if [ -z "${owner}" ] || [ -z "${removal}" ]; then
      echo "error: incomplete identity allowlist row for ${path_regex}" >&2
      exit 2
    fi
    if [[ "${file}" =~ ${path_regex} ]] && [[ "${text}" =~ ${content_regex} ]]; then
      return 0
    fi
  done < "${allowlist}"
  return 1
}

shopt -s nocasematch
while IFS= read -r -d '' file; do
  case "${file}" in
    "${allowlist}"|scripts/tests/run-name-residue.sh)
      continue
      ;;
  esac
  while IFS= read -r match; do
    [ -n "${match}" ] || continue
    line="${match%%:*}"
    text="${match#*:}"
    if ! is_allowed "${file}" "${text}"; then
      printf '%s:%s:%s\n' "${file}" "${line}" "${text}" >> "${violations}"
    fi
  done < <(LC_ALL=C grep -nEi 'logprobe' "${file}" 2>/dev/null || true)
done < <(git ls-files --cached --others --exclude-standard -z)

if [ -s "${violations}" ]; then
  echo "Unowned pre-EvalWitness identity residue:" >&2
  cat "${violations}" >&2
  exit 1
fi

echo "Identity residue gate passed."
