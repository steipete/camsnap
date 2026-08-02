# 📸 camsnap — One command to grab frames, clips, or motion alerts from RTSP/ONVIF cameras and local webcams.

## Install / Run
- Homebrew (installs `ffmpeg` automatically): `brew install steipete/tap/camsnap`
- Requirements for source run: Go 1.26+ and `ffmpeg` on PATH. Local webcams use AVFoundation on macOS and v4l2 on Linux; Windows is not supported.
- Run in-place: `go run ./cmd/camsnap --help`
- Run in Docker: `docker run --rm ghcr.io/steipete/camsnap --help`  
  Mount volumes for persistent config and output:
  ```sh
  docker run --rm -v camsnap-config:/config -v "$PWD":/output \
    ghcr.io/steipete/camsnap snap kitchen --out shot.jpg
  ```
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

## Local webcams

List the local video inputs that ffmpeg can see, then either save one in the camera config or use it ad hoc. On macOS the device can be an AVFoundation index or name; on Linux use a `/dev/videoN` path.

```sh
camsnap devices
camsnap devices --json

# Save a macOS camera, then use the same snap/clip/watch commands as RTSP cameras.
camsnap add --name mbp --protocol local --device 0
camsnap snap mbp --out shot.jpg
camsnap clip mbp --dur 5s --out clip.mp4
camsnap watch mbp --threshold 0.2 --action 'touch /tmp/motion'

# Or capture without adding a camera first.
camsnap snap --device 0 --framerate 30 --video-size 1280x720 --warmup 1s --out shot.jpg
camsnap clip --device /dev/video0 --dur 5s --out clip.mp4
```

Local snapshots warm up the camera before keeping the final JPEG so auto-exposure can settle. Local clips are video-only for now: camsnap encodes H.264 and does not request microphone access.

On macOS, Camera permission belongs to the terminal or application that launches camsnap—not to the `camsnap` binary itself. Grant that launcher access in **System Settings → Privacy & Security → Camera**. An SSH session cannot display the macOS permission prompt; if access was previously denied, run `tccutil reset Camera` locally and retry from the launching terminal. Continuity Camera is listed only while the iPhone is nearby and unlocked.

Direct local-device capture is not supported from the camsnap Docker image. Windows local webcams are also unsupported; RTSP cameras continue to work on both Docker and Windows builds.

## Tapo specifics
- Enable “Third‑Party NVR/RTSP” and set a per‑camera account; disable Privacy Mode.
- TC70 often needs `udp` + `stream2` + `gortsplib` and may require disabling Tapo Care/SD recording to free RTSP streams.
- C225 works with `udp` + `stream1` (ffmpeg client).
- mp4 + PCMA audio can fail; use `--no-audio` or `--audio-codec aac`.

## Behavior notes
- Motion uses ffmpeg scene-change detection; actions can log JSON (`--json`).
- Doctor classifies ffmpeg probe errors (auth, network, local-device, and macOS Camera permission failures).
- Per-camera defaults reduce flag noise for devices with quirks.

## Roadmap
- ONVIF device-info fetch with WS-Security.
- Ubiquiti Protect local API integration.
- Smarter RTSP fallback / retries.
