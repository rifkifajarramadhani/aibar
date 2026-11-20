# aibar project overview

aibar is the `overhaul.aibar` plugin for Omarchy's stock Quickshell bar. It
answers two questions at a glance. How much of each allowance has been used,
and what was the recent activity for that provider?

## Runtime

[The manifest](../manifest.json) registers one service and one bar widget.
Omarchy owns the service lifecycle. Every monitor's widget looks up the same
service through the bar API, so adding an output adds a button and popup
without adding another collector timer.

[`Usage.qml`](../widget/Usage.qml) runs Omarchy's updater at startup and
every 15 minutes. Requests to open a popup ask for a limits-only run. Manual
refresh asks for a forced run. One update can run at a time. If a wider
refresh is requested while a smaller one runs, the service queues it once.
A five-minute timeout bounds an unresponsive updater.

The service discovers record files before and after refresh and watches known
files for changes. Updates from collectors arrive as atomic replacements.
When one collector exits with an error, the updater keeps that collector's
previous file. aibar reports the failed refresh and marks the affected
provider stale when the updater identifies it. An unreadable replacement
keeps the last valid in-memory record for that file. On successful discovery,
a removed file drops out of the display.

## Record contract

[`Records.js`](../widget/Records.js) validates version 1 records from
`${XDG_STATE_HOME:-$HOME/.local/state}/omarchy/agents/usage/`. A record's ID
must match its filename. Provider discovery comes from those files, including
records for providers added after aibar was released.

- `limits[].percent` is a fraction. `limits[].resetsAt` is the source's reset
  time. After that time passes, the widget keeps the measured percentage
  visible until it receives another reading.
- `tierLabel`, `balance`, `resetCredits`, and `accounts` supply account
  details. Omarchy's updater and account commands own authentication,
  last-good quota caches, and any configured account switching.
- `limitsStale` and `limitsFetchedAt` identify cached limits. The service
  also recognizes collector failures reported by the updater. The panel uses
  the measurement timestamp when available, with the record's `updatedAt`
  as a fallback.
- `recentDays[].messageCount` holds tokens despite its name. History covers
  seven local dates including today. A missing date is blank. A recorded zero
  appears as zero. The panel uses the record date so yesterday's totals
  aren't carried over at midnight.

The upstream record definition is in
[Omarchy's collector reference](https://github.com/omacom/omarchy/blob/quattro/docs/agent-usage-collectors.md#the-record).
The desktop test runs the Omarchy 4.5.0-3 shell with synthetic records and
a stand-in updater.

## Popup

[`Panel.qml`](../widget/Panel.qml) uses Omarchy's `BarIconButton`,
`KeyboardPanel`, and theme tokens. The popup fits the output and keeps
refresh controls above a scrollable list. The panel clock updates countdowns
once a second while open. It performs no file reads or refresh requests on
each tick.

Collector text, including help and account fields, renders as plain text.
The plugin forwards no collector-supplied values into shell commands.
See [DESIGN.md](../DESIGN.md) for the visual and interaction conventions.
