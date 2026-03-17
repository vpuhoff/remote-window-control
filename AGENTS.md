# AGENTS

## Project Overview

Windows-only remote window streaming: host + mobile PWA client over Tailscale.

- `host/`: Go server, auth, signaling, WebRTC, input injection
- `client/`: Vite mobile PWA client (window picker, gestures, keyboard)
- `native-capture/`: .NET `Windows.Graphics.Capture` + `CaptureProbe`

## Required Build Order

When changing runtime behavior, build in this order:

1. `client`
2. `native-capture`
3. `host`

### Client

Run from `client/`:

```bash
npm install
npm run build
```

### Native Capture

Build from repo root:

```bash
dotnet build "native-capture/tests/CaptureProbe/CaptureProbe.csproj"
```

This produces the `CaptureProbe` executable used by the Go host.

### Host

Run from `host/`:

```bash
go build ./...
```

Do not rely on plain `go` from `PATH` unless you have verified it resolves the required toolchain correctly.

## Run Commands

### Local HTTP

```bash
cd host
SHARE_APP_ADDR=:8095 SHARE_APP_SECRET=test-secret go run ./cmd/share-host
```

### Tailscale HTTPS

Certificates are expected in repo root when using the current workflow.

```bash
cd host
SHARE_APP_ADDR=:8443 SHARE_APP_CERT_DIR="d:\Dev\share-app" SHARE_APP_TAILSCALE_DOMAIN=bigbro.tail38c17.ts.net SHARE_APP_SECRET=test-secret go run ./cmd/share-host
```

## Runtime Assumptions

- `ffmpeg` must be available in `PATH`
- Tailscale certificates are generated with:

```bash
tailscale cert bigbro.tail38c17.ts.net
```

- `CaptureProbe` is resolved from:
  - `native-capture/tests/CaptureProbe/bin/Debug/net6.0-windows10.0.19041.0/`
  - or the `Release` equivalent

## Working Rules For Agents

- Do not commit generated files from `client/node_modules/`, `client/dist/`, `bin/`, or `obj/`.
- Do not commit `*.crt` or `*.key`.
- Prefer editing source files only; rebuild artifacts locally when needed.
- After changing:
  - `client/src/**`: rebuild client
  - `native-capture/**`: rebuild `CaptureProbe`
  - `host/**`: rebuild host
- Key files when debugging issues:
  - `host/internal/webrtc/windowstream.go`
  - `host/internal/nativecapture/bridge.go`
  - `native-capture/tests/CaptureProbe/Program.cs`
  - `native-capture/src/WindowCapture.Native/WgcCaptureService.cs`

## Current Streaming Model

The current implementation uses:

1. one long-lived `CaptureProbe` process per selected target
2. one long-lived `ffmpeg` encoder process per peer
3. raw `BGRA` frames from `CaptureProbe`
4. `VP8/IVF` samples into Pion

Do not revert this back to per-frame `CaptureProbe` or per-frame `ffmpeg`.

## Input Notes

- Gestures: `client/src/gestures.js` — tap, long press (right click), single-finger scroll, two-finger scroll
- Scroll is sent to coordinates of last tap (`x`, `y` in `input.scroll`)
- Host: `sendinput.go`, `target.go` — coordinate normalization, `postMouseWheel` with focus and SendMessage
- Auth: `client/src/auth.js` — secret in localStorage, `refreshToken` on 401; `api.js` — retry on 401

## Client Flow

1. `bootstrapAuth` — exchange secret for token, store in localStorage
2. `showWindowSelect` — `fetchWindows`, window cards
3. Click on window → `setTargetWindow` → `startRemoteControl` (WebRTC)
4. On 401 API retry with `refreshToken`

## Useful Smoke Tests

Check native capture:

```bash
dotnet run --project "native-capture/tests/CaptureProbe/CaptureProbe.csproj" -- --hwnd 657830 --out "d:\Dev\share-app\probe-test.png"
```

Check target selection:

```bash
curl "http://127.0.0.1:8095/api/windows"
curl -X POST -H "Content-Type: application/json" -d "{\"handle\":657830}" "http://127.0.0.1:8095/api/target-window"
```

Check snapshot:

```bash
curl -H "Authorization: Bearer <token>" "http://127.0.0.1:8095/api/snapshot?out=d:/Dev/share-app/check.png"
```
