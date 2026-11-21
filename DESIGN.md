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
popup surface and border built into `KeyboardPanel`. Use the theme's urgent
color for a nearly exhausted allowance or a refresh failure, alongside a
visible number or status label. Meter tracks use `Style.selectedFillFor`
against the popup foreground. The palette follows light and dark themes live.

## Typography

Use the Omarchy bar font and `Style.font` sizes. Use `heading` for the popup
title, `title` for provider names, `body` for subsection headings and primary
readings, and `bodySmall` for supporting text and history rows. Wrap long text
as plain text and right-align numbers in their columns.

Provider headings use the `Style.font.iconLarge` slot and `Style.spacing.lg`
gap. Align the icon with the first line of the name. Load `<id>-light.svg`
from the installed Agents assets on light popup backgrounds when available,
then try `<id>.svg`. Dark backgrounds use `<id>.svg` directly. If neither
candidate loads, center the aibar glyph in the same slot. The icon is
decorative and the provider name supplies the accessible label.

## Layout

Keep the title and refresh action visible while the body scrolls. Use
`Style.spacing.xxl` between providers, `xl` between groups within a provider,
and `xs` to `md` inside a group. Let `KeyboardPanel` size the popup to the
screen. Each history row gives the day, a scaled bar, and the exact token
count. Fit the date and count columns to their text within the available
width. Missing days keep their row with an empty value.

## Elevation & Depth

The shell draws the popup layer and border. Use the shared `PanelSeparator`
between providers.

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
