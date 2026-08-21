# Security audit — 2026-08-21

## Scope

Static review of `server.py`, `build.py`, configuration and dependencies. No
network or printer was exercised during this review.

## Findings

| Priority | Finding | Evidence | Required remediation |
|---|---|---|---|
| Critical | The raw-print service accepts unauthenticated traffic on every network interface. Any reachable host can submit a print job and arbitrary printer-language commands. | `start_server()` binds `0.0.0.0`; `handle_client()` forwards received bytes directly to `WritePrinter`. | Default to loopback; require an explicit allow-list of CIDRs for LAN use. Add per-client authentication (shared token or mTLS) before accepting a job. Limit the firewall rule to the selected profile/private network. |
| High | Denial of service: connections, goroutines/threads and received data are unbounded. A client may hold a connection open indefinitely or exhaust memory by streaming bytes. | One thread per `accept`; `recv()` has no socket timeout; data is repeatedly appended to an in-memory `bytes` value. | Set read/write deadlines, a maximum job size, connection and worker limits, plus backpressure. Stream bounded data to a temporary spool instead of accumulating it in memory. |
| High | Starting the server terminates any process already listening on the configured port. This can kill an unrelated or privileged application. | `start_server()` calls `kill_process_on_port()`; it calls `terminate()` and then `kill()`. | Remove automatic termination. Report the occupied port, process ID/name where permitted, and let the user select another port or stop only this application. |
| Medium | Configuration is read from the current directory and `%TEMP%`; an attacker who controls either location can alter target printer, port and autostart behavior. | `load_config()` and `save_config()` enumerate `config.json` and temp locations. | Use one per-user data directory (`%LOCALAPPDATA%\\PrinterOne`), create it with restricted ACLs, validate all fields, and write atomically with `0600`-equivalent Windows ACLs. |
| Medium | The GUI is updated directly by server/test worker threads, which Tkinter does not support. This may cause crashes or corrupted UI state under load. | `PrinterOneServer.log()` invokes GUI callback; `handle_client()` runs in a worker thread. | Send log/status events through a thread-safe queue; update UI only from the UI event loop. |
| Medium | Print handles and document/page lifecycle are not closed in `finally` blocks. An exception can leak a printer handle or leave a job incomplete. | `print_raw()` closes `hPrinter` only on success. | Defer/`finally` cleanup and track job status. |
| Medium | Logs include command-line arguments, `USERPROFILE`, the full environment, absolute paths and configuration. | startup logging at module import and `__main__`. | Use structured logs, redact secrets/identifiers, avoid logging the full environment, and implement bounded rotation. |
| Low | Port and configuration values are not validated at the domain boundary. | GUI uses `IntVar`; server trusts loaded JSON. | Enforce port range 1–65535, printer-name length/known printer validation, and strict JSON schema. |
| Low | Dependencies use lower bounds only and have no locked, repeatable build. | `requirements.txt`. | Pin/lock dependencies until the Go migration; enable Dependabot/Renovate and add vulnerability scanning. |

## Security baseline for the next release

1. Remove automatic process termination.
2. Bind to `127.0.0.1` by default; LAN exposure must be an opt-in setting.
3. Add allowed-client CIDRs, a job-size limit, read timeout, concurrent-client limit and a bounded queue.
4. Keep authentication disabled only for an explicitly chosen trusted private LAN, with a prominent warning; make a pre-shared token the normal LAN mode.
5. Move configuration and logs to `%LOCALAPPDATA%\\PrinterOne` and redact sensitive diagnostics.
6. Add integration tests with a fake printer/spooler before behavior changes.

## Review limitations

The project has no automated test suite and the local Python interpreter could
not be started in this environment, so runtime behavior, dependency CVEs and
Windows spooler interactions still require verification on a Windows test host.
