#!/usr/bin/env bash
set -euo pipefail

repo="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
temp="$(mktemp -d)"
trap 'rm -rf -- "$temp"' EXIT

"$repo/integrations/gnome/pack.sh" "$temp"
mkdir -m 700 "$temp/runtime"
env -u WAYLAND_DISPLAY -u DISPLAY \
  XDG_RUNTIME_DIR="$temp/runtime" XDG_STATE_HOME="$temp/state" LC_ALL=C.UTF-8 \
  timeout 90s dbus-run-session -- gnome-shell-test-tool --headless --disable-animations \
  --extension "$temp/aibar@rifkifajarramadhani.github.io.shell-extension.zip" "$repo/tests/gnome.js"
