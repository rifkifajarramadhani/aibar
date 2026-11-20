# Working on aibar

aibar is a native Omarchy bar widget. Read [AGENTS.md](AGENTS.md) for repository
rules and checkpoint commits, [the project overview](docs/PROJECT_OVERVIEW.md)
for the record and refresh contracts, and [DESIGN.md](DESIGN.md) for popup
behavior.

Omarchy loads the refresh service once per shell. Bar instances on different
monitors share it. Omarchy's updater and collectors manage credentials and
quota caches and write the usage records. Keep UI changes in
`widget/Panel.qml` and normalization in `widget/Records.js`. Any text read from
a usage record must stay plain text in the popup.

Use `node --test tests/records.test.mjs` for fixture-based checks. Use
`tests/desktop.sh` for changes to loading, theme, keyboard, or popup behavior.
The test creates a separate desktop session with synthetic usage and leaves
your user bar configuration alone.
