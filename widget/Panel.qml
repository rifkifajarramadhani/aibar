pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as Controls
import QtQuick.Layouts
import Quickshell
import qs.Commons as Commons
import qs.Ui as Ui
import "Records.mjs" as Records

Ui.Panel {
  id: root

  moduleName: "overhaul.aibar"
  implicitWidth: barButton.implicitWidth
  implicitHeight: barButton.implicitHeight

  readonly property var usage: bar?.shell?.serviceFor(moduleName) ?? null
  readonly property color foreground: Commons.Color.popups.text
  // Secondary text. Dark popups match the stock Agents popup; on light
  // popups darkening would make captions heavier than body text, so they
  // fade toward the background instead.
  readonly property color dim: {
    if (!lightPopup)
      return Qt.darker(foreground, 1.55);
    const back = Commons.Color.popups.background;
    return Qt.rgba(foreground.r * 0.6 + back.r * 0.4, foreground.g * 0.6 + back.g * 0.4, foreground.b * 0.6 + back.b * 0.4, 1);
  }
  readonly property color urgent: Commons.Color.urgent
  readonly property color accent: Commons.Color.accent
  readonly property string fontFamily: bar?.fontFamily || Commons.Style.font.family
  readonly property var locale: Qt.locale(Commons.I18n.formatLocale)
  readonly property string providerAssets: "file://" + Quickshell.env("OMARCHY_PATH") + "/shell/plugins/agents/assets/"
  readonly property bool lightPopup: {
    const color = Commons.Color.popups.background;
    return 0.2126 * linearChannel(color.r) + 0.7152 * linearChannel(color.g) + 0.0722 * linearChannel(color.b) >= 0.5;
  }
  property double now: Date.now()
  readonly property double today: {
    const date = new Date(now);
    return new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
  }

  // Value columns are sized to their widest expected text so every meter in
  // the popup ends at the same place.
  readonly property real percentWidth: Math.ceil(percentMetrics.advanceWidth)
  readonly property real timeWidth: Math.ceil(timeMetrics.advanceWidth)
  readonly property real columnGap: Commons.Style.space(10)
  readonly property real valueWidth: percentWidth + columnGap + timeWidth

  // Match the relative luminance threshold used by the stock Agents popup.
  function linearChannel(channel) {
    return channel <= 0.03928 ? channel / 12.92 : Math.pow((channel + 0.055) / 1.055, 2.4);
  }

  function alpha(color, value) {
    return Qt.rgba(color.r, color.g, color.b, value);
  }

  function formatCount(value) {
    return value.toLocaleString(locale, "f", 0);
  }

  function counted(key, value) {
    return strings.tn(key, value).arg(formatCount(value));
  }

  function money(value, currency) {
    return Commons.I18n.formatNumber(value, 2) + " " + currency;
  }

  function percentText(fraction) {
    return strings.t("percent").arg(formatCount(Math.round(fraction * 100)));
  }

  // The time left before a limit resets, short enough for its column.
  function resetClock(reset) {
    const duration = Records.countdown(reset, now);
    if (!duration)
      return "";
    if (duration.seconds === 0)
      return strings.t("resetNow");
    if (duration.days > 0)
      return strings.t("durationDaysHours").arg(formatCount(duration.days)).arg(formatCount(duration.hours));
    if (duration.hours > 0)
      return strings.t("durationHoursMinutes").arg(formatCount(duration.hours)).arg(formatCount(duration.minutes));
    if (duration.minutes > 0)
      return strings.t("durationMinutesSeconds").arg(formatCount(duration.minutes)).arg(formatCount(duration.seconds % 60));
    return strings.t("durationSeconds").arg(formatCount(duration.seconds));
  }

  function limitDescription(limit) {
    const name = limit.label || strings.t("limit");
    const used = strings.t("percentUsed").arg(formatCount(Math.round(limit.percent * 100)));
    const clock = resetClock(limit.reset);
    if (clock === "")
      return strings.t("limitSummary").arg(name).arg(used);
    if (Records.countdown(limit.reset, now).seconds === 0)
      return strings.t("limitSummary").arg(name).arg(used) + " · " + strings.t("resetPassed");
    return strings.t("limitSummary").arg(name).arg(used) + " · " + strings.t("resetIn").arg(clock);
  }

  function staleText(timestamp) {
    return timestamp === null ? strings.t("staleUnknown") : strings.t("stale").arg(new Date(timestamp).toLocaleString(locale, Locale.ShortFormat));
  }

  function heroMeta() {
    if (!usage)
      return "";
    if (usage.refreshing)
      return strings.t("refreshing");
    const count = usage.providers.length;
    return count > 0 ? counted("providers", count) : "";
  }

  onOpenedChanged: if (opened) {
    now = Date.now();
    if (popup.loadedContent)
      popup.loadedContent.resetScroll();
    if (usage)
      usage.request(usage.limitsOnly);
  }

  Timer {
    interval: 1000
    running: root.opened
    repeat: true
    onTriggered: root.now = Date.now()
  }

  Commons.Translations {
    id: strings
    directory: Qt.resolvedUrl("i18n")
  }

  TextMetrics {
    id: percentMetrics
    font.family: root.fontFamily
    font.pixelSize: Commons.Style.font.bodySmall
    font.bold: true
    text: "100%"
  }

  TextMetrics {
    id: timeMetrics
    font.family: root.fontFamily
    font.pixelSize: Commons.Style.font.bodySmall
    text: "00m 00s"
  }

  Ui.BarIconButton {
    id: barButton
    anchors.fill: parent
    bar: root.bar
    text: "󱚣"
    tooltipText: strings.t("title")
    active: root.usage?.refreshFailed ?? false
    Accessible.role: Accessible.Button
    Accessible.name: strings.t("title")
    Accessible.onPressAction: root.toggle()
    onPressed: button => {
      if (button === Qt.LeftButton)
        root.toggle();
      if (button === Qt.MiddleButton && root.usage)
        root.usage.request(root.usage.manual);
    }
  }

  Ui.KeyboardPanel {
    id: popup
    anchorItem: barButton
    owner: root
    bar: root.bar
    open: root.opened
    focusTarget: loadedContent?.refreshButton ?? null
    contentWidth: fittedContentWidth(Commons.Style.space(380))
    contentHeight: fittedContentHeight(loadedContentHeight, Commons.Style.space(860))

    content: FocusScope {
      anchors.fill: parent
      property alias refreshButton: refresh
      implicitHeight: header.implicitHeight + popupLayout.spacing + sections.implicitHeight

      function resetScroll() {
        scroll.contentY = 0;
      }

      Keys.onEscapePressed: root.close()
      Keys.onUpPressed: scroll.step(-Commons.Style.space(56))
      Keys.onDownPressed: scroll.step(Commons.Style.space(56))
      Keys.onPressed: event => {
        switch (event.key) {
        case Qt.Key_PageUp:
          scroll.step(-scroll.height);
          break;
        case Qt.Key_PageDown:
          scroll.step(scroll.height);
          break;
        case Qt.Key_Home:
          scroll.contentY = 0;
          break;
        case Qt.Key_End:
          scroll.contentY = scroll.maximumY;
          break;
        default:
          return;
        }
        event.accepted = true;
      }

      ColumnLayout {
        id: popupLayout
        anchors.fill: parent
        spacing: Commons.Style.space(16)

        // Hero: glyph, title, status caption, and refresh, as in the stock
        // Agents popup. It stays put while the providers scroll.
        ColumnLayout {
          id: header
          Layout.fillWidth: true
          spacing: Commons.Style.space(10)

          RowLayout {
            Layout.fillWidth: true
            spacing: 0

            Text {
              textFormat: Text.PlainText
              text: barButton.text
              color: root.foreground
              font.family: root.fontFamily
              font.pixelSize: Commons.Style.font.display
              Accessible.ignored: true
            }

            ColumnLayout {
              Layout.fillWidth: true
              Layout.leftMargin: Commons.Style.space(14)
              Layout.rightMargin: Commons.Style.space(12)
              Layout.alignment: Qt.AlignVCenter
              spacing: Commons.Style.space(2)

              Label {
                Layout.fillWidth: true
                text: strings.t("title")
                font.pixelSize: Commons.Style.font.title
                font.bold: true
                wrapMode: Text.NoWrap
                elide: Text.ElideRight
              }

              Label {
                Layout.fillWidth: true
                visible: text !== ""
                text: root.heroMeta().toUpperCase()
                color: root.dim
                font.pixelSize: Commons.Style.font.caption
                font.bold: true
                font.letterSpacing: 1.2
                wrapMode: Text.NoWrap
                elide: Text.ElideRight
              }
            }

            Ui.Button {
              id: refresh
              implicitWidth: Commons.Style.space(34)
              implicitHeight: implicitWidth
              horizontalPadding: 0
              verticalPadding: 0
              iconText: "󰑐"
              iconSize: Commons.Style.font.heading
              iconSpinning: root.usage?.refreshing ?? false
              tooltipText: strings.t("refresh")
              foreground: root.accent
              background: root.alpha(root.accent, 0.12)
              fontFamily: root.fontFamily
              focusable: true
              enabled: root.usage !== null
              Accessible.role: Accessible.Button
              Accessible.name: tooltipText
              Accessible.onPressAction: clicked()
              onClicked: if (root.usage)
                root.usage.request(root.usage.manual)
            }
          }

          // Failures stay readable as sentences, not only as hero caps.
          Caption {
            Layout.fillWidth: true
            visible: root.usage?.refreshFailed ?? false
            color: root.urgent
            text: strings.t("refreshFailed")
          }

          Caption {
            Layout.fillWidth: true
            visible: root.usage?.invalidRecords ?? false
            color: root.urgent
            text: strings.t("invalidRecords")
          }
        }

        Flickable {
          id: scroll
          Layout.fillWidth: true
          Layout.fillHeight: true
          Layout.minimumHeight: 0
          clip: true
          contentWidth: width
          contentHeight: sections.implicitHeight
          boundsBehavior: Flickable.StopAtBounds
          flickableDirection: Flickable.VerticalFlick
          interactive: maximumY > 0
          readonly property real maximumY: Math.max(0, contentHeight - height)

          function step(delta) {
            contentY = Math.max(0, Math.min(maximumY, contentY + delta));
          }

          Controls.ScrollBar.vertical: Controls.ScrollBar {
            id: scrollbar
            policy: Controls.ScrollBar.AsNeeded
            width: Commons.Style.space(6)
            padding: 0
            minimumSize: 0.08
            contentItem: Rectangle {
              visible: scrollbar.size < 1
              implicitWidth: scrollbar.width
              radius: width / 2
              color: root.foreground
              opacity: 0.45
            }
          }

          ColumnLayout {
            id: sections
            // Only a scrolling list gives up a strip to its scroll bar.
            readonly property real scrollbarSpace: scroll.interactive ? scrollbar.width + Commons.Style.space(6) : 0
            x: LayoutMirroring.enabled ? scrollbarSpace : 0
            width: Math.max(0, scroll.width - scrollbarSpace)
            spacing: Commons.Style.space(16)

            Label {
              Layout.fillWidth: true
              visible: providerRows.count === 0
              text: root.usage?.refreshing ? strings.t("emptyLoading") : strings.t("empty")
            }

            Repeater {
              id: providerRows
              model: root.usage?.providers ?? []

              ColumnLayout {
                id: section
                required property int index
                required property var modelData
                readonly property var provider: modelData.record
                readonly property bool failed: root.usage?.failedProviders[provider.id] === true
                readonly property bool updatedToday: provider.updated !== null && provider.updated >= root.today
                readonly property var days: Records.history(provider, root.today)
                readonly property double peak: Math.max(1, ...days.map(day => day.tokens ?? 0))

                Layout.fillWidth: true
                spacing: Commons.Style.space(16)

                Ui.PanelSeparator {
                  Layout.fillWidth: true
                  foreground: root.foreground
                }

                RowLayout {
                  Layout.fillWidth: true
                  spacing: Commons.Style.space(10)

                  ProviderIcon {
                    Layout.alignment: Qt.AlignTop
                    providerId: section.provider.id
                  }

                  Label {
                    Layout.fillWidth: true
                    text: section.provider.name
                    font.bold: true
                    Accessible.role: Accessible.StaticText
                    Accessible.name: text
                  }

                  Caption {
                    Layout.alignment: Qt.AlignTop
                    Layout.topMargin: Math.max(0, (Commons.Style.font.body - Commons.Style.font.caption) / 2)
                    Layout.maximumWidth: section.width * 0.35
                    visible: text !== ""
                    text: section.provider.plan
                    wrapMode: Text.NoWrap
                    elide: Text.ElideRight
                    horizontalAlignment: Text.AlignRight
                  }
                }

                Details {
                  Layout.fillWidth: true
                  dataItem: section.provider
                  sourceFailed: section.failed || section.modelData.unreadable
                  fallbackTime: section.provider.updated
                }

                Repeater {
                  model: section.provider.accounts
                  ColumnLayout {
                    id: account
                    required property var modelData
                    Layout.fillWidth: true
                    spacing: Commons.Style.space(12)

                    RowLayout {
                      Layout.fillWidth: true
                      spacing: Commons.Style.space(8)

                      Label {
                        Layout.maximumWidth: account.width * 0.5
                        text: account.modelData.name
                        font.bold: account.modelData.active
                        wrapMode: Text.NoWrap
                        elide: Text.ElideRight
                      }
                      Caption {
                        Layout.fillWidth: true
                        text: [account.modelData.email, account.modelData.plan].filter(part => part !== "").join(" · ")
                        wrapMode: Text.NoWrap
                        elide: Text.ElideRight
                      }
                      Caption {
                        visible: account.modelData.active
                        text: strings.t("active").toUpperCase()
                        color: root.foreground
                        font.bold: true
                        font.letterSpacing: 1.2
                        wrapMode: Text.NoWrap
                      }
                    }

                    Details {
                      Layout.fillWidth: true
                      dataItem: account.modelData
                      sourceFailed: section.failed || section.modelData.unreadable
                      fallbackTime: section.provider.updated
                    }
                  }
                }

                Caption {
                  Layout.fillWidth: true
                  visible: !section.provider.hasActivity && !section.provider.limits.length && !section.provider.balance && !section.provider.accounts.length && !section.provider.status && !section.provider.help
                  text: strings.t("noDetails")
                }

                ColumnLayout {
                  Layout.fillWidth: true
                  visible: section.provider.hasActivity
                  spacing: Commons.Style.space(6)

                  SectionHeading {
                    Layout.fillWidth: true
                    text: strings.t("today")
                  }

                  SmallLabel {
                    Layout.fillWidth: true
                    text: {
                      const readings = [];
                      const todayTokens = section.days[6].tokens;
                      if (todayTokens !== null)
                        readings.push(root.counted("tokens", todayTokens));
                      if (section.updatedToday && section.provider.todayPrompts !== null)
                        readings.push(root.counted("prompts", section.provider.todayPrompts));
                      if (section.updatedToday && section.provider.todaySessions !== null)
                        readings.push(root.counted("sessions", section.provider.todaySessions));
                      return readings.length ? readings.join(" · ") : strings.t("unavailable");
                    }
                  }
                }

                ColumnLayout {
                  Layout.fillWidth: true
                  visible: section.provider.hasActivity
                  spacing: Commons.Style.space(8)

                  SectionHeading {
                    Layout.fillWidth: true
                    text: strings.t("history")
                  }

                  Repeater {
                    model: section.provider.hasActivity ? section.days : []
                    delegate: ValueRow {
                      id: dayRow
                      required property var modelData
                      readonly property string fullDate: modelData.date.toLocaleDateString(root.locale, Locale.LongFormat)
                      Layout.fillWidth: true
                      label: modelData.date.toLocaleDateString(root.locale, "ddd")
                      fraction: (modelData.tokens ?? 0) / section.peak
                      value: modelData.tokens === null ? strings.t("noData") : root.formatCount(modelData.tokens)
                      valueColor: root.dim
                      valueBold: false
                      description: modelData.tokens === null ? strings.t("noHistoryForDate").arg(fullDate) : strings.t("historyForDate").arg(fullDate).arg(root.counted("tokens", modelData.tokens))
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
  }

  component Label: Text {
    textFormat: Text.PlainText
    wrapMode: Text.Wrap
    color: root.foreground
    font.family: root.fontFamily
    font.pixelSize: Commons.Style.font.body
  }

  component SmallLabel: Label {
    font.pixelSize: Commons.Style.font.bodySmall
  }

  component Caption: Label {
    color: root.dim
    font.pixelSize: Commons.Style.font.caption
  }

  component SectionHeading: Ui.PanelSectionHeader {
    foreground: root.foreground
    fontFamily: root.fontFamily
    wrapMode: Text.Wrap
  }

  component ProviderIcon: Item {
    id: icon
    required property string providerId
    readonly property string asset: root.providerAssets + providerId

    implicitWidth: Commons.Style.font.heading
    implicitHeight: implicitWidth
    Accessible.ignored: true

    ProviderLogo {
      id: lightLogo
      source: root.lightPopup ? icon.asset + "-light.svg" : ""
    }

    ProviderLogo {
      id: defaultLogo
      source: !root.lightPopup || lightLogo.status === Image.Error ? icon.asset + ".svg" : ""
    }

    Ui.OpticalGlyph {
      anchors.fill: parent
      visible: lightLogo.status !== Image.Ready && defaultLogo.status !== Image.Ready
      text: barButton.text
      color: root.foreground
      fontFamily: root.fontFamily
      fontSize: icon.implicitWidth
      paintedSize: icon.implicitWidth
      Accessible.ignored: true
    }
  }

  component ProviderLogo: Image {
    anchors.fill: parent
    fillMode: Image.PreserveAspectFit
    sourceSize.width: width * 2
    sourceSize.height: height * 2
    Accessible.ignored: true
  }

  component Meter: Item {
    id: meter
    required property real fraction
    property bool alarming: false

    implicitHeight: Math.max(Commons.Style.space(4), Math.round(Commons.Style.spacing.controlHeight * 0.14))

    Rectangle {
      anchors.fill: parent
      color: Commons.Style.selectedFillFor(root.foreground, root.accent)
      radius: height / 2
    }
    Rectangle {
      anchors.left: parent.left
      height: parent.height
      width: parent.width * Math.max(0, Math.min(meter.fraction, 1))
      radius: height / 2
      color: meter.alarming ? root.urgent : root.foreground
      Behavior on width {
        NumberAnimation {
          duration: Commons.Style.duration(160)
          easing.type: Easing.OutCubic
        }
      }
    }
  }

  // One reading per line: a label in the first 30%, a meter, and values in
  // fixed-width columns on the right. `time` reserves the countdown column
  // whenever `reserveTime` is set, so limit meters line up with or without
  // a reset; rows without it give the whole value area to `value`.
  component ValueRow: Item {
    id: row
    property string label: ""
    property real fraction: 0
    property bool showMeter: true
    property bool alarming: false
    property string value: ""
    property color valueColor: alarming ? root.urgent : root.foreground
    property bool valueBold: true
    property string time: ""
    property bool reserveTime: false
    property string description: ""

    // Anchored rather than laid out: widths here derive from the row's own
    // width, which a RowLayout would feed back into its implicit size.
    implicitHeight: Math.max(rowLabel.implicitHeight, rowValue.implicitHeight)
    Accessible.role: Accessible.StaticText
    Accessible.name: description

    HoverHandler {
      id: rowHover
    }

    Ui.PanelToolTip {
      visible: rowHover.hovered && row.description !== ""
      implicitWidth: Math.min(implicitContentWidth + leftPadding + rightPadding, Commons.Style.space(320))
      text: row.description
    }

    SmallLabel {
      id: rowLabel
      anchors.left: parent.left
      anchors.verticalCenter: parent.verticalCenter
      width: Math.round(row.width * 0.3)
      text: row.label
      wrapMode: Text.NoWrap
      elide: Text.ElideRight
    }
    Meter {
      anchors.left: rowLabel.right
      anchors.leftMargin: Commons.Style.space(8)
      anchors.right: rowValue.left
      anchors.rightMargin: Commons.Style.space(16)
      anchors.verticalCenter: parent.verticalCenter
      fraction: row.fraction
      alarming: row.alarming
      opacity: row.showMeter ? 1 : 0
    }
    SmallLabel {
      id: rowValue
      anchors.right: rowTime.visible ? rowTime.left : parent.right
      anchors.rightMargin: rowTime.visible ? root.columnGap : 0
      anchors.verticalCenter: parent.verticalCenter
      // A long amount may push its meter in rather than lose digits.
      width: row.reserveTime ? root.percentWidth : Math.min(row.width * 0.45, Math.max(root.valueWidth, implicitWidth))
      text: row.value
      color: row.valueColor
      font.bold: row.valueBold
      wrapMode: Text.NoWrap
      elide: Text.ElideLeft
      horizontalAlignment: Text.AlignRight
    }
    SmallLabel {
      id: rowTime
      visible: row.reserveTime
      anchors.right: parent.right
      anchors.verticalCenter: parent.verticalCenter
      width: root.timeWidth
      text: row.time
      color: root.dim
      wrapMode: Text.NoWrap
      elide: Text.ElideRight
      horizontalAlignment: Text.AlignRight
    }
  }

  component Details: ColumnLayout {
    id: details
    required property var dataItem
    required property bool sourceFailed
    required property var fallbackTime
    visible: dataItem.limits.length > 0 || dataItem.balance !== null || dataItem.resetCredits > 0 || sourceFailed || dataItem.stale || dataItem.status !== "" || dataItem.help !== ""
    spacing: Commons.Style.space(12)

    Repeater {
      model: details.dataItem.limits
      ValueRow {
        required property var modelData
        Layout.fillWidth: true
        label: modelData.label || strings.t("limit")
        fraction: modelData.percent
        alarming: modelData.percent >= 0.9
        value: root.percentText(modelData.percent)
        time: root.resetClock(modelData.reset)
        reserveTime: true
        description: root.limitDescription(modelData)
      }
    }

    // A prepaid balance drains toward empty instead of filling toward a cap.
    ColumnLayout {
      readonly property var balance: details.dataItem.balance
      readonly property bool funded: balance !== null && balance.funded > 0
      readonly property bool low: funded && balance.remaining / balance.funded <= 0.1
      Layout.fillWidth: true
      visible: balance !== null
      spacing: Commons.Style.space(6)

      ValueRow {
        Layout.fillWidth: true
        label: strings.t("balance")
        showMeter: parent.funded
        fraction: parent.funded ? parent.balance.remaining / parent.balance.funded : 0
        alarming: parent.low
        value: parent.balance ? root.money(parent.balance.remaining, parent.balance.currency) : ""
        description: parent.balance ? strings.t("remaining").arg(value) : ""
      }
      Caption {
        Layout.fillWidth: true
        visible: text !== ""
        text: {
          const balance = parent.balance;
          return balance && balance.spent !== null && balance.funded !== null ? strings.t(balance.estimated ? "estimatedSpent" : "spent").arg(root.money(balance.spent, balance.currency)).arg(root.money(balance.funded, balance.currency)) : "";
        }
      }
    }

    ColumnLayout {
      Layout.fillWidth: true
      visible: resetCredits.visible || staleNote.visible || statusNote.visible || helpNote.visible
      spacing: Commons.Style.space(4)

      Caption {
        id: resetCredits
        Layout.fillWidth: true
        visible: details.dataItem.resetCredits > 0
        text: visible ? root.counted("resetCredits", details.dataItem.resetCredits) : ""
      }

      Caption {
        id: staleNote
        Layout.fillWidth: true
        visible: details.sourceFailed || details.dataItem.stale
        text: visible ? root.staleText(details.dataItem.fetched ?? details.fallbackTime) : ""
      }

      SmallLabel {
        id: statusNote
        Layout.fillWidth: true
        visible: text !== ""
        color: root.urgent
        text: details.dataItem.status
      }

      Caption {
        id: helpNote
        Layout.fillWidth: true
        visible: text !== ""
        text: details.dataItem.help
      }
    }
  }
}
