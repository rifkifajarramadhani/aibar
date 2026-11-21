---
version: alpha
name: aibar
description: AI usage in the native Omarchy bar
omitted:
  - section: colors
    reason: "Palette comes from the active Omarchy theme at runtime"
  - section: typography
    reason: "The Omarchy shell owns font families and sizes"
  - section: rounded
    reason: "Omarchy owns corners and borders"
  - section: spacing
    reason: "Omarchy Style scales layout and control spacing"
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
It opens from the Omarchy bar and uses the same controls and spacing as the
stock Agents widget. Each provider section works top to bottom. Limits and
balances come first, followed by today's activity and seven days of tokens.
Account details stay with the allowance they describe.

The shell supplies the active locale, including number and date formats.
aibar ships English copy and uses Omarchy's catalog fallback for other
languages. Provider names and status details come from the records.

## Colors

Use the shell popup foreground from `Commons.Color.popups.text` and the
popup surface and border built into `KeyboardPanel`. Use the theme's urgent
color for a nearly exhausted allowance or a refresh failure, alongside a
visible number or status label. Meter tracks use `Style.selectedFillFor`
against the popup foreground. The palette follows light and dark themes live.

## Typography

Use the Omarchy bar font and `Style.font` sizes. Provider names, section
labels, and readings use the same family. Long provider text wraps as plain
text. History counts stay right-aligned for comparison.

Provider headings use the stock Agents icon slot and gap, sized with
`Style.font.heading` and `Style.space(10)`. Load `<id>-light.svg` from the
installed Agents assets on light popup backgrounds when available, then try
`<id>.svg`. Dark backgrounds use `<id>.svg` directly. If neither candidate
loads, use the aibar glyph in the same slot. The icon is decorative and
the provider name supplies the accessible label.

## Layout

Keep the title and refresh action visible while the body scrolls. Use
`Style.space` and `Style.spacing` for the gaps. Let `KeyboardPanel` size
the popup to the screen. Each history row gives the day, a scaled bar, and
the exact token count. Missing days keep their row with an empty value.

## Elevation & Depth

The shell draws the popup layer and border. Flat sections use the shared
`PanelSeparator` to mark each provider.

## Shapes

Use native bar and button shapes. Meters use the rounded track from the stock
Agents design, with an adjacent percentage and reset countdown.

## Components

`BarIconButton` opens the popup. The shared `Button` shows a focus ring and
handles keyboard activation for refresh. Escape closes the popup and arrow
keys scroll the body. `KeyboardPanel` owns popup motion. Meter animation uses
`Style.duration` so it follows the shell's motion setting.

A refresh leaves usable readings visible. Its status line reports failures,
and provider sections mark cached readings with their available update time.
Zero usage has a number. Missing usage has a distinct empty value.

## Do's and Don'ts

- Keep status and exact numbers visible without hover.
- Bind collector-supplied text to `Text.PlainText`.
- Use Omarchy's existing controls for popup and button behavior.
