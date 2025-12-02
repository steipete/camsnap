# camsnap

CLI to capture snapshots, short clips, and run motion detection against RTSP/ONVIF cameras (Tapo first, Ubiquiti next).

## Install / Run
- Requirements: Go 1.25+ and `ffmpeg` on PATH.
- Run in-place: `go run ./cmd/camsnap --help`
- Camera name may be positional (e.g., `camsnap snap kitchen ...`).
- If `--out` is omitted, snap/clip writes to a temp file and prints the path.

## Config
- Stored at `/Users/steipete/Library/Application Support/camsnap/config.yaml` on macOS (XDG fallback can be added later).
- Per-camera defaults supported: `rtsp_transport`, `stream`, `rtsp_client`, `no_audio`, `audio_codec`.

### Add a camera
```sh
go run ./cmd/camsnap add --name kitchen --host 192.168.0.175 --user tapo --pass 'secret' \
  --rtsp-transport udp --stream stream2 --rtsp-client gortsplib
```

### Snapshot
```sh
go run ./cmd/camsnap snap kitchen --out shot.jpg
# or rely on per-camera defaults; set as needed:
#   --rtsp-transport tcp|udp  --stream stream1|stream2  --rtsp-client ffmpeg|gortsplib
```

### Clip
```sh
go run ./cmd/camsnap clip kitchen --dur 5s --no-audio --out clip.mp4
# video is copied; audio can be dropped (--no-audio) or transcoded (--audio-codec aac)
```

### Motion watch
```sh
go run ./cmd/camsnap watch kitchen --threshold 0.2 --cooldown 5s \
  --json --action 'touch /tmp/motion-$(date +%s)'
# env passed to action: CAMSNAP_CAMERA, CAMSNAP_SCORE, CAMSNAP_TIME
```

### Discover (ONVIF)
```sh
go run ./cmd/camsnap discover --info
```

### Doctor
```sh
go run ./cmd/camsnap doctor --probe --rtsp-transport udp
```

## Tapo specifics
- Enable “Third‑Party NVR/RTSP” and set a per‑camera account; disable Privacy Mode.
- TC70 often needs `udp` + `stream2` + `gortsplib` and may require disabling Tapo Care/SD recording to free RTSP streams.
- C225 works with `udp` + `stream1` (ffmpeg client).
- mp4 + PCMA audio can fail; use `--no-audio` or `--audio-codec aac`.

## Behavior notes
- Motion uses ffmpeg scene-change detection; actions can log JSON (`--json`).
- Doctor classifies ffmpeg probe errors (auth vs network).
- Per-camera defaults reduce flag noise for devices with quirks.

## Roadmap
- ONVIF device-info fetch with WS-Security.
- Ubiquiti Protect local API integration.
- Smarter RTSP fallback / retries.
