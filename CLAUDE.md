# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

SimLink bridges USB sim gear (VIRPIL, MOZA, etc.) to RC transmitters. It reads Linux evdev input devices, maps axes/buttons to up to 32 RC channels, and outputs via CRSF/ELRS serial, SBUS serial, or PPM waveform over an audio jack (trainer port). A React web UI on port 8080 handles all configuration.

## Commands

### Backend

All backend commands run from `backend/`:

```bash
# Build
go build ./...

# Run (dev) — frontend must be built first, see below
DATA_DIR=../data go run ./main.go

# Lint
go vet ./...
```

No test suite yet. The binary requires `/dev/input/event*` access; run with sudo or add your user to the `input` group on Linux.

### Frontend

All frontend commands run from `frontend/`:

```bash
npm run build      # builds into backend/static/ (required before go build)
npm run dev        # dev server on :5173, proxies /api and /ws to localhost:8080
```

The Vite dev proxy (`vite.config.ts`) expects the Go backend running on `:8080`.

### Docker (full stack)

Run from the repo root:

```bash
docker compose build
docker compose up
# UI at http://<host>:8080
```

The multi-stage Dockerfile (`backend/Dockerfile`, context = repo root) builds the frontend first, copies `backend/static/` into the Go binary via `go:embed`, then produces a single Alpine image.

## Architecture

### Data flow

```
USB HID device
  └─ /dev/input/event*
       └─ input.Manager (goroutine per device, non-blocking evdev reads, 2s hotplug poll)
            └─ input.Event → mapping.Engine.Process()
                  └─ [32]float64 channel state (atomic CAS swap, normalized -1.0..1.0)
                        ├─ output.Manager → crsf/sbus/ppm goroutine (ticker-driven)
                        └─ api.BroadcastLoop → WebSocket clients (50 Hz)
```

### Key design decisions

**Channel state is a copy-on-write atomic pointer** (`mapping/mapping.go`). `Process()` does a CAS loop to update a single channel without locking the output goroutines. Readers (`Channels()`, output tickers, WS broadcast) just load the pointer — zero contention.

**Axis normalization requires axis range data.** `mapping.Engine` holds per-device per-axis min/max in `axisMin`/`axisMax` (keyed by `deviceID:code`). These are populated by `Engine.SetAxisRange()` which must be called when a device is opened (currently missing from the event loop in `main.go` — device axes are scanned in `input.buildInfo` but ranges are not forwarded to the engine yet; this is a known gap).

**Output protocols** are each a self-contained sub-package (`output/crsf`, `output/sbus`, `output/ppm`). Each implements a `Run(ctx, channels func() [32]float64)` pattern — it owns its goroutine and exits when ctx is cancelled. `output.Manager.SetConfig()` stops the previous output goroutine and starts a new one atomically.

**Failsafe is owned by mapping rules, not output config.** Each `mapping.Rule` has a `Failsafe float64` field (normalized -1.0..1.0) set per channel in the Mapping UI. `Engine.Failsafe()` builds the `[32]float64` array from current rules (channels with no rule → 0). `output.Manager.SetConfig()` calls `engine.Failsafe()` to derive the array at output start time. `output.Config` has no failsafe field.

**PPM uses `aplay` subprocess.** `output/ppm` generates S16_LE mono PCM at 48 kHz and pipes it to `aplay` stdin. `alsa-utils` must be present in the runtime image (it is, via the Dockerfile). The 20 ms frame contains 8 channels; channels 9–32 are not transmitted over PPM.

**SBUS requires hardware signal inversion.** The serial port outputs non-inverted 8E2 at 100000 baud; a transistor or 74HC04 inverter is needed between TX and the receiver.

**Frontend build is embedded.** `main.go` uses `//go:embed static` — the `backend/static/` directory must be non-empty at `go build` time. A placeholder `static/index.html` exists so `go build` works before running `npm run build`. Running `npm run build` overwrites it with the real app.

**Profiles store the full config blob as JSON.** `profile.Config` is a `json.RawMessage` column. When a profile is activated (`POST /api/profiles/:id/activate`), the API unmarshals it into `{ rules: []mapping.Rule, output: output.Config }` and applies both.

**Auth is optional.** If no password is set (`data/auth.json` has no `password_hash`), all routes are open. Session tokens are HMAC-SHA256 signed with a random secret generated on first run and persisted in `data/auth.json`.

### Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `DATA_DIR` | `/data` | SQLite files (`profiles.db`, `telemetry.db`) + `auth.json` |
| `LISTEN_ADDR` | `:8080` | HTTP listen address |
| `GIN_MODE` | (unset) | Set to `release` to suppress gin debug logs |

### Adding a new output protocol

1. Create `backend/internal/output/<name>/<name>.go` with a struct implementing `Run(ctx context.Context, channels func() [32]float64)`.
2. Add a `Protocol<Name>` constant in `output/config.go`.
3. Add a case in `output.Manager.SetConfig()`.
4. Add the protocol option to `frontend/src/pages/Output.tsx` selector.
