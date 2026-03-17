# Share App

Windows-host + mobile web client for remote control of a selected application window over Tailscale.

## Components

- `host/`: Go service that serves the client, handles auth, signaling, WebRTC, and input injection.
- `client/`: Vite/Vanilla JS mobile web client.
- `native-capture/`: .NET capture layer and `CaptureProbe` helper built on `Windows.Graphics.Capture`.

## Current Architecture

Video path:

1. `host` starts a WebRTC peer.
2. `host` opens a long-lived `CaptureProbe` process.
3. `CaptureProbe` keeps a long-lived `WgcCaptureService` session for the selected `HWND`.
4. Raw `BGRA` frames stream to `host`.
5. `host` feeds frames into one long-lived `ffmpeg` process.
6. `ffmpeg` encodes `VP8/IVF`.
7. `host` writes encoded samples into the Pion WebRTC track.

Input path:

1. Mobile client sends touch/keyboard commands over WebRTC data channel or WebSocket fallback.
2. `host/internal/input` converts normalized coordinates into the selected window client area.
3. Win32 input is injected with `SendInput` and a few direct window messages.

## Prerequisites

- Windows 10/11
- Node.js for `client/`
- .NET SDK that can build `net6.0-windows10.0.19041.0`
- `ffmpeg` available in `PATH`
- Tailscale installed and logged in

## Build

### 1. Build client

```bash
cd client
npm install
npm run build
```

### 2. Build native capture helper

```bash
dotnet build "native-capture/tests/CaptureProbe/CaptureProbe.csproj"
```

### 3. Build Go host

```bash
go build ./...
```

Run this inside `host/`.

## Local HTTP Run

Use for quick desktop testing:

```bash
cd host
SHARE_APP_ADDR=:8095 SHARE_APP_SECRET=test-secret go run ./cmd/share-host
```

Open:

- `http://127.0.0.1:8095/?secret=test-secret`

## Tailscale HTTPS Run

Generate certificates first:

```bash
tailscale cert bigbro.tail38c17.ts.net
```

This writes:

- `bigbro.tail38c17.ts.net.crt`
- `bigbro.tail38c17.ts.net.key`

Run host from `host/`:

```bash
SHARE_APP_ADDR=:8443 SHARE_APP_CERT_DIR="d:\Dev\share-app" SHARE_APP_TAILSCALE_DOMAIN=bigbro.tail38c17.ts.net SHARE_APP_SECRET=test-secret go run ./cmd/share-host
```

Open from phone:

- `https://bigbro.tail38c17.ts.net:8443/?secret=test-secret`

## Selecting The Target Window

You must select a target window before control/video works.

Options:

- local host UI: `http://127.0.0.1:8095/host-ui`
- API:

```bash
curl -X POST -H "Content-Type: application/json" -d "{\"handle\":657830}" "http://127.0.0.1:8095/api/target-window"
```

List windows:

```bash
curl "http://127.0.0.1:8095/api/windows"
```

## Useful Debug Commands

Verify native capture:

```bash
dotnet run --project "native-capture/tests/CaptureProbe/CaptureProbe.csproj" -- --hwnd 657830 --out "d:\Dev\share-app\test.png"
```

Verify host snapshot:

```bash
curl -H "Authorization: Bearer <token>" "http://127.0.0.1:8095/api/snapshot?out=d:/Dev/share-app/host-check.png"
```

## Important Notes

- Root `.gitignore` intentionally ignores:
  - `*.crt`
  - `*.key`
  - `bin/`, `obj/`
  - `client/node_modules/`
  - `client/dist/`
- `CaptureProbe` is a required runtime dependency for streaming.
- `host/internal/nativecapture/bridge.go` resolves `CaptureProbe.exe` from the repo build output.
- The selected target window is currently in-memory only and is lost when the host restarts.
