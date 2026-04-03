#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
manifest="${script_dir}/fixture_bootstrap.tsv"

usage() {
  cat <<'EOF'
Usage:
  ./scripts/bootstrap-fixtures.sh
  ./scripts/bootstrap-fixtures.sh --list
  ./scripts/bootstrap-fixtures.sh --help

By default, downloads every fixture listed in scripts/fixture_bootstrap.tsv into
the repo's testdata/ directory. Existing files with the expected byte size are
left untouched. Downloads are resumable. Smaller fixtures are downloaded first
so common validation assets are restored quickly.

Supported manifest kinds:
  direct       Download directly from a verified asset URL.
  gopro-share  Resolve a fresh signed GoPro asset URL from a public share page.
EOF
}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

file_size() {
  local path="$1"
  stat -f '%z' "$path" 2>/dev/null || stat -c '%s' "$path"
}

final_header_value() {
  local header_file="$1"
  local header_name="$2"
  awk -v key="${header_name}" '
    BEGIN { IGNORECASE = 1 }
    $0 ~ ("^" key ":") {
      value = $0
    }
    END {
      sub(/\r$/, "", value)
      sub(/^[^:]*:[[:space:]]*/, "", value)
      print value
    }
  ' "${header_file}"
}

resolve_gopro_share() {
  local share_url="$1"
  python3 - "${share_url}" <<'PY'
import json
import sys
import urllib.parse
import urllib.request

share_url = sys.argv[1]
collection_id = urllib.parse.urlparse(share_url).path.rstrip("/").split("/")[-1]
if not collection_id:
    raise SystemExit("could not extract GoPro collection id from share URL")

media_headers = {
    "Accept": "application/vnd.gopro.jk.media+json; version=2.0.0",
    "Content-Type": "application/json",
}

def fetch_json(url, headers):
    req = urllib.request.Request(url, headers=headers)
    with urllib.request.urlopen(req) as resp:
        return json.load(resp)

items = fetch_json(
    f"https://api.gopro.com/media/items/{collection_id}",
    media_headers,
)
item_list = items.get("items") or []
if not item_list:
    raise SystemExit("GoPro share page returned no media items")

medium = item_list[0].get("medium") or {}
medium_id = medium.get("id")
if not medium_id:
    raise SystemExit("GoPro share item has no medium id")

download = fetch_json(
    f"https://api.gopro.com/media/{medium_id}/download",
    media_headers,
)
embedded = download.get("_embedded") or {}
variations = embedded.get("variations") or []
selected_url = ""
for label in ("source", "baked_source"):
    for variation in variations:
        if variation.get("available") and variation.get("label") == label:
            selected_url = variation.get("url") or ""
            break
    if selected_url:
        break

if not selected_url:
    files = embedded.get("files") or []
    for file_entry in files:
        if file_entry.get("available"):
            selected_url = file_entry.get("url") or ""
            if selected_url:
                break

if not selected_url:
    raise SystemExit("GoPro download API returned no usable file URL")

print(selected_url)
PY
}

resolve_url() {
  local kind="$1"
  local locator="$2"
  case "${kind}" in
    direct)
      printf '%s\n' "${locator}"
      ;;
    gopro-share)
      resolve_gopro_share "${locator}"
      ;;
    *)
      die "unsupported manifest kind: ${kind}"
      ;;
  esac
}

validate_remote() {
  local url="$1"
  local expected_type="$2"
  local expected_size="$3"
  local header_file
  local actual_type
  local actual_size

  header_file="$(mktemp)"

  curl -fsSIL -D "${header_file}" -o /dev/null "${url}"

  actual_type="$(final_header_value "${header_file}" 'content-type' | tr '[:upper:]' '[:lower:]')"
  actual_size="$(final_header_value "${header_file}" 'content-length')"
  rm -f "${header_file}"

  if [[ -n "${expected_type}" && "${actual_type}" != "${expected_type}"* ]]; then
    die "unexpected content-type for ${url}: got '${actual_type}', want prefix '${expected_type}'"
  fi
  if [[ -n "${expected_size}" && -n "${actual_size}" && "${actual_size}" != "${expected_size}" ]]; then
    die "unexpected content-length for ${url}: got '${actual_size}', want '${expected_size}'"
  fi
}

download_fixture() {
  local id="$1"
  local target="$2"
  local kind="$3"
  local locator="$4"
  local source_page="$5"
  local expected_type="$6"
  local expected_size="$7"
  local description="$8"
  local abs_target
  local actual_size
  local url

  abs_target="${repo_root}/${target}"
  mkdir -p "$(dirname "${abs_target}")"

  printf '\n[%s] %s\n' "${id}" "${description}"
  printf '  target: %s\n' "${target}"
  printf '  kind:   %s\n' "${kind}"
  printf '  source: %s\n' "${source_page}"

  url="$(resolve_url "${kind}" "${locator}")"
  validate_remote "${url}" "${expected_type}" "${expected_size}"

  if [[ -f "${abs_target}" ]]; then
    actual_size="$(file_size "${abs_target}")"
    if [[ -n "${expected_size}" && "${actual_size}" == "${expected_size}" ]]; then
      printf '  status: already present (%s bytes)\n' "${actual_size}"
      return
    fi
    printf '  status: resuming existing partial/local file (%s bytes)\n' "${actual_size}"
  else
    printf '  status: downloading\n'
  fi

  curl -fL --retry 3 --retry-delay 2 --continue-at - --output "${abs_target}" "${url}"

  actual_size="$(file_size "${abs_target}")"
  if [[ -n "${expected_size}" && "${actual_size}" != "${expected_size}" ]]; then
    rm -f "${abs_target}"
    die "downloaded size mismatch for ${target}: got '${actual_size}', want '${expected_size}'"
  fi

  printf '  done: %s bytes\n' "${actual_size}"
}

list_manifest() {
  awk -F '\t' '
    NR == 1 { next }
    {
      printf "%-32s %-12s %s\n", $1, $3, $8
    }
  ' "${manifest}"
}

sorted_manifest_rows() {
  awk -F '\t' '
    NR == 1 { next }
    {
      size = $7
      if (size == "") {
        size = "999999999999999"
      }
      printf "%015d\t%s\t%s\n", size + 0, $2, $0
    }
  ' "${manifest}" | sort -t $'\t' -k1,1n -k2,2 | cut -f3-
}

if [[ ! -f "${manifest}" ]]; then
  die "manifest not found: ${manifest}"
fi

case "${1-}" in
  --help|-h)
    usage
    exit 0
    ;;
  --list)
    list_manifest
    exit 0
    ;;
  "")
    ;;
  *)
    usage
    die "this script is intentionally full-service; run it without arguments"
    ;;
esac

printf 'Downloading all bootstrap fixtures listed in %s\n' "${manifest}"

while IFS=$'\t' read -r id target kind locator source_page expected_type expected_size description; do
  download_fixture "${id}" "${target}" "${kind}" "${locator}" "${source_page}" "${expected_type}" "${expected_size}" "${description}"
done < <(sorted_manifest_rows)

cat <<'EOF'

Bootstrap complete.

These downloads are local-only fixtures. They are gitignored on purpose.
For the remaining manual-only fixtures, see docs/FIXTURE_ACQUISITION.md.
EOF
