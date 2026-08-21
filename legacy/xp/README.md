# PrinterOne for Windows XP and Windows 7

This directory contains the isolated native edition for Windows XP SP3 (x86)
and Windows 7. It does not import Wails or the modern desktop module and has no
third-party dependencies.

The executable contains its application icon as a native 386 PE resource. The
same icon is used by Explorer, the main window and the notification area.

The compact native interface supports Русский, English, Deutsch and Español.
Select a language next to the port field and click the adjacent `OK` button.
PrinterOne XP saves the language and immediately restarts to apply it.

## Screenshots

### Compact interface

![PrinterOne XP compact interface in Russian](../../assets/screenshots/printerone-xp-interface.png)

### Running on Windows XP SP3

![PrinterOne XP running on a Windows XP SP3 desktop](../../assets/screenshots/printerone-xp-desktop.png)

### Verified RAW printing on Windows XP SP3

![PrinterOne XP receiving and printing a RAW job over the network](../../assets/screenshots/windows-xp-raw-printing.png)

## Running the release locally

Download `PrinterOne-XP-SP3-x86.exe` from the GitHub Release and run it directly.
The legacy executable is portable and does not require WebView2, .NET, Node.js or
additional DLLs. Use this release build for Windows XP and Windows 7 testing.

Configuration and logs are created under `%APPDATA%\PrinterOne-XP`; they are not
written next to the executable. A binary produced by `build-local.ps1` with a
newer Go compiler is only intended for a quick smoke test on the development PC
and must not be treated as XP-compatible.

## Local development build

Run `./build-local.ps1`. This validates the source and produces a 32-bit Windows
executable with the locally installed Go compiler. Such a binary is only a smoke
artifact when the compiler is newer than Go 1.10.8; it is not XP-compatible.

## XP-compatible build

Extract the official Go 1.10.8 Windows archive into
`.toolchains\go1.10.8\go`, then run `./build-xp.ps1`. A different toolchain path
can be passed with `-GoRoot`. The script stages the source in a temporary GOPATH because Go
1.10 predates module support and writes `build/PrinterOne-XP-SP3-x86.exe`.

## Runtime data

- Configuration: `%APPDATA%\PrinterOne-XP\config.json`
- Logs: `%APPDATA%\PrinterOne-XP\logs\printerone-xp-*.log`
- Default endpoint: `0.0.0.0:9100`

`0.0.0.0` is the bind address, not a client destination. While the server is
running, the status line and session log show the detected LAN IPv4 endpoint
that other computers should use.

The XP build intentionally does not change Windows Firewall settings. If the
firewall is enabled, allow inbound TCP traffic for the configured port manually.

## Releases

`.github/workflows/legacy-xp.yml` runs the Go 1.10.8 tests and build independently
from the Wails job. Every `v*` tag uploads `PrinterOne-XP-SP3-x86.exe` and its
SHA-256 file to the same GitHub Release as the modern Windows build.
