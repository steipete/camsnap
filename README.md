# camsnap 📸 — Frame first, questions later.

[![CI](https://img.shields.io/github/actions/workflow/status/steipete/camsnap/ci.yml?branch=main&style=flat-square&label=ci)](https://github.com/steipete/camsnap/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/steipete/camsnap?style=flat-square)](https://github.com/steipete/camsnap/releases/latest)
[![Platforms](https://img.shields.io/badge/platforms-macOS%20%7C%20Linux%20%7C%20Windows-555?style=flat-square)](#platform-support)
[![License](https://img.shields.io/github/license/steipete/camsnap?style=flat-square)](LICENSE)
[![Homebrew](https://img.shields.io/badge/Homebrew-steipete%2Ftap-FBB040?style=flat-square&logo=homebrew)](https://github.com/steipete/homebrew-tap)

camsnap is a command-line tool for capturing snapshots and short clips from RTSP cameras, discovering ONVIF devices, and running motion-triggered shell actions. It also captures local webcams on macOS and Linux; Windows builds support network cameras.

```console
$ camsnap add --name kitchen --host 192.168.1.50 --user camera --pass 'camera-password'
Added camera "kitchen"
$ camsnap snap kitchen --out kitchen.jpg
```

Save a camera once, then address it by name for snapshots, clips, health checks, and motion detection.

## Install

Homebrew is the smallest install path on macOS and Linux. The formula installs `ffmpeg` as a dependency.

```sh
brew install steipete/tap/camsnap
```

Prebuilt binaries for macOS, Linux, and Windows are available from [GitHub Releases](https://github.com/steipete/camsnap/releases/latest). Install `ffmpeg` separately when using a release binary; Windows supports RTSP cameras but not local webcams.

You can also run camsnap in a container or build it from source. See [Docker usage](docs/docker.md) or [Development](#development).

## Quick start

Add an RTSP camera, confirm the saved configuration, then capture a frame:

```sh
camsnap add --name kitchen --host 192.168.1.50 \
  --user camera --pass 'camera-password'
camsnap list
camsnap snap kitchen --out kitchen.jpg
```

Replace the example host and credentials with values from your camera.

The default RTSP transport is TCP and the default stream is `stream1`. Use camera-specific defaults when needed:

```sh
camsnap add --name kitchen --host 192.168.1.50 \
  --user camera --pass 'camera-password' \
  --rtsp-transport udp --stream stream2 --rtsp-client gortsplib
```

camsnap stores configuration in `~/.config/camsnap/config.yaml`, or `$XDG_CONFIG_HOME/camsnap/config.yaml` when `XDG_CONFIG_HOME` is set. The file contains camera credentials in plain text and is written with user-only permissions. Use `--config` to choose another path.

## Commands

| Command | Purpose |
| --- | --- |
| `add` | Add or update a saved RTSP or local camera. |
| `list` | List saved cameras without exposing passwords. |
| `snap` | Capture one frame to a file. |
| `clip` | Record a short clip. |
| `watch` | Detect scene changes and run a shell action. |
| `discover` | Find ONVIF devices on the local network. |
| `devices` | List local video inputs. |
| `ptz` | Control pan, tilt, and zoom on supported USB webcams. |
| `doctor` | Check `ffmpeg`, connectivity, and optional capture probes. |

Run `camsnap <command> --help` for the complete flags and defaults.

### Clips

camsnap copies the video stream when possible. Drop audio or transcode it to AAC when a camera's source audio is not MP4-compatible.

```sh
camsnap clip kitchen --dur 5s --no-audio --out kitchen.mp4
camsnap clip kitchen --dur 5s --audio-codec aac --out kitchen.mp4
```

When `--out` is omitted, `snap` and `clip` write to a temporary file and print its path.

### Motion actions

`watch` uses ffmpeg scene-change detection and applies a cooldown between actions:

```sh
camsnap watch kitchen --threshold 0.2 --cooldown 5s --json \
  --action 'touch /tmp/camsnap-motion'
```

The action receives `CAMSNAP_CAMERA`, `CAMSNAP_SCORE`, and `CAMSNAP_TIME`. Pass `--duration` to stop after a fixed interval or `--action-template` to interpolate `{camera}`, `{score}`, and `{time}` into the command.

### Discovery and diagnostics

Find ONVIF cameras on the current network, then check saved cameras:

```sh
camsnap discover --info
camsnap doctor --probe
```

`discover --info` attempts ONVIF device-information requests and uses saved credentials for matching hosts. `doctor --probe` classifies authentication, network, local-device, and macOS Camera permission failures.

## Camera sources

### RTSP and RTSPS

Saved cameras can set defaults for transport, stream, RTSP client, audio handling, and an explicit stream path. Command flags override those defaults.

Use `--path` for cameras whose stream URL contains an opaque token instead of `stream1` or `stream2`. See [RTSP camera setup](docs/rtsp-cameras.md) for tokenized UniFi Protect streams and tuning controls.

### Local webcams

On macOS, official builds use native AVFoundation for device listing and snapshots; clips and motion detection use ffmpeg. Linux uses v4l2 through ffmpeg.

```sh
camsnap devices
camsnap snap --device 0 --out webcam.jpg
camsnap ptz status --device 0
camsnap ptz goto --device 0 --pan 12.5 --tilt -5 --zoom 50
camsnap ptz move --device 0 --pan -10 --zoom 5
camsnap ptz home --device 0
camsnap ptz goto --device 0 --pan 45 --settle 3s --timeout 6s
```

PTZ commands keep the selected camera streaming while reading or changing its position, so they require macOS Camera permission. Motion commands verify the observed position after `--settle` (default `2s`) and fail if it does not stabilize before `--timeout` (default `5s`).

See [Local webcams](docs/local-webcams.md) for stable macOS device selectors, Camera permission behavior, UVC PTZ control, Linux device paths, and backend selection.

## Platform support

| Platform | RTSP cameras | Local webcams |
| --- | --- | --- |
| macOS | Yes | Native snapshots; ffmpeg clips and motion |
| Linux | Yes | v4l2 through ffmpeg |
| Windows | Yes | No |
| Docker | Yes | No direct device capture |

## Development

Source builds require Go 1.26 or newer and `ffmpeg` on `PATH`.

```sh
make build
make lint
make test
```

`make build` embeds the Camera usage description and signs the binary on macOS. Run `go run ./cmd/camsnap --help` for an unsigned source invocation that does not access a local camera.

## License

[MIT](LICENSE)
