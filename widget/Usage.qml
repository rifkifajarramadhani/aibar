pragma ComponentBehavior: Bound

import QtQml.Models
import QtQuick
import Quickshell
import Quickshell.Io
import "OmarchyFiles.mjs" as OmarchyFiles
import "Records.mjs" as Records

Item {
  id: root

  readonly property string directory: (Quickshell.env("XDG_STATE_HOME") || Quickshell.env("HOME") + "/.local/state") + "/omarchy/agents/usage"

  property var providers: []
  property bool invalidRecords: false
  property bool refreshFailed: false
  property var failedProviders: ({})

  readonly property bool refreshing: updateKind !== 0
  readonly property int limitsOnly: 1
  readonly property int regular: 2
  readonly property int manual: 3
  property int updateKind: 0
  property int pendingKind: 0
  property bool scanPending: false
  property var failures: ({})

  function scan() {
    if (discovery.running)
      scanPending = true;
    else
      discovery.running = true;
  }

  function setFiles(files) {
    for (let i = records.count - 1; i >= 0; i--) {
      const id = records.get(i).recordId;
      if (!(id in files))
        records.remove(i);
      else
        records.setProperty(i, "readableSize", files[id]);
    }

    const known = new Set();
    for (let i = 0; i < records.count; i++)
      known.add(records.get(i).recordId);
    for (const id of Object.keys(files).sort()) {
      if (!known.has(id))
        records.append({
          recordId: id,
          readableSize: files[id]
        });
    }

    // The updater renames into place. Reload even if inotify missed the rename.
    for (let i = 0; i < readers.count; i++)
      (readers.objectAt(i) as RecordReader)?.reload();
    Qt.callLater(root.publish);
  }

  function publish() {
    const providers = [];
    let invalid = false;
    for (let i = 0; i < readers.count; i++) {
      const reader = readers.objectAt(i) as RecordReader;
      if (!reader)
        continue;
      if (reader.record)
        providers.push({
          record: reader.record,
          unreadable: reader.invalid
        });
      invalid = invalid || reader.invalid;
    }
    providers.sort((a, b) => a.record.id.localeCompare(b.record.id));
    root.providers = providers;
    root.invalidRecords = invalid;
  }

  function request(kind) {
    if (refreshing) {
      // A wider refresh covers a cheaper request, even from another monitor.
      if (kind > updateKind)
        pendingKind = Math.max(pendingKind, kind);
      return;
    }
    scan();
    updateKind = kind;
    failures = Object.create(null);
    const command = ["timeout", "--kill-after=5s", "5m", "omarchy-agent-usage-update"];
    if (kind === manual)
      command.push("--force");
    if (kind === limitsOnly)
      command.push("--limits-only");
    updater.exec(command);
  }

  function finishRefresh(code, status) {
    refreshFailed = code !== 0 || status !== 0;
    failedProviders = failures;
    scan();
    const next = pendingKind;
    pendingKind = 0;
    updateKind = 0;
    if (next !== 0)
      request(next);
  }

  Timer {
    interval: 15 * 60 * 1000
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: root.request(root.regular)
  }

  Process {
    id: updater
    onExited: (code, status) => Qt.callLater(root.finishRefresh, code, status)
    stderr: SplitParser {
      onRead: line => {
        const match = /^omarchy-agent-usage-update: ([a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}) collector failed$/.exec(line);
        if (match)
          root.failures[match[1]] = true;
      }
    }
  }

  Process {
    id: discovery
    command: ["find", root.directory, "-maxdepth", "1", "-type", "f", "-name", "*.json", "-printf", "%f\\0%s\\0"]
    stdout: StdioCollector {
      id: listing
    }
    onExited: (code, status) => {
      if (code === 0 && status === 0)
        root.setFiles(OmarchyFiles.listing(listing.text));
      if (root.scanPending) {
        root.scanPending = false;
        Qt.callLater(root.scan);
      }
    }
  }

  ListModel {
    id: records
  }

  Instantiator {
    id: readers
    model: records
    onObjectAdded: Qt.callLater(root.publish)
    onObjectRemoved: Qt.callLater(root.publish)

    delegate: RecordReader {}
  }

  component RecordReader: QtObject {
    id: reader
    required property string recordId
    required property bool readableSize
    property var record: null
    property bool invalid: !readableSize

    onReadableSizeChanged: {
      if (!readableSize)
        invalid = true;
      Qt.callLater(root.publish);
    }
    onRecordChanged: Qt.callLater(root.publish)
    onInvalidChanged: Qt.callLater(root.publish)

    function reload() {
      if (readableSize)
        file.reload();
    }

    property FileView file: FileView {
      path: reader.readableSize ? root.directory + "/" + reader.recordId + ".json" : ""
      printErrors: false
      watchChanges: true
      onFileChanged: root.scan()
      onLoadFailed: if (reader.readableSize)
        reader.invalid = true
      onLoaded: {
        if (!reader.readableSize)
          return;
        const parsed = Records.parse(text(), reader.recordId);
        reader.invalid = parsed === null;
        if (parsed)
          reader.record = parsed.record;
      }
    }
  }
}
