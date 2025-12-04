# 📸 camsnap — One command to grab frames, clips, or motion alerts from your cams (RTSP/ONVIF).

## Install / Run
- Homebrew (installs `ffmpeg` automatically): `brew install steipete/tap/camsnap`
- Requirements for source run: Go 1.22+ and `ffmpeg` on PATH.
- Run in-place: `go run ./cmd/camsnap --help`
- Camera name may be positional (e.g., `camsnap snap kitchen ...`).
- If `--out` is omitted, snap/clip writes to a temp file and prints the path.

## Config
- Stored at `~/.config/camsnap/config.yaml` (XDG).
- Per-camera defaults supported: `rtsp_transport`, `stream`, `rtsp_client`, `no_audio`, `audio_codec`, `path` (for tokenized RTSP such as UniFi Protect).

### Add a camera
```sh
go run ./cmd/camsnap add --name kitchen --host 192.168.0.175 --user tapo --pass 'secret' \
  --rtsp-transport udp --stream stream2 --rtsp-client gortsplib
```
For UniFi Protect (RTSP token), enable RTSP in Protect, copy the stream URL, and add it with the token path:
```sh
go run ./cmd/camsnap add --name ssg15-livingroom --host 192.168.1.1 --port 7447 \
  --protocol rtsp --path Bfy47SNWz9n2WRrw
```

### Snapshot
```sh
go run ./cmd/camsnap snap kitchen --out shot.jpg
# or rely on per-camera defaults; set as needed:
#   --rtsp-transport tcp|udp  --stream stream1|stream2  --rtsp-client ffmpeg|gortsplib
# For Protect tokenized streams:
#   go run ./cmd/camsnap snap ssg15-livingroom --path Bfy47SNWz9n2WRrw --out shot.jpg
# (Longer timeouts like --timeout 20s may help Protect streams deliver the first keyframe.)
```

### Clip
```sh
go run ./cmd/camsnap clip kitchen --dur 5s --no-audio --out clip.mp4
# video is copied; audio can be dropped (--no-audio) or transcoded (--audio-codec aac)
# Protect example:
#   go run ./cmd/camsnap clip ssg15-livingroom --path Bfy47SNWz9n2WRrw --dur 5s --out clip.mp4
```

### Motion watch
```sh
go run ./cmd/camsnap watch kitchen --threshold 0.2 --cooldown 5s \
  --json --action 'touch /tmp/motion-$(date +%s)'
# env passed to action: CAMSNAP_CAMERA, CAMSNAP_SCORE, CAMSNAP_TIME
# Protect example (tokenized path):
#   go run ./cmd/camsnap watch ssg15-livingroom --path Bfy47SNWz9n2WRrw --threshold 0.2 --action 'touch /tmp/motion'
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
