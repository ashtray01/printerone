# Windows XP SP3 checkpoint 1

Use `build/PrinterOne-XP-SP3-x86.exe`, built by `build-xp.ps1` with Go 1.10.8.

1. Start the EXE and confirm that the main window opens without installing a runtime.
2. Start it a second time and confirm that the second instance reports an error.
3. Confirm that the printer list contains the XP printers, including `Generic / Text Only`.
4. Select `Generic / Text Only`, keep `0.0.0.0:9100`, and click `Сохранить`.
5. Click `Запустить`; the status must change to `Сервер работает`.
6. From the host, connect to the VM port 9100 and close without data. The XP log must show a connection test.
7. Send a small RAW/text file from the host. The XP log must show CONNECT, RECEIVE, FORMAT, SPOOL and PRINT entries.
8. Confirm that the test printer receives the exact data.
9. Click `Тестовая печать` and confirm that the local loopback job reaches the printer.
10. With `В трей` enabled, close to tray, restore by clicking the tray icon, then exit with `Выход`.
11. Disable `В трей` without saving and close the window. Confirm that the process and tray icon disappear.
12. Change the language and click the small adjacent `OK` button. Confirm that the app restarts and the whole UI is translated.
13. Enable `Запускать с Windows`, sign out and back in, and confirm one automatic launch.
14. Leave a `PrinterOne RAW job` in the Windows queue, restart the app, and confirm that the stale job is removed rather than printed again.

Runtime files are isolated under `%APPDATA%\PrinterOne-XP`. If a step fails,
copy the newest file from `%APPDATA%\PrinterOne-XP\logs` together with the
visible error text.

Firewall configuration is intentionally manual in this checkpoint. If host-to-VM
connections fail but the local test works, temporarily disable Windows Firewall
or add TCP port 9100 manually before reporting a server defect.
