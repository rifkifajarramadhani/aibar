pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as Controls
import QtQuick.Layouts
import qs.Commons as Commons
import qs.Ui as Ui
import "Records.js" as Records

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
  property double now: Date.now()
  readonly property double today: {
    const date = new Date(now);
    return new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
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
      implicitHeight: header.implicitHeight + Commons.Style.spacing.md + sections.implicitHeight

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
        anchors.fill: parent
        spacing: Commons.Style.spacing.md

        ColumnLayout {
          id: header
          Layout.fillWidth: true
          spacing: Commons.Style.spacing.sm

          RowLayout {
            Layout.fillWidth: true
            Label {
              Layout.fillWidth: true
              text: strings.t("title")
              font.pixelSize: Commons.Style.font.body
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

          Label {
            Layout.fillWidth: true
            visible: root.usage?.refreshing ?? false
            text: strings.t("refreshing")
          }

          Label {
            Layout.fillWidth: true
            visible: root.usage?.refreshFailed ?? false
            color: root.urgent
            text: strings.t("refreshFailed")
          }

          Label {
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
            spacing: Commons.Style.spacing.md

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
                required property var modelData
                readonly property var provider: modelData.record
                readonly property bool failed: root.usage?.failedProviders[provider.id] === true
                readonly property bool updatedToday: provider.updated !== null && provider.updated >= root.today
                readonly property var days: Records.history(provider, root.today)
                readonly property double peak: Math.max(1, ...days.map(day => day.tokens ?? 0))

                Layout.fillWidth: true
                spacing: Commons.Style.spacing.sm

                Ui.PanelSeparator {
                  Layout.fillWidth: true
                  foreground: root.foreground
                }

                Label {
                  Layout.fillWidth: true
                  text: section.provider.name
                  font.pixelSize: Commons.Style.font.body
                  font.bold: true
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
                    spacing: Commons.Style.spacing.sm

                    Label {
                      Layout.fillWidth: true
                      text: account.modelData.name
                      font.bold: true
                    }
                    Label {
                      Layout.fillWidth: true
                      visible: account.modelData.active
                      text: strings.t("active")
                    }
                    Label {
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

                Label {
                  Layout.fillWidth: true
                  visible: !section.provider.hasActivity && !section.provider.limits.length && !section.provider.balance && !section.provider.accounts.length && !section.provider.status && !section.provider.help
                  text: strings.t("noDetails")
                }

                Ui.PanelSectionHeader {
                  Layout.fillWidth: true
                  visible: section.provider.hasActivity
                  foreground: root.foreground
                  fontFamily: root.fontFamily
                  text: strings.t("today")
                }

                Label {
                  Layout.fillWidth: true
                  visible: section.provider.hasActivity
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

                Ui.PanelSectionHeader {
                  Layout.fillWidth: true
                  visible: section.provider.hasActivity
                  foreground: root.foreground
                  fontFamily: root.fontFamily
                  text: strings.t("history")
                }

                ColumnLayout {
                  Layout.fillWidth: true
                  visible: section.provider.hasActivity
                  spacing: Commons.Style.spacing.xs

                  Repeater {
                    model: section.provider.hasActivity ? section.days : []
                    delegate: RowLayout {
                      required property var modelData
                      Layout.fillWidth: true
                      spacing: Commons.Style.spacing.md
                      readonly property string dateLabel: modelData.date.toLocaleDateString(root.locale, "ddd")
                      readonly property string fullDate: modelData.date.toLocaleDateString(root.locale, Locale.LongFormat)
                      Accessible.role: Accessible.StaticText
                      Accessible.name: modelData.tokens === null ? strings.t("noHistoryForDate").arg(fullDate) : strings.t("historyForDate").arg(fullDate).arg(root.counted("tokens", modelData.tokens))

                      Label {
                        Layout.preferredWidth: scroll.width * 0.15
                        text: parent.dateLabel
                      }
                      Meter {
                        Layout.fillWidth: true
                        fraction: (parent.modelData.tokens ?? 0) / section.peak
                      }
                      Label {
                        Layout.preferredWidth: scroll.width * 0.35
                        text: parent.modelData.tokens === null ? strings.t("noData") : root.formatCount(parent.modelData.tokens)
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
    font.pixelSize: Commons.Style.font.bodySmall
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
    spacing: Commons.Style.spacing.sm

    Label {
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
          Label {
            Layout.fillWidth: true
            text: limit.modelData.label || strings.t("limit")
          }
          Label {
            text: strings.t("percentUsed").arg(root.formatCount(Math.round(limit.modelData.percent * 100)))
          }
        }
        Meter {
          Layout.fillWidth: true
          fraction: limit.modelData.percent
          alarming: fraction >= 0.9
        }
        Label {
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
      }
      Meter {
        Layout.fillWidth: true
        visible: parent.balance?.funded > 0
        fraction: visible ? parent.balance.remaining / parent.balance.funded : 0
        alarming: visible && fraction <= 0.1
      }
      Label {
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

    Label {
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

    Label {
      Layout.fillWidth: true
      visible: text !== ""
      text: details.dataItem.help
    }
  }
}
