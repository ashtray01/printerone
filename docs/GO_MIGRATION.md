# Go migration plan

## Target

Retain Windows local-printer support and a desktop UI while replacing the
Python/Tkinter monolith with a testable Go application. The migration must not
change the wire protocol accidentally: current clients connect by TCP and end a
job by closing the connection.

## Proposed layout

```text
cmd/printerone/          application entry point
internal/config/         schema, validation, atomic persistence
internal/receiver/       TCP listener, limits, access policy, authentication
internal/jobs/           bounded queue, spool files, status and retention
internal/printer/        interface + Windows spooler implementation
internal/service/        orchestration, lifecycle and events
internal/logging/        structured/redacted logging
internal/ui/             desktop adapter; consumes service events only
```

The central interfaces should be deliberately small:

```go
type Printer interface {
    List(ctx context.Context) ([]string, error)
    Print(ctx context.Context, name string, r io.Reader, meta JobMeta) (JobID, error)
}

type JobStore interface {
    Enqueue(ctx context.Context, r io.Reader, meta JobMeta) (JobID, error)
    Get(ctx context.Context, id JobID) (Job, error)
}
```

Implement Windows printing behind `internal/printer/windows`; do not expose
Win32 types to the receiver, UI or configuration packages. Use `context` for
shutdown/deadlines, a bounded worker pool for client connections and a bounded
job queue for the spooler.

## UI direction

Use the supplied dark, compact desktop reference as the UX target: a narrow
left navigation rail, one task-focused view at a time, dark cards, prominent
start/stop actions and log/search controls. The current Python UI implements a
compact functional approximation only; pixel-level parity and richer controls
belong in the Go rewrite.

The three screens are **Dashboard** (running state, endpoint, queue and recent
jobs), **Test client** (connection and a test-job action), and **Settings**
(printer, listener/access policy, autostart, theme and language). The UI must
support Russian (default), English, German, Simplified Chinese and Spanish.
All user-visible text belongs in locale resource files; no strings are embedded
in view code. The UI never invokes printer or socket APIs directly: it sends
commands to the service and renders emitted state/events.

Choose Wails for the Go UI: it permits a modern HTML/CSS implementation close
to the reference while the print server remains pure Go. The desktop shell
should expose only a typed, local IPC surface to the frontend; it must not
create a local unauthenticated HTTP control API.

## Incremental delivery

1. Freeze current behavior with a protocol contract and fake-printer tests.
2. Extract and harden the Python configuration and TCP limits as a short-term
   release, without adding more features to the monolith.
3. Implement the Go domain packages and fake printer; verify packet framing,
   limits, cancellation and queue behavior in CI.
4. Add the Windows spooler adapter and run parallel test prints against a
   non-production printer.
5. Build the new UI against the Go service, add import of the existing JSON
   settings, then release it as an opt-in preview.
6. Migrate users only after compatibility, security and rollback criteria pass.

## Compatibility decisions to make before implementation

- Whether a TCP close remains the only end-of-job delimiter, or a versioned
  framed protocol is introduced for new clients.
- Which LAN authentication model is acceptable: shared token, mTLS, or both.
- Maximum permitted job size, queue length, retention period and retry policy.
- Chosen UI toolkit and the Windows delivery model (desktop executable only,
  Windows Service + desktop controller, or both).

## First acceptance criteria

- Default install is unreachable from the LAN.
- A configured, authorized client can submit a job; an unauthorized client
  cannot.
- A slow client, oversize job and full queue do not exhaust memory or prevent
  shutdown.
- Every received job has an auditable status: accepted, rejected, printing,
  printed or failed.
- Existing `config.json` can be imported once, then configuration is stored in
  the per-user application-data directory.
