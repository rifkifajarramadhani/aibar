# Platform support for aibar

Keep `main` as the native Omarchy plugin. If the goal is to preserve the old
Waybar version, an annotated tag such as `waybar-legacy` at `fa158aa` gives
users a fixed point to check out. If someone wants to maintain that version,
`maintenance/waybar` can start at the same commit. A branch gives Waybar its own
fixes and release schedule, with a second build and test path to maintain.
[Git tagging guide](https://git-scm.com/book/en/v2/Git-Basics-Tagging)

## How other projects organize support

- **One branch, multiple backends.** Eww enables X11 and Wayland by default
  using Cargo features. Its CI targets `master` and checks the default build
  and each backend individually. Waybar's own Meson build selects Linux or
  BSD CPU and memory implementations and keeps modules for Sway, Hyprland,
  and Wayfire in the same source tree.
  [Eww feature definitions](https://github.com/elkowar/eww/blob/master/crates/eww/Cargo.toml)
  · [Eww build workflow](https://github.com/elkowar/eww/blob/master/.github/workflows/build.yml)
  · [Waybar build](https://github.com/Alexays/Waybar/blob/master/meson.build)
- **One branch, multiple OS builds.** Ripgrep uses a release workflow to build
  Linux, macOS, and Windows binaries. A single release can carry artifacts for
  several operating systems.
  [Ripgrep release workflow](https://github.com/BurntSushi/ripgrep/blob/master/.github/workflows/release.yml)
  · [Ripgrep releases](https://github.com/BurntSushi/ripgrep/releases)
- **Long-lived platform branches.** Mole develops its macOS CLI on `main`,
  Linux on `linux`, and Windows on `windows`. The maintainers' synchronization
  notes show which fixes they applied to each branch and which tags shipped
  them. The November 10 review records a fix for truncating wide characters
  in a column, with separate commits and release versions for all three CLIs.
  [Mole synchronization notes](https://github.com/tw93/Mole/blob/main/SYNC.md#2026-11-10-three-cli-review)
  · [Mole releases](https://github.com/tw93/Mole/releases)
- **Separate repositories.** KDE Connect uses one C++ and QML repository for
  its Linux, Windows, macOS, and Linux Mobile clients. Its Java Android client
  and Swift/SwiftUI iOS client each have their own repository and build
  instructions.
  [KDE Connect development guide](https://kdeconnect.kde.org/get-involved.html)

## What would a split mean here?

Omarchy's Quickshell bar and Waybar are two integration targets on Wayland.
The Omarchy plugin loads QML and reads records from the Omarchy updater. The
old Waybar configuration launches a Go daemon that collects Codex and Claude
usage and prints JSON for Waybar. `fa158aa` is the last Waybar-only commit.
`9b91d66` added the Omarchy widget alongside it, and `ce4afa1` removed the Go
code and Waybar config.
[aibar runtime](PROJECT_OVERVIEW.md#runtime)
· [Old Waybar config](https://github.com/rifkifajarramadhani/aibar/blob/fa158aa741e42f2d8ee4a228cf5f1a07cd2e33e5/waybar-snippet.jsonc)
· [Native widget commit](https://github.com/rifkifajarramadhani/aibar/commit/9b91d661b2ac0f0870463f656bd7398eedc6ae57)
· [Waybar retirement commit](https://github.com/rifkifajarramadhani/aibar/commit/ce4afa1e7e0579788af9b80a04887585f2906acf)
· [Waybar compositor support](https://github.com/Alexays/Waybar#compositor-integration)

For a maintained Waybar branch, document both install paths on the default
branch. Run the Go tests on Waybar changes and check a real Waybar launch and
refresh before calling the branch supported. The old Go daemon owns provider
collection, so provider changes can require maintenance even when the Waybar
config stays the same. Keep the Omarchy desktop checks for the QML build.
[Old Waybar setup](https://github.com/rifkifajarramadhani/aibar/blob/fa158aa741e42f2d8ee4a228cf5f1a07cd2e33e5/README.md)
· [Omarchy development checks](../AGENTS.md#development-checks)

If both integrations return to active development, keep the Omarchy entry
points where they are and put the Waybar code in `integrations/waybar/` on
`main`. Build and test each integration separately. If both bars eventually
share a record format, one pull request can test a record change against both.
Waybar users outside Omarchy would still need a collector that provides those
records. A separate repository would make sense if Waybar gains its own
maintainers and needs its own issue tracker and releases.
[Plugin manifest](../manifest.json)
· [Omarchy record contract](PROJECT_OVERVIEW.md#record-contract)

## Verification

The Go tests passed at `fa158aa` in a temporary checkout with an isolated home.
On `main`, the desktop smoke test passed with synthetic records on Omarchy
4.5.0-3 under Wayland. The archived Waybar build still needs a desktop test
and live provider checks before it can be offered as supported.
