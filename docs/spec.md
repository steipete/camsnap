## camsnap – CLI for RTSP/ONVIF cameras

### Goals (MVP)
- Add/list cameras with stored per‑camera credentials (Tapo “Camera Account” or equivalent local user).
- Grab a still frame (`snap`) or short clip (`clip`) from an RTSP URL.
- Skeleton for motion watch daemon (`watch`) that will later trigger a command on motion.
- ONVIF WS-Discovery to find cameras and print ready-to-use `add` commands.

### Out of scope for MVP
- Tapo cloud login (no public API; only per‑camera accounts via RTSP/ONVIF).
- Battery/low‑power Tapo models that disable RTSP.
- Ubiquiti Protect API integration (planned next; today use RTSPS/RTSP URLs manually).

### Command surface
- `camsnap add --name cam1 --host 192.168.1.50 --user tapo --pass secret [--port 554] [--protocol rtsp]`
  - Stores/updates camera in `~/.config/camsnap/config.yaml`.
- `camsnap list`
  - Shows saved cameras and derived RTSP URLs (without passwords in output).
- `camsnap snap --camera cam1 --out cam1.jpg [--timeout 5s]`
  - Uses `ffmpeg` to grab a single frame via RTSP. If `--out` is omitted, writes to a temp file and prints the path.
- `camsnap clip --camera cam1 --dur 10s [--out cam1.mp4] [--timeout 20s]`
  - Uses `ffmpeg` to pull a short segment (copy or transcode later). If `--out` is omitted, writes to a temp file and prints the path.
- `camsnap discover`
  - ONVIF WS-Discovery multicast probe; prints host:port and an example `add` command. `--info` optionally calls GetDeviceInformation (WS-Security UsernameToken, fallback to basic) to show model/fw.
- `camsnap doctor`
  - Checks for ffmpeg in PATH, verifies config exists, attempts TCP reachability to each camera’s port. `--probe` runs a 1s ffmpeg probe per camera with retries and classifies failures (auth vs network).
- `camsnap watch --camera cam1 --action "say motion"` 
  - Uses ffmpeg scene-change detection (`select=gt(scene,threshold)`) to trigger an action; supports threshold/cooldown/duration. Exposes `CAMSNAP_CAMERA`, `CAMSNAP_SCORE`, `CAMSNAP_TIME` env vars to the action; logs either key/value or JSON lines; optional `--action-template` with `{camera},{score},{time}` placeholders.
- `--rtsp-auth auto|basic|digest` available on snap/clip/watch/doctor to force auth preference when devices are picky.
- `camsnap version`

### Architecture
- **CLI**: `spf13/cobra` wired in `cmd/camsnap/main.go`; subcommands live in `internal/cli`.
- **Config**: `internal/config` handles load/save to XDG config dir. YAML via `gopkg.in/yaml.v3`.
- **RTSP helpers**: `internal/rtsp/url.go` builds safe RTSP URLs with auth and ports.
- **Media execution**: `internal/exec/ffmpeg.go` wraps `ffmpeg` calls with timeouts.
- **Motion (future)**: `internal/motion` placeholder; will plug in frame diff or gocv later.

### Tooling
- Go 1.25; `gofmt`/`goimports`.
- `golangci-lint` with a focused rule set (vet, staticcheck, errcheck, gofmt, goimports).
- `go test ./...`.
- Makefile shortcuts: `fmt`, `lint`, `test`, `all`.
- External binaries: `ffmpeg` available in `PATH` for `snap`/`clip`; CLI checks and fails fast if missing.


### Data model (config.yaml)
```yaml
cameras:
  - name: porch
    host: 192.168.1.50
    port: 554
    protocol: rtsp
    username: tapo
    password: secret
```

### Security notes
- Config stored locally, unencrypted; suitable for single-user hosts. Document before multi-user packaging.
- Passwords are never echoed in list output; RTSP URL building includes auth only when constructing commands.

### Next steps after MVP
- ONVIF discovery (`github.com/use-go/onvif` or maintained fork) to auto-add cameras.
- Ubiquiti Protect integration (local API token) with RTSPS URLs.
- Motion detection with gocv frame diff; action hooks (run command, webhook, save clip).
- Preflight checks for `ffmpeg` presence and camera reachability.
