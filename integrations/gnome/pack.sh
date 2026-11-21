#!/usr/bin/env bash
set -euo pipefail

source_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
output_dir="${1:-.}"
mkdir -p -- "$output_dir"
stage="$(mktemp -d)"
trap 'rm -rf -- "$stage"' EXIT

cp -- "$source_dir"/{extension.js,reader.js,metadata.json,stylesheet.css} \
  "$source_dir/../../widget/Records.mjs" "$stage/"
gnome-extensions pack --out-dir="$output_dir" \
  --extra-source=reader.js --extra-source=Records.mjs "$stage"
