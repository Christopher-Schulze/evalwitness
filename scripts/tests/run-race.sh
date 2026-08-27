#!/usr/bin/env bash
# Run every project Go test under the race detector while isolating expensive
# evidence packages and verification-lineage tests into deterministic shards.

set -euo pipefail
cd "$(dirname "$0")/../.."

light_race_timeout="20m"
command_race_timeout="30m"
stress_race_timeout="30m"
lineage_race_timeout="20m"
module_path="$(go list -m -f '{{.Path}}')"
command_package="${module_path}/cmd/evalwitness"
command_shards=10
command_found=0
stress_package="${module_path}/internal/stress"
stress_shards=4
stress_found=0
lineage_package="github.com/Christopher-Schulze/evalwitness/internal/lineage"
lineage_shards=4
lineage_found=0
heavy_packages=(
  "${module_path}/internal/capsule"
  "${module_path}/internal/claim"
  "${module_path}/internal/explorer"
  "${module_path}/internal/reliance"
)
heavy_race_timeouts=("30m" "30m" "30m" "45m")
heavy_found=()
for ((index = 0; index < ${#heavy_packages[@]}; index++)); do
  heavy_found+=(0)
done
light_packages=()

project_package_output="$(scripts/tests/list-project-go-packages.sh)"
project_packages=()
while IFS= read -r package; do
  if [ -n "${package}" ]; then
    project_packages+=("${package}")
  fi
done <<<"${project_package_output}"

for package in "${project_packages[@]}"; do
  if [ "${package}" = "${command_package}" ]; then
    command_found=$((command_found + 1))
    continue
  fi
  if [ "${package}" = "${stress_package}" ]; then
    stress_found=$((stress_found + 1))
    continue
  fi
  if [ "${package}" = "${lineage_package}" ]; then
    lineage_found=$((lineage_found + 1))
    continue
  fi
  matched_heavy=0
  for ((index = 0; index < ${#heavy_packages[@]}; index++)); do
    if [ "${package}" = "${heavy_packages[index]}" ]; then
      heavy_found[index]=$((heavy_found[index] + 1))
      matched_heavy=1
      break
    fi
  done
  if [ "${matched_heavy}" -eq 0 ]; then
    light_packages+=("${package}")
  fi
done

if [ "${command_found}" -ne 1 ] || [ "${stress_found}" -ne 1 ] || [ "${lineage_found}" -ne 1 ] || [ "${#light_packages[@]}" -eq 0 ]; then
  echo "race gate: package inventory is incomplete" >&2
  exit 1
fi
if [ "${#heavy_race_timeouts[@]}" -ne "${#heavy_packages[@]}" ]; then
  echo "race gate: every heavy package must have exactly one timeout" >&2
  exit 1
fi
for ((index = 0; index < ${#heavy_packages[@]}; index++)); do
  if [ "${heavy_found[index]}" -ne 1 ]; then
    echo "race gate: heavy package ${heavy_packages[index]} occurs ${heavy_found[index]} times" >&2
    exit 1
  fi
done
assigned_packages=$((${#light_packages[@]} + ${#heavy_packages[@]} + command_found + stress_found + lineage_found))
if [ "${assigned_packages}" -ne "${#project_packages[@]}" ]; then
  echo "race gate: package shard assignment is incomplete" >&2
  exit 1
fi

printf 'Project race inventory: packages=%d light=%d command=%d stress=%d heavy=%d lineage=%d\n' \
  "${#project_packages[@]}" "${#light_packages[@]}" "${command_found}" "${stress_found}" \
  "${#heavy_packages[@]}" "${lineage_found}"

echo "==> race: light packages (${#light_packages[@]})"
go test -race -count=1 -timeout "${light_race_timeout}" "${light_packages[@]}"

command_tests=()
while IFS= read -r test_name; do
  case "${test_name}" in
    Test*|Example*|Fuzz*)
      command_tests+=("${test_name}")
      ;;
  esac
done < <(go test "${command_package}" -list '^(Test|Example|Fuzz)')

if [ "${#command_tests[@]}" -eq 0 ]; then
  echo "race gate: no command tests discovered" >&2
  exit 1
fi

for ((left = 0; left < ${#command_tests[@]}; left++)); do
  for ((right = 0; right < left; right++)); do
    if [ "${command_tests[left]}" = "${command_tests[right]}" ]; then
      echo "race gate: duplicate command test ${command_tests[left]}" >&2
      exit 1
    fi
  done
done

run_command_shard() {
  local shard_index="$1"
  local pattern="^("
  local separator=""
  local selected=0
  local index

  for ((index = shard_index; index < ${#command_tests[@]}; index += command_shards)); do
    pattern="${pattern}${separator}${command_tests[index]}"
    separator="|"
    selected=$((selected + 1))
  done
  pattern="${pattern})$"

  printf '==> race: command shard %d/%d (%d tests)\n' \
    "$((shard_index + 1))" "${command_shards}" "${selected}"
  go test -race -count=1 -timeout "${command_race_timeout}" "${command_package}" -run "${pattern}"
}

assigned_command_tests=0
for ((shard = 0; shard < command_shards; shard++)); do
  for ((index = shard; index < ${#command_tests[@]}; index += command_shards)); do
    assigned_command_tests=$((assigned_command_tests + 1))
  done
done
if [ "${assigned_command_tests}" -ne "${#command_tests[@]}" ]; then
  echo "race gate: command shard assignment is incomplete" >&2
  exit 1
fi

printf 'Command race inventory: tests=%d shards=%d timeout=%s\n' \
  "${#command_tests[@]}" "${command_shards}" "${command_race_timeout}"
for ((shard = 0; shard < command_shards; shard++)); do
  run_command_shard "${shard}"
done

log_dir="$(mktemp -d /tmp/evalwitness-race.XXXXXX)"
trap 'rm -rf "${log_dir}"' EXIT

stress_tests=()
while IFS= read -r test_name; do
  case "${test_name}" in
    Test*|Example*|Fuzz*)
      stress_tests+=("${test_name}")
      ;;
  esac
done < <(go test "${stress_package}" -list '^(Test|Example|Fuzz)')

if [ "${#stress_tests[@]}" -eq 0 ]; then
  echo "race gate: no stress tests discovered" >&2
  exit 1
fi

for ((left = 0; left < ${#stress_tests[@]}; left++)); do
  for ((right = 0; right < left; right++)); do
    if [ "${stress_tests[left]}" = "${stress_tests[right]}" ]; then
      echo "race gate: duplicate stress test ${stress_tests[left]}" >&2
      exit 1
    fi
  done
done

run_stress_shard() {
  local shard_index="$1"
  local pattern="^("
  local separator=""
  local selected=0
  local index

  for ((index = shard_index; index < ${#stress_tests[@]}; index += stress_shards)); do
    pattern="${pattern}${separator}${stress_tests[index]}"
    separator="|"
    selected=$((selected + 1))
  done
  pattern="${pattern})$"

  printf '==> race: stress shard %d/%d (%d tests)\n' \
    "$((shard_index + 1))" "${stress_shards}" "${selected}"
  go test -race -count=1 -timeout "${stress_race_timeout}" "${stress_package}" -run "${pattern}"
}

assigned_stress_tests=0
for ((shard = 0; shard < stress_shards; shard++)); do
  for ((index = shard; index < ${#stress_tests[@]}; index += stress_shards)); do
    assigned_stress_tests=$((assigned_stress_tests + 1))
  done
done
if [ "${assigned_stress_tests}" -ne "${#stress_tests[@]}" ]; then
  echo "race gate: stress shard assignment is incomplete" >&2
  exit 1
fi

printf 'Stress race inventory: tests=%d shards=%d timeout=%s\n' \
  "${#stress_tests[@]}" "${stress_shards}" "${stress_race_timeout}"
stress_shard_pids=()
for ((shard = 0; shard < stress_shards; shard++)); do
  run_stress_shard "${shard}" >"${log_dir}/stress-${shard}.log" 2>&1 &
  stress_shard_pids[shard]=$!
done

failed=0
for ((shard = 0; shard < stress_shards; shard++)); do
  if ! wait "${stress_shard_pids[shard]}"; then
    failed=1
  fi
  sed -n '1,$p' "${log_dir}/stress-${shard}.log"
done
if [ "${failed}" -ne 0 ]; then
  echo "race gate: at least one stress shard failed" >&2
  exit 1
fi

for ((index = 0; index < ${#heavy_packages[@]}; index++)); do
  package_timeout="${heavy_race_timeouts[index]}"
  started_at=${SECONDS}
  printf '==> race: heavy package %d/%d (%s; timeout=%s)\n' \
    "$((index + 1))" "${#heavy_packages[@]}" "${heavy_packages[index]}" "${package_timeout}"
  go test -race -count=1 -timeout "${package_timeout}" "${heavy_packages[index]}"
  printf 'race heavy package passed: package=%s duration_seconds=%d timeout=%s\n' \
    "${heavy_packages[index]}" "$((SECONDS - started_at))" "${package_timeout}"
done

lineage_tests=()
while IFS= read -r test_name; do
  case "${test_name}" in
    Test*|Example*|Fuzz*)
      lineage_tests+=("${test_name}")
      ;;
  esac
done < <(go test ./internal/lineage -list '^(Test|Example|Fuzz)')

if [ "${#lineage_tests[@]}" -eq 0 ]; then
  echo "race gate: no lineage tests discovered" >&2
  exit 1
fi

for ((left = 0; left < ${#lineage_tests[@]}; left++)); do
  for ((right = 0; right < left; right++)); do
    if [ "${lineage_tests[left]}" = "${lineage_tests[right]}" ]; then
      echo "race gate: duplicate lineage test ${lineage_tests[left]}" >&2
      exit 1
    fi
  done
done

assigned=0
for ((shard = 0; shard < lineage_shards; shard++)); do
  for ((index = shard; index < ${#lineage_tests[@]}; index += lineage_shards)); do
    assigned=$((assigned + 1))
  done
done
if [ "${assigned}" -ne "${#lineage_tests[@]}" ]; then
  echo "race gate: lineage shard assignment is incomplete" >&2
  exit 1
fi

run_lineage_shard() {
  local shard_index="$1"
  local pattern="^("
  local separator=""
  local selected=0
  local index

  for ((index = shard_index; index < ${#lineage_tests[@]}; index += lineage_shards)); do
    pattern="${pattern}${separator}${lineage_tests[index]}"
    separator="|"
    selected=$((selected + 1))
  done
  pattern="${pattern})$"

  printf '==> race: lineage shard %d/%d (%d tests)\n' \
    "$((shard_index + 1))" "${lineage_shards}" "${selected}"
  go test -race -count=1 -timeout "${lineage_race_timeout}" ./internal/lineage -run "${pattern}"
}

shard_pids=()

printf 'Lineage race inventory: tests=%d shards=%d timeout=%s\n' \
  "${#lineage_tests[@]}" "${lineage_shards}" "${lineage_race_timeout}"
for ((shard = 0; shard < lineage_shards; shard++)); do
  run_lineage_shard "${shard}" >"${log_dir}/lineage-${shard}.log" 2>&1 &
  shard_pids[shard]=$!
done

failed=0
for ((shard = 0; shard < lineage_shards; shard++)); do
  if ! wait "${shard_pids[shard]}"; then
    failed=1
  fi
  sed -n '1,$p' "${log_dir}/lineage-${shard}.log"
done
if [ "${failed}" -ne 0 ]; then
  echo "race gate: at least one lineage shard failed" >&2
  exit 1
fi

printf 'Race gate passed: project_packages=%d light_packages=%d command_tests=%d command_shards=%d stress_tests=%d stress_shards=%d heavy_packages=%d lineage_tests=%d lineage_shards=%d\n' \
  "${#project_packages[@]}" "${#light_packages[@]}" "${#command_tests[@]}" "${command_shards}" \
  "${#stress_tests[@]}" "${stress_shards}" "${#heavy_packages[@]}" "${#lineage_tests[@]}" "${lineage_shards}"
