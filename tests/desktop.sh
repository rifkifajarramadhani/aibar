#!/usr/bin/env bash
# Run the packaged Omarchy shell in a separate Hyprland session with fake usage.
set -euo pipefail

repo=$(cd -- "$(dirname -- "$0")/.." && pwd)
omarchy_path=${OMARCHY_PATH:-/usr/share/omarchy}
: "${WAYLAND_DISPLAY:?run from a Wayland session}"
: "${XDG_RUNTIME_DIR:?run from a Wayland session}"
outer_display=$WAYLAND_DISPLAY
[[ $outer_display = /* ]] || outer_display=$XDG_RUNTIME_DIR/$outer_display

test_root=$(mktemp -d /tmp/ab.XXXXX)
result=1
trap 'if (( result == 0 )); then rm -r -- "$test_root"; else echo "Desktop logs and screenshots: $test_root" >&2; fi' EXIT
mkdir -m 700 "$test_root/run"

dbus-run-session -- bash -s "$repo" "$omarchy_path" "$test_root" "$outer_display" 3>&1 >"$test_root/session.log" 2>&1 <<'SESSION'
set -euo pipefail
repo=$1
export OMARCHY_PATH=$2
test_root=$3
export WAYLAND_DISPLAY=$4
export HOME="$test_root/home" XDG_RUNTIME_DIR="$test_root/run"
export XDG_CONFIG_HOME="$HOME/.config" XDG_STATE_HOME="$test_root/state"
export XDG_CACHE_HOME="$test_root/cache" XDG_DATA_HOME="$test_root/data"
export PATH="$test_root/bin:$OMARCHY_PATH/bin:$PATH" QT_QPA_PLATFORM=wayland
unset DISPLAY HYPRLAND_INSTANCE_SIGNATURE

plugin="$XDG_CONFIG_HOME/omarchy/plugins/overhaul.aibar"
theme="$HOME/.local/state/omarchy/current/theme"
usage="$XDG_STATE_HOME/omarchy/agents/usage"
mkdir -p "$plugin" "$theme" "$test_root/bin"
cp -R "$repo/manifest.json" "$repo/widget" "$plugin/"
cp "$OMARCHY_PATH/themes/tokyo-night/colors.toml" "$theme/"
node "$repo/tests/fixtures.mjs" "$usage"
cat > "$XDG_CONFIG_HOME/omarchy/shell.json" <<'JSON'
{
  "version": 1,
  "bar": {
    "position": "top",
    "background": "opaque",
    "layout": {
      "left": [], "centerBegin": [], "center": [], "centerEnd": [], "tray": [],
      "right": [{ "id": "overhaul.aibar" }]
    }
  },
  "plugins": [{ "id": "omarchy.agents", "enabled": false }, { "id": "overhaul.aibar", "enabled": true }]
}
JSON
cat > "$test_root/bin/omarchy-agent-usage-update" <<'UPDATER'
#!/usr/bin/env bash
set -e
printf '%s\n' "${*:-startup}" >> "$HOME/refresh.log"
sleep 0.2
if [[ ${1:-} == --force ]]; then
  usage=$XDG_STATE_HOME/omarchy/agents/usage
  jq '.limits[0].percent = 0.55' "$usage/codex.json" > "$usage/.codex"
  mv "$usage/.codex" "$usage/codex.json"
  printf '%s\n' 'omarchy-agent-usage-update: claude collector failed' >&2
  exit 1
fi
UPDATER
chmod +x "$test_root/bin/omarchy-agent-usage-update"
cat > "$test_root/hyprland.lua" <<'LUA'
hl.monitor({ output = "", mode = "preferred", position = "auto", scale = 1 })
hl.config({ misc = { disable_hyprland_logo = true, disable_splash_rendering = true }, cursor = { no_hardware_cursors = true } })
LUA

shell_pid=
hypr_pid=
cleanup() {
  if [[ -n $shell_pid ]]; then kill "$shell_pid" 2>/dev/null || true; wait "$shell_pid" 2>/dev/null || true; fi
  if [[ -n $hypr_pid ]]; then kill "$hypr_pid" 2>/dev/null || true; wait "$hypr_pid" 2>/dev/null || true; fi
}
trap cleanup EXIT

AQ_BACKENDS=wayland Hyprland --config "$test_root/hyprland.lua" > "$test_root/hypr.log" 2>&1 &
hypr_pid=$!
shopt -s nullglob

ready() {
  local instances=("$XDG_RUNTIME_DIR"/hypr/*/.socket.sock)
  local displays=("$XDG_RUNTIME_DIR"/wayland-*.lock)
  (( ${#instances[@]} && ${#displays[@]} )) || return 1
  export HYPRLAND_INSTANCE_SIGNATURE=$(basename "$(dirname "${instances[0]}")")
  export WAYLAND_DISPLAY=$(basename "${displays[0]}" .lock)
  hyprctl -j monitors | jq -e 'length > 0' >/dev/null
}

until_ready() {
  for _ in {1..80}; do
    if "$@" >/dev/null 2>&1; then return; fi
    sleep 0.1
  done
  echo "Timed out waiting for $*" >&2
  return 1
}

until_ready ready
first=$(hyprctl -j monitors | jq -r '.[0].name')
hyprctl output create headless >/dev/null
second=$(hyprctl -j monitors | jq -er --arg first "$first" '.[] | select(.name != $first) | .name')

quickshell -p "$OMARCHY_PATH/shell" > "$test_root/shell.log" 2>&1 &
shell_pid=$!
until_ready omarchy-shell shell ping
until_ready test -s "$HOME/refresh.log"
sleep 0.6
[[ $(cat "$HOME/refresh.log") == startup ]]

capture() {
  local output=$1 name=$2 width height
  grim -o "$output" "$test_root/$name.png"
  read -r width height < <(magick identify -format '%w %h\n' "$test_root/$name.png")
  local crop_width=$(( width < 440 ? width : 440 ))
  height=${3:-$height}
  local top=${4:-0}
  magick "$test_root/$name.png" -crop "${crop_width}x${height}+$((width - crop_width))+$top" +repage \
    -resize 200% "$test_root/$name-crop.png"
  # Column mode recognizes countdowns tucked beneath their meters.
  tesseract "$test_root/$name-crop.png" "$test_root/$name" --psm 4 txt tsv 2>/dev/null
}

check() {
  local name=$1 pattern=$2
  if ! rg -i "$pattern" "$test_root/$name.txt"; then
    echo "Expected '$pattern' in $name" >&2
    return 1
  fi
}

has_color() {
  local expected_color=$1
  shift
  magick "$@" -depth 8 -format %c histogram:info: | rg -Fi "$expected_color" >/dev/null
}

check_icon() {
  local name=$1 heading=$2 expected_color=$3 left top coverage
  # OCR locates the heading in the 2x crop. Sample its icon slot to the left.
  if ! read -r left top < <(awk -F '\t' -v word="$heading" 'tolower($12) ~ ("^" word "$") { print $7, $8; exit }' "$test_root/$name.tsv"); then
    echo "Could not locate $heading in $name" >&2
    return 1
  fi
  # Allow for antialiasing and reject isolated matching pixels.
  coverage=$(magick "$test_root/$name-crop.png" -crop "44x44+$((left - 56))+$((top - 10))" +repage \
    -fuzz 3% -transparent "$expected_color" -alpha extract -negate -format '%[fx:mean]' info:)
  if ! awk -v coverage="$coverage" 'BEGIN { exit !(coverage > 0.01) }'; then
    echo "Expected icon color $expected_color beside $heading in $name" >&2
    return 1
  fi
}

[[ $(omarchy-shell shell summon overhaul.aibar) == ok ]]
sleep 0.6
capture "$first" dark
check dark 'Claude Code'
check dark 'Some usage records could not be read'
check dark 'Stale.*updated'
check dark '<img src='
check dark '37% used'
check dark 'Resets in [0-9]+m [0-5]?[0-9]s'
check_icon dark claude '#D97757'
check_icon dark 'c?codex' '#FFFFFF'
sleep 1.2
capture "$first" countdown
reset_line='Resets in [0-9]+m [0-5]?[0-9]s'
[[ $(rg -im1 "$reset_line" "$test_root/dark.txt") != $(rg -im1 "$reset_line" "$test_root/countdown.txt") ]]

wtype -k End
sleep 0.3
capture "$first" end
check end 'USD remaining'
check end 'Hermes'
check end 'Waiting for auth'
check end 'Test Provider'
check end 'Both Monitors'
dark_text=$(sed -n 's/^foreground = "\(#.*\)"/\1/p' "$theme/colors.toml")
check_icon end test "$dark_text"

wtype -k Return
sleep 0.5
wtype -k Home
sleep 0.2
capture "$first" failed
check failed 'Some usage could not be refreshed'
check failed 'Claude Code'
check failed '55% used'

dark_color=$(sed -n 's/^background = "\(#.*\)"/\1/p' "$theme/colors.toml")
cp "$OMARCHY_PATH/themes/catppuccin-latte/colors.toml" "$theme/colors.toml"
light_color=$(sed -n 's/^background = "\(#.*\)"/\1/p' "$theme/colors.toml")
[[ $(omarchy-shell shell applyTheme "$(base64 -w0 "$theme/colors.toml")" "") == ok ]]
sleep 0.5
capture "$first" light
has_color "$dark_color" "$test_root/dark.png"
has_color "$light_color" "$test_root/light.png"
check_icon light claude '#D97757'
check_icon light 'c?codex' '#111111'

wtype -k End
sleep 0.3
capture "$first" light_end
check light_end 'Test Provider'
check light_end 'Both Monitors'
light_text=$(sed -n 's/^foreground = "\(#.*\)"/\1/p' "$theme/colors.toml")
check_icon light_end test "$light_text"

wtype -k Escape
sleep 0.3
capture "$first" closed
if rg -iq 'usage|Claude Code' "$test_root/closed.txt"; then
  echo "Escape left the popup open" >&2
  exit 1
fi

hyprctl dispatch "hl.dsp.focus({ monitor = \"$second\" })" >/dev/null
[[ $(omarchy-shell shell summon overhaul.aibar) == ok ]]
sleep 0.6
capture "$second" second
check second 'Claude Code'
[[ $(cat "$HOME/refresh.log") == $'startup\n--limits-only\n--force\n--limits-only' ]]

# Move only the synthetic records. Reopening must show the empty state.
mkdir "$test_root/used-records"
mv "$usage"/*.json "$test_root/used-records/"
wtype -k Escape
sleep 0.3
hyprctl dispatch "hl.dsp.focus({ monitor = \"$second\" })" >/dev/null
[[ $(omarchy-shell shell summon overhaul.aibar) == ok ]]
sleep 0.6
capture "$second" empty 78 63
check empty 'No usage records yet'

if rg -n 'Plugin widget overhaul.aibar failed|service plugin load failed for overhaul.aibar|overhaul.aibar/widget/.*(TypeError|ReferenceError|Binding loop|recursive rearrange)' "$test_root/shell.log"; then
  exit 1
fi
echo "Verified the native popup on two outputs, provider icons, live themes, keyboard, countdown, and partial refresh." >&3
SESSION
result=0
