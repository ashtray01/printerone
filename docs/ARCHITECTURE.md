# Architecture

PrinterOne is a Windows desktop application built as two Go modules:

- the root module contains configuration, the bounded TCP receiver and the
  Windows raw-printer adapter;
- `desktop/` contains the Wails shell, system tray, Windows Firewall and
  startup-registry integration, plus the bundled web UI.

## Data flow

```text
LAN client
    │ raw TCP job, connection close terminates the job
    ▼
TCP receiver ── validates size/deadline/concurrency limits
    │
    ▼
Windows spooler adapter ── RAW document ──► selected local printer
```

The frontend never opens sockets or accesses the printer directly. It calls a
small set of Wails-bound Go methods and renders the returned state. Network
events are recorded in a thread-safe, bounded in-memory log.

## Runtime integrations

- **Printers:** Windows spooler through `github.com/alexbrainman/printer`.
- **Tray:** `fyne.io/systray`; closing the window may hide it without stopping
  the receiver.
- **Firewall:** a private-profile inbound TCP rule created through an explicit
  UAC-confirmed action.
- **Startup:** per-user `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`.
- **Configuration:** `%APPDATA%\PrinterOne\config.json`, written atomically.
- **Diagnostics:** per-session metadata logs under
  `%APPDATA%\PrinterOne\logs`; raw job contents are never persisted.

## Limits

The receiver bounds each job, applies a resettable idle read deadline, limits concurrent
connections and rejects work above the configured outstanding-job limit. The
GUI retains at most 500 server log lines, 100 UI messages and 200 test-client
messages.
