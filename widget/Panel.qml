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
  readonly property color urgent: Commons.Color.urgent
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

  // Match the relative luminance threshold used by the stock Agents popup.
  function linearChannel(channel) {
    return channel <= 0.03928 ? channel / 12.92 : Math.pow((channel + 0.055) / 1.055, 2.4);
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

  function resetText(reset) {
    const duration = Records.countdown(reset, now);
    if (!duration)
      return "";
    if (duration.seconds === 0)
      return strings.t("resetPassed");
    let clock;
    if (duration.days > 0)
      clock = strings.t("durationDaysHours").arg(formatCount(duration.days)).arg(formatCount(duration.hours));
    else if (duration.hours > 0)
      clock = strings.t("durationHoursMinutes").arg(formatCount(duration.hours)).arg(formatCount(duration.minutes));
    else if (duration.minutes > 0)
      clock = strings.t("durationMinutesSeconds").arg(formatCount(duration.minutes)).arg(formatCount(duration.seconds % 60));
    else
      clock = strings.t("durationSeconds").arg(formatCount(duration.seconds));
    return strings.t("resetIn").arg(clock);
  }

  function staleText(timestamp) {
    return timestamp === null ? strings.t("staleUnknown") : strings.t("stale").arg(new Date(timestamp).toLocaleString(locale, Locale.ShortFormat));
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
    contentWidth: fittedContentWidth(Commons.Style.space(424))
    contentHeight: fittedContentHeight(loadedContentHeight, Commons.Style.space(720))

    content: FocusScope {
      anchors.fill: parent
      property alias refreshButton: refresh
      implicitHeight: header.implicitHeight + popupLayout.spacing + sections.implicitHeight

      function resetScroll() {
        scroll.contentY = 0;
      }

      Keys.onEscapePressed: root.close()
      Keys.onUpPressed: scroll.step(-Commons.Style.space(44))
      Keys.onDownPressed: scroll.step(Commons.Style.space(44))
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
        spacing: Commons.Style.spacing.xl

        ColumnLayout {
          id: header
          Layout.fillWidth: true
          spacing: Commons.Style.spacing.sm

          RowLayout {
            Layout.fillWidth: true
            spacing: Commons.Style.spacing.lg

            Label {
              Layout.fillWidth: true
              text: strings.t("title")
              font.pixelSize: Commons.Style.font.heading
              font.bold: true
            }
            Ui.Button {
              id: refresh
              text: strings.t("refresh")
              foreground: root.foreground
              fontFamily: root.fontFamily
              focusable: true
              enabled: root.usage !== null
              Accessible.role: Accessible.Button
              Accessible.name: text
              Accessible.onPressAction: clicked()
              onClicked: if (root.usage)
                root.usage.request(root.usage.manual)
            }
          }

          SupportingLabel {
            Layout.fillWidth: true
            visible: root.usage?.refreshing ?? false
            text: strings.t("refreshing")
          }

          SupportingLabel {
            Layout.fillWidth: true
            visible: root.usage?.refreshFailed ?? false
            color: root.urgent
            text: strings.t("refreshFailed")
          }

          SupportingLabel {
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
            readonly property real scrollbarSpace: scrollbar.width + Commons.Style.spacing.sm
            x: LayoutMirroring.enabled ? scrollbarSpace : 0
            width: Math.max(0, scroll.width - scrollbarSpace)
            spacing: Commons.Style.spacing.xxl

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
                spacing: Commons.Style.spacing.xl

                Ui.PanelSeparator {
                  Layout.fillWidth: true
                  visible: section.index > 0
                  foreground: root.foreground
                }

                RowLayout {
                  Layout.fillWidth: true
                  spacing: Commons.Style.spacing.lg

                  ProviderIcon {
                    Layout.alignment: Qt.AlignTop
                    providerId: section.provider.id
                  }

                  Label {
                    Layout.fillWidth: true
                    text: section.provider.name
                    font.pixelSize: Commons.Style.font.title
                    font.bold: true
                    Accessible.role: Accessible.StaticText
                    Accessible.name: text
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
                    Layout.leftMargin: Commons.Style.spacing.md
                    spacing: Commons.Style.spacing.xs

                    Label {
                      Layout.fillWidth: true
                      text: account.modelData.name
                      font.bold: true
                    }
                    SupportingLabel {
                      Layout.fillWidth: true
                      visible: account.modelData.active
                      text: strings.t("active")
                    }
                    SupportingLabel {
                      Layout.fillWidth: true
                      visible: text !== ""
                      text: account.modelData.email
                    }
                    Details {
                      Layout.fillWidth: true
                      dataItem: account.modelData
                      sourceFailed: section.failed || section.modelData.unreadable
                      fallbackTime: section.provider.updated
                    }
                  }
                }

                SupportingLabel {
                  Layout.fillWidth: true
                  visible: !section.provider.hasActivity && !section.provider.limits.length && !section.provider.balance && !section.provider.accounts.length && !section.provider.status && !section.provider.help
                  text: strings.t("noDetails")
                }

                ColumnLayout {
                  Layout.fillWidth: true
                  visible: section.provider.hasActivity
                  spacing: Commons.Style.spacing.xs

                  SectionHeading {
                    Layout.fillWidth: true
                    text: strings.t("today")
                  }

                  Label {
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
                  id: historyGroup
                  readonly property var dateLabels: section.days.map(day => day.date.toLocaleDateString(root.locale, "ddd"))
                  readonly property var countLabels: section.days.map(day => day.tokens === null ? strings.t("noData") : root.formatCount(day.tokens))
                  readonly property real dateWidth: Math.min(width * 0.24, dateMeasure.implicitWidth)
                  readonly property real countWidth: Math.min(width * 0.38, countMeasure.implicitWidth)
                  Layout.fillWidth: true
                  visible: section.provider.hasActivity
                  spacing: Commons.Style.spacing.xs

                  // Measure the widest label in each column using the displayed font.
                  SupportingLabel {
                    id: dateMeasure
                    visible: false
                    wrapMode: Text.NoWrap
                    text: historyGroup.dateLabels.join("\n")
                  }

                  SupportingLabel {
                    id: countMeasure
                    visible: false
                    wrapMode: Text.NoWrap
                    text: historyGroup.countLabels.join("\n")
                  }

                  SectionHeading {
                    Layout.fillWidth: true
                    text: strings.t("history")
                  }

                  Repeater {
                    model: section.provider.hasActivity ? section.days : []
                    delegate: RowLayout {
                      id: dayRow
                      required property int index
                      required property var modelData
                      Layout.fillWidth: true
                      spacing: Commons.Style.spacing.lg
                      readonly property string fullDate: modelData.date.toLocaleDateString(root.locale, Locale.LongFormat)
                      Accessible.role: Accessible.StaticText
                      Accessible.name: modelData.tokens === null ? strings.t("noHistoryForDate").arg(fullDate) : strings.t("historyForDate").arg(fullDate).arg(root.counted("tokens", modelData.tokens))

                      SupportingLabel {
                        Layout.preferredWidth: historyGroup.dateWidth
                        text: historyGroup.dateLabels[dayRow.index]
                      }
                      Meter {
                        Layout.fillWidth: true
                        fraction: (dayRow.modelData.tokens ?? 0) / section.peak
                      }
                      SupportingLabel {
                        Layout.preferredWidth: historyGroup.countWidth
                        text: historyGroup.countLabels[dayRow.index]
                        horizontalAlignment: Text.AlignRight
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
  }

  component Label: Text {
    textFormat: Text.PlainText
    wrapMode: Text.Wrap
    color: root.foreground
    font.family: root.fontFamily
    font.pixelSize: Commons.Style.font.body
  }

  component SupportingLabel: Label {
    font.pixelSize: Commons.Style.font.bodySmall
  }

  component SectionHeading: Ui.PanelSectionHeader {
    color: root.foreground
    fontFamily: root.fontFamily
    fontSize: Commons.Style.font.body
    wrapMode: Text.Wrap
  }

  component ProviderIcon: Item {
    id: icon
    required property string providerId
    readonly property string asset: root.providerAssets + providerId

    implicitWidth: Commons.Style.font.iconLarge
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
      color: Commons.Style.selectedFillFor(root.foreground, Commons.Color.accent)
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

  component Details: ColumnLayout {
    id: details
    required property var dataItem
    required property bool sourceFailed
    required property var fallbackTime
    visible: dataItem.plan !== "" || dataItem.limits.length > 0 || dataItem.balance !== null || dataItem.resetCredits > 0 || sourceFailed || dataItem.stale || dataItem.status !== "" || dataItem.help !== ""
    spacing: Commons.Style.spacing.md

    SupportingLabel {
      Layout.fillWidth: true
      visible: text !== ""
      text: details.dataItem.plan
    }

    Repeater {
      model: details.dataItem.limits
      ColumnLayout {
        id: limit
        required property var modelData
        Layout.fillWidth: true
        spacing: Commons.Style.spacing.xs

        RowLayout {
          Layout.fillWidth: true
          spacing: Commons.Style.spacing.lg

          Label {
            Layout.fillWidth: true
            text: limit.modelData.label || strings.t("limit")
          }
          Label {
            Layout.alignment: Qt.AlignTop
            Layout.maximumWidth: popup.contentWidth * 0.42
            text: strings.t("percentUsed").arg(root.formatCount(Math.round(limit.modelData.percent * 100)))
            font.bold: true
            horizontalAlignment: Text.AlignRight
          }
        }
        Meter {
          Layout.fillWidth: true
          fraction: limit.modelData.percent
          alarming: fraction >= 0.9
        }
        SupportingLabel {
          Layout.fillWidth: true
          visible: text !== ""
          text: root.resetText(limit.modelData.reset)
        }
      }
    }

    ColumnLayout {
      readonly property var balance: details.dataItem.balance
      Layout.fillWidth: true
      visible: balance !== null
      spacing: Commons.Style.spacing.xs

      Label {
        Layout.fillWidth: true
        text: parent.balance ? strings.t("remaining").arg(root.money(parent.balance.remaining, parent.balance.currency)) : ""
        font.bold: true
      }
      Meter {
        Layout.fillWidth: true
        visible: parent.balance?.funded > 0
        fraction: visible ? parent.balance.remaining / parent.balance.funded : 0
        alarming: visible && fraction <= 0.1
      }
      SupportingLabel {
        Layout.fillWidth: true
        visible: text !== ""
        text: {
          const balance = parent.balance;
          return balance && balance.spent !== null && balance.funded !== null ? strings.t(balance.estimated ? "estimatedSpent" : "spent").arg(root.money(balance.spent, balance.currency)).arg(root.money(balance.funded, balance.currency)) : "";
        }
      }
    }

    Label {
      Layout.fillWidth: true
      visible: details.dataItem.resetCredits > 0
      text: visible ? root.counted("resetCredits", details.dataItem.resetCredits) : ""
    }

    SupportingLabel {
      Layout.fillWidth: true
      visible: details.sourceFailed || details.dataItem.stale
      text: visible ? root.staleText(details.dataItem.fetched ?? details.fallbackTime) : ""
    }

    Label {
      Layout.fillWidth: true
      visible: text !== ""
      color: root.urgent
      text: details.dataItem.status
    }

    SupportingLabel {
      Layout.fillWidth: true
      visible: text !== ""
      text: details.dataItem.help
    }
  }
}
