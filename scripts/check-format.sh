#!/usr/bin/env bash

set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "${repo_root}"

if ! command -v gofumpt >/dev/null 2>&1; then
  printf 'error: gofumpt not found in PATH\n' >&2
  exit 1
fi

if ! command -v goimports >/dev/null 2>&1; then
  printf 'error: goimports not found in PATH\n' >&2
  exit 1
fi

go_files="$(git ls-files '*.go')"
if [[ -z "${go_files}" ]]; then
  exit 0
fi

gofumpt_output="$(gofumpt -l ${go_files})"
if [[ -n "${gofumpt_output}" ]]; then
  printf 'error: gofumpt would reformat these files:\n%s\n' "${gofumpt_output}" >&2
  exit 1
fi

goimports_output="$(goimports -l -local github.com/tonimelisma/videometa ${go_files})"
if [[ -n "${goimports_output}" ]]; then
  printf 'error: goimports would rewrite these files:\n%s\n' "${goimports_output}" >&2
  exit 1
fi
