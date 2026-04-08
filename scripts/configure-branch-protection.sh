#!/usr/bin/env bash

set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "${repo_root}"

repo_slug="${GITHUB_REPOSITORY:-}"
if [[ -z "${repo_slug}" ]]; then
  remote_url="$(git config --get remote.origin.url)"
  repo_slug="$(printf '%s' "${remote_url}" | sed -E 's#(git@github.com:|https://github.com/)##; s#\.git$##')"
fi

main_branch="${1:-main}"

gh api \
  --method PUT \
  -H "Accept: application/vnd.github+json" \
  "repos/${repo_slug}/branches/${main_branch}/protection" \
  --input - <<'JSON'
{
  "required_status_checks": {
    "strict": true,
    "contexts": [
      "release-guard",
      "hosted-verify"
    ]
  },
  "enforce_admins": true,
  "required_pull_request_reviews": {
    "dismiss_stale_reviews": true,
    "require_code_owner_reviews": false,
    "required_approving_review_count": 0,
    "require_last_push_approval": false
  },
  "restrictions": null,
  "required_linear_history": false,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "block_creations": false,
  "required_conversation_resolution": true,
  "lock_branch": false,
  "allow_fork_syncing": false
}
JSON
