#!/usr/bin/env bash

set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
target_dir="${repo_root}/testdata"

committed_fixtures=(
  "IMG_5179.MOV"
  "google.mp4"
  "sony_a6700.mp4"
)

bootstrap_fixtures=(
  "gopro_action.mp4"
  "dji_inspire3_car_4k120_rec709.mov"
  "dji_ronin4d_4k_prores4444_25fps.mov"
)

missing=()
for fixture in "${committed_fixtures[@]}"; do
  if [[ ! -f "${target_dir}/${fixture}" ]]; then
    missing+=("${fixture}")
  fi
done

if (( ${#missing[@]} > 0 )); then
  printf 'error: committed validated fixtures are missing:\n' >&2
  printf '  %s\n' "${missing[@]}" >&2
  exit 1
fi

need_bootstrap=0
for fixture in "${bootstrap_fixtures[@]}"; do
  if [[ ! -f "${target_dir}/${fixture}" ]]; then
    need_bootstrap=1
    break
  fi
done

if (( need_bootstrap )); then
  (
    cd "${repo_root}"
    ./scripts/bootstrap-fixtures.sh
  )
fi

missing=()
for fixture in "${bootstrap_fixtures[@]}"; do
  if [[ ! -f "${target_dir}/${fixture}" ]]; then
    missing+=("${fixture}")
  fi
done

if (( ${#missing[@]} > 0 )); then
  printf 'error: bootstrap-downloadable validated fixtures are missing after bootstrap:\n' >&2
  printf '  %s\n' "${missing[@]}" >&2
  exit 1
fi

printf 'validated fixture corpus ready:\n'
printf '\ncommitted fixtures:\n'
for fixture in "${committed_fixtures[@]}"; do
  ls -lh "${target_dir}/${fixture}"
done

printf '\nbootstrap fixtures:\n'
for fixture in "${bootstrap_fixtures[@]}"; do
  ls -lh "${target_dir}/${fixture}"
done
