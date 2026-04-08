#!/usr/bin/env bash

set -euo pipefail

version="${EXIFTOOL_VERSION:-13.50}"
work_dir="${RUNNER_TEMP:-$(pwd)/.tmp}"
archive="${work_dir}/exiftool-${version}.tar.gz"
extract_dir="${work_dir}/exiftool-${version}"
bin_dir="${work_dir}/bin"

mkdir -p "${work_dir}" "${bin_dir}"

if [[ ! -x "${extract_dir}/exiftool" ]]; then
  curl -fsSL "https://github.com/exiftool/exiftool/archive/refs/tags/${version}.tar.gz" -o "${archive}"
  tar -xzf "${archive}" -C "${work_dir}"
fi

cat > "${bin_dir}/exiftool" <<EOF
#!/usr/bin/env bash
exec perl -I "${extract_dir}/lib" "${extract_dir}/exiftool" "\$@"
EOF
chmod +x "${bin_dir}/exiftool"

if [[ -n "${GITHUB_PATH:-}" ]]; then
  printf '%s\n' "${bin_dir}" >> "${GITHUB_PATH}"
else
  export PATH="${bin_dir}:${PATH}"
fi

"${bin_dir}/exiftool" -ver
