#!/usr/bin/env bash
# List only module-owned Go packages that are eligible for repository gates.

set -euo pipefail
cd "$(dirname "$0")/../.."

if [ "$#" -ne 0 ]; then
  echo "usage: scripts/tests/list-project-go-packages.sh" >&2
  exit 2
fi

module_path="$(go list -m -f '{{.Path}}')"
raw_packages="$(go list ./...)"
project_packages=()
excluded_dependencies=0

while IFS= read -r package; do
  if [ -z "${package}" ]; then
    continue
  fi
  case "${package}" in
    "${module_path}"|"${module_path}/"*) ;;
    *)
      echo "Go package inventory escaped module ${module_path}: ${package}" >&2
      exit 1
      ;;
  esac
  case "${package}" in
    */node_modules/*)
      excluded_dependencies=$((excluded_dependencies + 1))
      continue
      ;;
  esac
  for existing in "${project_packages[@]}"; do
    if [ "${existing}" = "${package}" ]; then
      echo "Go package inventory contains duplicate ${package}" >&2
      exit 1
    fi
  done
  project_packages+=("${package}")
done <<<"${raw_packages}"

if [ "${#project_packages[@]}" -eq 0 ]; then
  echo "Go package inventory is empty" >&2
  exit 1
fi

printf '%s\n' "${project_packages[@]}"
printf 'Go package inventory: module=%s project_packages=%d excluded_dependencies=%d\n' \
  "${module_path}" "${#project_packages[@]}" "${excluded_dependencies}" >&2
