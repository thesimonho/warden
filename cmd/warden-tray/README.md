# warden-tray

System tray companion for `warden-desktop`. Shows a persistent icon so users
know Warden is running after closing the browser window.

## Building

This binary requires CGo (for native tray APIs):

```bash
cd cmd/warden-tray
CGO_ENABLED=1 go build -o ../../bin/warden-tray .
```

The three core Warden binaries (`warden`, `warden-desktop`, `warden-tui`)
remain CGo-free. Only this tray binary requires a C compiler.

## Architecture

The tray is a standalone process that talks to `warden-desktop` over HTTP.
It does not import any packages from the main Warden module — this keeps
the CGo dependency fully isolated.

Environment variables:

| Variable     | Default                  | Description         |
| ------------ | ------------------------ | ------------------- |
| `WARDEN_URL` | `http://127.0.0.1:8090` | Server base URL     |

## Dependencies

- [fyne-io/systray](https://github.com/fyne-io/systray) — cross-platform system tray (Linux DBus, macOS Cocoa, Windows Win32)
- [ncruces/zenity](https://github.com/ncruces/zenity) — cross-platform native dialogs (no CGo needed for dialogs)
