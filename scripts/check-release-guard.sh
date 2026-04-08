#!/usr/bin/env bash

set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "${repo_root}"

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

read_version() {
  tr -d '[:space:]' < VERSION
}

is_valid_semver() {
  [[ "$1" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]
}

semver_gt() {
  local lhs="${1#v}"
  local rhs="${2#v}"
  local lhs_major lhs_minor lhs_patch
  local rhs_major rhs_minor rhs_patch

  IFS=. read -r lhs_major lhs_minor lhs_patch <<<"${lhs}"
  IFS=. read -r rhs_major rhs_minor rhs_patch <<<"${rhs}"

  if (( lhs_major != rhs_major )); then
    (( lhs_major > rhs_major ))
    return
  fi
  if (( lhs_minor != rhs_minor )); then
    (( lhs_minor > rhs_minor ))
    return
  fi
  (( lhs_patch > rhs_patch ))
}

path_changed_from_base() {
  local base_ref="$1"
  local path="$2"

  if ! git diff --quiet "origin/${base_ref}...HEAD" -- "${path}"; then
    return 0
  fi

  git status --porcelain -- "${path}" | grep -q .
}

version="$(read_version)"
is_valid_semver "${version}" || die "VERSION must contain vMAJOR.MINOR.PATCH; got '${version}'"

release_notes="docs/releases/${version}.md"
[[ -s "${release_notes}" ]] || die "release note file missing or empty: ${release_notes}"

for section in \
  "## Summary" \
  "## API / behavior changes" \
  "## New validated fixture coverage" \
  "## exiftool parity fixes" \
  "## Scope-policy changes" \
  "## Known remaining gaps"; do
  if ! grep -Fqx "${section}" "${release_notes}"; then
    die "release notes must contain section '${section}'"
  fi
done

latest_tag="$(git tag --list 'v*' --sort=-version:refname | head -n1)"
if [[ -n "${latest_tag}" ]]; then
  [[ "${version}" != "${latest_tag}" ]] || die "VERSION ${version} matches the latest tag; bump it before merging"
  semver_gt "${version}" "${latest_tag}" || die "VERSION ${version} must be greater than the latest tag ${latest_tag}"
fi

base_ref="${VIDEOMETA_RELEASE_GUARD_BASE_REF:-${GITHUB_BASE_REF:-}}"
if [[ -n "${base_ref}" ]]; then
  git rev-parse --verify "origin/${base_ref}" >/dev/null 2>&1 || git fetch --no-tags origin "${base_ref}" >/dev/null 2>&1

  base_version="$(git show "origin/${base_ref}:VERSION" 2>/dev/null | tr -d '[:space:]' || true)"
  if [[ -n "${base_version}" && "${version}" == "${base_version}" ]]; then
    die "VERSION must change from origin/${base_ref} (${base_version}) for every release-producing PR"
  fi

  path_changed_from_base "${base_ref}" VERSION || die "VERSION was not modified relative to origin/${base_ref}"
  path_changed_from_base "${base_ref}" "${release_notes}" || die "${release_notes} must be added or updated in the PR"
fi
