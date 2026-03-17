# Share App

Windows host + mobile web client for remote control of a selected application window via Tailscale.

## Components

- `host/` — Go service: client serving, auth, signaling, WebRTC, input injection
- `client/` — Vite/Vanilla JS PWA client for mobile
- `native-capture/` — .NET capture layer on `Windows.Graphics.Capture`

## Architecture

**Video:**
1. Host creates WebRTC peer
2. Starts long-lived `CaptureProbe` process
3. `CaptureProbe` holds `WgcCaptureService` session for selected `HWND`
4. Raw BGRA frames → host → ffmpeg (VP8/IVF) → Pion WebRTC track

**Input:**
1. Client sends touch/keyboard via WebRTC data channel or WebSocket
2. `host/internal/input` maps normalized coordinates to window area
3. Win32: `SendInput`, `PostMessage` (WM_MOUSEWHEEL), `SetForegroundWindow`

**Gestures (client):**
- Tap — click
- Long press — right click + drag
- Swipe up — scroll down, swipe down — scroll up (single finger)
- Two fingers — scroll by movement
- Scroll is sent to coordinates of last tap

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

## Window Selection

When opening a link with `?secret=...` the client shows a "Select application" screen with a list of windows. After selection, streaming and control begin.

**Alternatives (for debugging):**
- Host UI: `http://127.0.0.1:8095/host-ui`
- API: `POST /api/target-window` with `{"handle": N}`, list — `GET /api/windows`

**Auth:** secret is stored in localStorage; on host restart the token is refreshed automatically (retry on 401).

## Useful Debug Commands

Verify native capture:

```bash
dotnet run --project "native-capture/tests/CaptureProbe/CaptureProbe.csproj" -- --hwnd 657830 --out "d:\Dev\share-app\test.png"
```

Verify host snapshot:

```bash
curl -H "Authorization: Bearer <token>" "http://127.0.0.1:8095/api/snapshot?out=d:/Dev/share-app/host-check.png"
```

## PWA

The client is a PWA: it can be installed on Android (Chrome: menu → "Install app"). Manifest and Service Worker are included.

## Important

- `.gitignore` excludes:
  - `*.crt`
  - `*.key`
  - `bin/`, `obj/`
  - `client/node_modules/`
  - `client/dist/`
- `CaptureProbe` is a required runtime dependency for streaming.
- `host/internal/nativecapture/bridge.go` resolves `CaptureProbe.exe` from the repo build output.
- Selected window is kept in host memory and reset on restart (client automatically gets a new token).
