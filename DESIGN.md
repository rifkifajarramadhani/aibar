---
version: alpha
name: aibar
description: AI usage in the Omarchy and GNOME Shell top bars
omitted:
  - section: colors
    reason: "Palette comes from the active desktop shell theme at runtime"
  - section: typography
    reason: "Each desktop shell supplies the base font and scale"
  - section: rounded
    reason: "Each desktop shell owns native corners and borders"
  - section: spacing
    reason: "Omarchy Style and GNOME Shell font-relative spacing scale the layout"
components:
  bar-button: {}
  popup: {}
  refresh-button: {}
  usage-meter: {}
  history-row: {}
---

# aibar design

## Overview

aibar is for people checking how much coding-agent allowance they have left.
It opens from the Omarchy or GNOME Shell bar. Each provider section works top
to bottom. Limits and balances come first, followed by today's activity and
seven days of tokens. Account details stay with the allowance they describe.

The shell supplies the active locale, including number and date formats.
aibar ships English copy and uses Omarchy's catalog fallback for other
languages. Provider names and status details come from the records.

## Colors

Use the shell popup foreground from `Commons.Color.popups.text` and the
popup surface and border built into `KeyboardPanel`. Secondary text uses a
dim tone: `Qt.darker(foreground, 1.55)` on dark popups, as the stock Agents
popup does, and the foreground faded 40% toward the background on light
popups so captions never outweigh body text. Use the theme's urgent color for
a limit at 90% or more, a balance at 10% or less, a provider status, and a
refresh failure, always alongside a visible number or sentence. The refresh
button uses an accent glyph on a 12% accent tint. Meter tracks use
`Style.selectedFillFor` against the popup foreground. The palette follows
light and dark themes live.

## Typography

Use the Omarchy bar font and `Style.font` sizes, following the stock Agents
popup. The hero uses the bar glyph at `display`, the title at `title` bold,
and an uppercase `caption` bold status line with 1.2 letter spacing in the
dim tone. Provider names use `body` bold. Reading rows, today's readings and
history use `bodySmall`. Plans, notes, help and `PanelSectionHeader`
subheadings use `caption`. Percentages are bold. Record text is plain text.

Provider marks use the `Style.font.heading` slot with a `space(10)` gap.
Align the mark with the first line of the name. Load `<id>-light.svg` from
the installed Agents assets on light popup backgrounds when available, then
try `<id>.svg`. Dark backgrounds use `<id>.svg` directly. If neither
candidate loads, center the aibar glyph in the same slot. The mark is
decorative and the provider name supplies the accessible label.

## Layout

The popup is `space(380)` wide and at most `space(860)` tall. The hero and
any failure sentences stay above the scrolling providers. Use `space(16)`
between the hero and the list, between providers, and between groups inside
a provider. Every provider starts with a `PanelSeparator`, then a header row
with the mark, the name, and the plan right-aligned in a dim caption. Long
names wrap.

Each limit, balance and history day is one row. The label takes the first
30% of the width and elides, the meter fills the middle, and the values sit
in fixed columns on the right. Limits show the bold percentage, then the time
left in the dim tone. The columns are measured from `100%` and `00m 00s` so
every meter ends at the same place. Balance and history values use both
columns. Limit rows are `space(12)` apart and history rows `space(8)` apart.
Accounts show their name, then email and plan in a dim caption, then ACTIVE
on the right, and their limits below with no indent.

## Elevation & Depth

The shell draws the popup layer and border. Use the shared `PanelSeparator`
above each provider.

## Shapes

Use native bar and button shapes. Meters use the rounded track from the stock
Agents design. The refresh button is a `space(34)` square with
`Style.cornerRadius`.

## Components

`BarIconButton` opens the popup. Refresh is the shared `Button` with an icon,
a tooltip, a focus ring, and keyboard activation. It spins while a refresh
runs. Escape closes the popup and arrow keys scroll the body. `KeyboardPanel`
owns popup motion. Meter animation uses `Style.duration` so it follows the
shell's motion setting.

The stock Agents popup puts the percentage in a tooltip. aibar keeps it in
its own column, because exact numbers must be visible without hover. Hovering
a row shows the full label, the percentage used, and the reset time. The same
text is the row's accessible name.

A refresh leaves usable readings visible. A failure is a sentence under the
hero, and provider sections mark cached readings with their available update
time. Zero usage has a number. Missing usage has a distinct empty value.

## Do's and Don'ts

- Keep status and exact numbers visible without hover.
- Bind collector-supplied text to `Text.PlainText`.
- Use Omarchy's existing controls for popup and button behavior.

## GNOME Shell menu

Use GNOME Shell's panel button, popup, button and scrollbar styles. Keep
the panel icon at the shell size. Relative to the menu font, use `1.2em` for
the popup title, `1.1em` for provider names, `1em` for section headings and
readings, and `0.9em` for supporting text. Use `1.15em` between providers,
`0.75em` between groups, and `0.25em` between related rows.

Keep **Reload records** in the header above the scrollable provider list.
The list has one keyboard focus stop and accepts the arrow keys, page keys,
Home and End. Let long labels wrap and align readings to the right. Use an
em dash for a missing token count and a number for a recorded zero. Use a
visible status line for a folder or file read failure and mark retained
readings in their provider section. Render record text as plain text.
