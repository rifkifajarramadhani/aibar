# aibar

aibar shows AI coding usage in the Omarchy bar and the GNOME Shell top bar.
Each provider's limits and reset countdowns come first, followed by today's
activity and a seven-day token history. Plans, balances, and read-only account
details appear when the records include them. Providers with activity but no
quota still get a section.

The initial compatibility baseline is **Omarchy 4.5.0-3** with its stock bar
and bundled Quickshell. Run `pacman -Q omarchy` to check the package version.

## Install on Omarchy

Run these commands from the repository root inside your Omarchy session.

```sh
omarchy plugin validate .
aibar_plugin_dir="$HOME/.config/omarchy/plugins/overhaul.aibar"
install -d "$aibar_plugin_dir"
cp -R manifest.json widget "$aibar_plugin_dir/"
omarchy shell shell rescanPlugins
omarchy plugin disable omarchy.agents
omarchy plugin enable overhaul.aibar --section right
```

If you cloned the stock Agents widget, disable that copy too. aibar and Agents
each schedule the same updater. Keep one enabled to avoid duplicate refreshes.

## Use the Omarchy widget

- Left-click the bar icon to open or close the popup. Middle-click it or select
  **Refresh** in the popup to force a refresh.
- Use the wheel, arrow keys, or Page Up and Page Down to scroll. Home and End
  jump to the beginning or end. The popup starts with **Refresh** focused.
  Press Enter or Space to refresh and Escape to close.
- Run `omarchy shell shell toggle overhaul.aibar` from a keyboard shortcut to
  open the popup on the focused output.

One service per Omarchy shell runs `omarchy-agent-usage-update` on startup and
every 15 minutes. Opening the popup requests `--limits-only`. Manual refresh
passes `--force`. Overlapping requests share the service across monitors.
The countdowns update while the popup is open using the recorded reset times.

The widget reads Omarchy's version 1 records from
`${XDG_STATE_HOME:-$HOME/.local/state}/omarchy/agents/usage/`. Omarchy's
collectors handle sign-in, network requests, and cached usage. Use Omarchy's
agent commands for account management and launching. See the
[Omarchy AI guide](https://omarchy.org/manual/ai/).

## Troubleshoot the Omarchy widget

- If the icon is missing, run `omarchy plugin list` and check that
  `overhaul.aibar` is enabled. Validate the installed plugin folder and run
  `omarchy shell shell rescanPlugins` after copying new files.
- If there are no records, set up a provider in Omarchy, then select
  **Refresh**. To see collector diagnostics in a terminal, run
  `omarchy agent usage-update <provider-id>`, such as
  `omarchy agent usage-update codex`.
- A failed refresh leaves usable readings visible. Check the provider's
  status and last update time in the popup. A malformed record produces a
  read warning while other providers stay visible.

## Install on GNOME Shell

aibar supports GNOME Shell **50** and **51**. The extension reads local version 1
usage records. For live data on a GNOME-only machine, run a separate producer
that writes those records to
`${XDG_STATE_HOME:-$HOME/.local/state}/omarchy/agents/usage/`.
See the [record contract](docs/PROJECT_OVERVIEW.md#record-contract).

From the repository root, use the `gnome-extensions` CLI to build and install
the ZIP. If the CLI cannot find the extension when you enable it, log out and
back in, then run the enable command again.

```sh
integrations/gnome/pack.sh
gnome-extensions install aibar@rifkifajarramadhani.github.io.shell-extension.zip
gnome-extensions enable aibar@rifkifajarramadhani.github.io
```

Open **AI usage** from the right side of the top bar. **Reload records** reads
the files immediately. The extension also reads them at startup, when you open
the menu, and every 30 seconds. With the provider list focused, use the arrow
keys, Page Up, Page Down, Home, or End to scroll. Escape closes the menu.

### Try GNOME with synthetic records

Run `tests/gnome.sh` on a system with GNOME Shell 50 or 51 and
`gnome-shell-test-tool`. The test installs the ZIP in a headless session,
writes synthetic records to a temporary state directory, and checks the menu
and its keyboard controls.

To see the menu yourself from a Wayland session, start a nested GNOME Shell
DevKit window with temporary data, configuration, and state directories.
This example uses Node.js to write the fixtures. Close the DevKit window to
end the test session. [GNOME Shell debugging guide](https://gjs.guide/extensions/development/debugging.html)

```sh
(
  aibar_demo="$(mktemp -d)"
  trap 'rm -rf -- "$aibar_demo"' EXIT
  export XDG_DATA_HOME="$aibar_demo/data" XDG_CONFIG_HOME="$aibar_demo/config"
  export XDG_CACHE_HOME="$aibar_demo/cache" XDG_STATE_HOME="$aibar_demo/state"
  export GSETTINGS_BACKEND=keyfile
  node tests/fixtures.mjs "$XDG_STATE_HOME/omarchy/agents/usage"
  integrations/gnome/pack.sh "$aibar_demo"
  dbus-run-session -- sh -ec '
    gnome-extensions install "$1"
    gsettings set org.gnome.shell enabled-extensions "[\"aibar@rifkifajarramadhani.github.io\"]"
    exec gnome-shell --devkit --wayland-display aibar-demo
  ' sh "$aibar_demo/aibar@rifkifajarramadhani.github.io.shell-extension.zip"
)
```

### Remove the GNOME extension

```sh
gnome-extensions disable aibar@rifkifajarramadhani.github.io
gnome-extensions uninstall aibar@rifkifajarramadhani.github.io
```

## Clean up an earlier Waybar installation

1. Remove the `custom/aibar` definition and any references to it in Waybar's
   module lists. The old snippet placed it in the `modules` list inside
   `group/tray-expander`. Remove the `#custom-aibar` CSS rules and reload
   Waybar if it is still running.
2. Stop any standalone `aibar daemon` process you started. Remove the old
   `aibar` binary from wherever you installed it, often `~/.local/bin/aibar`.
3. Remove any Claude Code `Stop` and `SessionEnd` hook commands that call
   `aibar refresh` from `~/.claude/settings.json`. Keep the other hook
   commands in those lists.
4. After the old daemon stops, its `aibar.sock`, `aibar.pid`, and `state.json`
   under `${XDG_CACHE_HOME:-$HOME/.cache}/aibar/` can be deleted. Check the
   paths you passed if you used `--cache-dir` or `--state`.

## Return to the stock Agents widget

```sh
omarchy plugin disable overhaul.aibar
omarchy plugin enable omarchy.agents --section right
```

Both widgets read the same Omarchy usage records, so the stock widget can use
them immediately. To uninstall aibar's files, run
`omarchy plugin remove overhaul.aibar` after disabling it.

See [the project overview](docs/PROJECT_OVERVIEW.md) for the record and
refresh behavior and [the contributor guide](AGENTS.md) for development checks.
