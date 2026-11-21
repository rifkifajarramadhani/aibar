# Repository guidelines

## Project structure

aibar supports the Omarchy bar and the GNOME Shell top bar. The Omarchy
baseline is 4.5.0-3. The root `manifest.json` declares the Omarchy plugin's
two entry points.

- `widget/Panel.qml` draws the bar button and popup using Omarchy's shared UI.
- `widget/Usage.qml` is the shared service for refresh and record discovery.
- `widget/Records.mjs` validates and normalizes version 1 usage records.
- `widget/OmarchyFiles.mjs` decodes Omarchy's file discovery output.
- `widget/i18n/` contains aibar's translation catalogs.
- `integrations/gnome/` contains the GNOME Shell 50 and 51 extension and
  its ZIP packager.
- `tests/` contains synthetic records, parser tests, and desktop smoke tests.

Read [the project overview](docs/PROJECT_OVERVIEW.md) for the runtime and
[DESIGN.md](DESIGN.md) for the visual conventions. Use the packaged shell at
`/usr/share/omarchy/shell` as the compatibility reference.

## Development checks

```sh
node --test tests/records.test.mjs
omarchy plugin validate .
/usr/lib/qt6/bin/qmlformat -w 2 -i widget/*.qml
/usr/lib/qt6/bin/qmllint widget/Usage.qml widget/Records.mjs widget/OmarchyFiles.mjs
bash -n tests/desktop.sh
tests/desktop.sh
```

Run the desktop smoke test from a Wayland session with Omarchy and its stock
Hyprland and Quickshell installed. It also uses Bash, Node.js, jq, ripgrep,
grim, wtype, ImageMagick, and Tesseract with English recognition. It starts
its own compositor, shell, and D-Bus session and feeds them synthetic records.
It uses temporary directories and prints the path to the logs and
screenshots if a check fails.

Run `tests/gnome.sh` on GNOME Shell 50 and 51 with the Extensions CLI and
`gnome-shell-test-tool` available. It installs a fresh ZIP in a headless
session and writes only to temporary settings and state paths.

Use each shell's native UI components and theme.
Keep provider-specific data extraction and authentication upstream. Treat
record strings as untrusted display data and render them as plain text. Never
commit credentials, real usage data, or generated state. Run desktop checks
with isolated settings and paths. Use synthetic records when adding schema
coverage.

## Checkpoint commit workflow

This guide authorizes local checkpoint commits during implementation.
Commit on the current branch, including `main`, unless asked otherwise.
Pushing requires a separate request.

Before editing, list small, independently reviewable checkpoints and an
intended commit subject for each. One logical change needs one checkpoint.

1. Make the change with any required tests or documentation.
2. Run focused checks and fix failures. For documentation, review the diff
   and check links you add or change.
3. Stage only the checkpoint's changes and review `git diff --cached`.
4. Commit as soon as the focused checks pass. Finish that commit before
   starting the next checkpoint.

Run the repository checks before reporting completion. Commit follow-up fixes
separately, check `git status`, and list the commits you created. Use short,
imperative subjects such as `fix: keep cached usage on refresh failure`.
Pull requests should describe the behavior and checks and include screenshots
or configuration examples when the widget or integration changes.
