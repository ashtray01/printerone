# Security

## Trust model

PrinterOne implements the common raw TCP printing protocol. It intentionally
does not add proprietary authentication bytes because ordinary print clients,
label software and embedded devices would not understand them.

Treat the configured port as a **trusted private-LAN service**:

- never expose it directly to the public internet;
- use the built-in Windows Firewall action, which creates a rule for the
  private network profile only;
- restrict access further with VLAN or firewall policy when the LAN contains
  untrusted devices;
- remember that raw printer languages can contain device-specific commands.

## Defensive controls

- configurable maximum job size and read timeout;
- bounded concurrent connections and outstanding print jobs;
- no automatic termination of another process when a port is occupied;
- per-user atomic configuration storage;
- bounded in-memory logs that do not record job contents;
- single-instance process guard.

## Reporting a vulnerability

Please report security issues privately to `ashtray@me.com`. Include the
affected version, reproduction steps and expected impact. Do not include
printer data or credentials from a production environment.
