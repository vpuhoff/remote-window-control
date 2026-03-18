# Share App

Windows host + mobile web client for remote control of a selected application window via Tailscale.

## Purpose

Share App lets you control a **single window** on your Windows PC from your phone over Tailscale. Use cases:

- Control one app (browser, IDE, game) from your phone without exposing the full desktop
- Quick access to a window on your home PC from anywhere (within your Tailscale network)
- Share or present a specific window without giving access to the rest of the desktop

**What makes it different:**
- **Window-level** — streams one window, not the full screen
- **PWA** — install from browser, no app store
- **Tailscale-native** — no port forwarding, private network only

## Alternatives

| Solution | Scope | Difference from Share App |
|----------|-------|---------------------------|
| RDP / Microsoft Remote Desktop | Full desktop | Entire screen; requires RDP client |
| Chrome Remote Desktop | Full desktop | Entire screen; Google dependency |
| TeamViewer / AnyDesk | Full desktop | Entire screen; paid tiers; public relay |
| Parsec | Full desktop (gaming) | Optimized for games; full screen |
| RustDesk | Full desktop | Open-source RDP/VNC; full screen |
| Tailscale + RDP | Full desktop | Full screen; requires RDP client |
| VNC (TightVNC, etc.) | Full desktop | Full screen; no built-in mobile PWA |
| Apache Guacamole | Full desktop | RDP/VNC in browser; heavier setup |

Share App fits when you need **one window** from your phone over Tailscale, without installing a dedicated client and without exposing the full desktop.

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
tailscale cert your-machine.tail12345.ts.net
```

This writes:

- `your-machine.tail12345.ts.net.crt`
- `your-machine.tail12345.ts.net.key`

Run host from `host/`:

```bash
SHARE_APP_ADDR=:8443 SHARE_APP_CERT_DIR="." SHARE_APP_TAILSCALE_DOMAIN=your-machine.tail12345.ts.net SHARE_APP_SECRET=test-secret go run ./cmd/share-host
```

Open from phone:

- `https://your-machine.tail12345.ts.net:8443/?secret=test-secret`

## Window Selection

When opening a link with `?secret=...` the client shows a "Select application" screen with a list of windows. After selection, streaming and control begin.

**Alternatives (for debugging):**
- Host UI: `http://127.0.0.1:8095/host-ui`
- API: `POST /api/target-window` with `{"handle": N}`, list — `GET /api/windows`

**Auth:** secret is stored in localStorage; on host restart the token is refreshed automatically (retry on 401).

## Release / Distro

Pre-built Windows distros are in [Releases](https://github.com/vpuhoff/remote-window-control/releases). Structure:

```
share-app-vX.Y.Z/
  share-host.exe
  CaptureProbe/
    CaptureProbe.exe, *.dll
  web/
    index.html, assets/, manifest, sw.js
```

Run from the extracted folder: `.\share-host.exe` (or double-click). Set env vars or use `.env` for `SHARE_APP_SECRET`, `SHARE_APP_TAILSCALE_DOMAIN`, etc.

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
