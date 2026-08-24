# Local webcams

camsnap captures local webcams on macOS and Linux. Windows builds support RTSP cameras but not local video devices, and the Docker image does not expose direct local-device capture.

## List devices

```sh
camsnap devices
camsnap devices --json
```

Official macOS builds use native AVFoundation and show `INDEX`, `ID`, `NAME`, and `DEFAULT`. A macOS device selector can be its unique ID, case-insensitive name, or listed integer index. Prefer the unique ID or name in saved configuration: native and ffmpeg device indices can differ, and indices can change when hardware is attached or removed.

Linux and macOS builds without cgo use ffmpeg-backed enumeration. Linux selectors use `/dev/videoN` paths.

## Save a local camera

On macOS, copy the stable ID from `camsnap devices`:

```sh
camsnap add --name desk --protocol local \
  --device '<avfoundation-unique-id>' --local-backend native
camsnap snap desk --out desk.jpg
camsnap clip desk --dur 5s --out desk.mp4
camsnap watch desk --threshold 0.2 --action 'touch /tmp/camsnap-motion'
```

Use a device directly when you do not need saved configuration:

```sh
camsnap snap --device 0 --framerate 30 --video-size 1280x720 \
  --warmup 1s --out webcam.jpg
camsnap snap --device 0 --local-backend ffmpeg --out ffmpeg-webcam.jpg
camsnap clip --device /dev/video0 --dur 5s --out webcam.mp4
```

`--local-backend native|ffmpeg` mirrors the saved `local_backend` setting. Native is the default when compiled into a cgo-enabled macOS build; ffmpeg is the default elsewhere and always handles `clip` and `watch`. Selecting `native` in a build without that backend returns an error.

In a native macOS build, bare `camsnap snap` selects the device marked `DEFAULT`. The command does not guess a device when the ffmpeg backend is selected, and `clip` and `watch` always require a camera or device.

Local snapshots warm up the camera before keeping the final frame so auto-exposure can settle. Local clips encode H.264 video without requesting microphone access.

## macOS Camera permission

The signed native build requests Camera permission itself. camsnap validates the device selector first, so an invalid `--device` reports the available choices before macOS displays a permission prompt.

For terminal launches, grant Camera access to the launching terminal in **System Settings → Privacy & Security → Camera**. An SSH session cannot display the permission prompt; if access was previously denied, run the reset locally and retry from a local terminal:

```sh
tccutil reset Camera
```

Continuity Camera appears only while the iPhone is nearby and unlocked.

## Pan, tilt, and zoom

On macOS, `camsnap ptz` controls USB webcams that advertise standard UVC camera-terminal pan, tilt, or zoom controls. It accepts the same native index, stable AVFoundation ID, or camera name as `snap --device`; omitting `--device` selects the default camera.

```sh
camsnap ptz status --device 0
camsnap ptz goto --device 0 --pan 12.5 --tilt -5 --zoom 50
camsnap ptz move --device 0 --pan -10 --zoom 5
camsnap ptz home --device 0
camsnap ptz goto --device 0 --pan 45 --settle 3s --timeout 6s
```

Some gimbal webcams service UVC controls only while their video stream is active: without a stream, position reads can be stale and accepted movement commands can be silently ignored. Every `ptz` subcommand starts a temporary AVFoundation capture session before accessing UVC, keeps the stream active throughout the operation, and stops it afterward without saving a frame. This requires the same macOS Camera permission as native snapshots.

`goto` uses absolute pan and tilt angles in degrees and zoom from 0–100 percent. `move` takes degree deltas and zoom percentage-point deltas. Values are clamped and snapped to the ranges reported by the camera, and the command prints the observed positions after they stabilize. `home` uses each control's UVC default, falling back to zero pan/tilt and minimum zoom when a device does not report defaults. Every subcommand supports `--json`.

The motion commands accept `--settle` (default `2s`) to give the gimbal time to reach its target and `--timeout` (default `5s`) for the overall verification wait. Verification uses a fresh UVC connection that never issued the movement command, because the original connection can echo an uncommitted setpoint even when the camera did not move. If the observed position differs from the requested target by more than the camera's reported control resolution, the command fails. Confirm that the camera can stream and disable any on-camera AI framing or tracking that overrides manual positioning before retrying.

The status table shows raw UVC ranges: pan and tilt use arcseconds, while zoom units are device-specific. Relative moves are implemented as a current-position read followed by a clamped absolute write because native UVC relative-speed controls vary between devices.

PTZ requires a cgo-enabled macOS build and a directly attached USB UVC camera with absolute controls. Built-in cameras, Studio Display cameras, Continuity Camera, and devices that do not expose UVC PTZ controls return a named unsupported-camera error.

## Building the native backend

`make build` embeds the Camera usage-description plist and applies an ad-hoc signature. Set `CAMSNAP_CODESIGN_IDENTITY` to use a local Developer ID identity instead.

```sh
make build
otool -s __TEXT __info_plist ./camsnap
codesign --verify --verbose ./camsnap
```

Official macOS release artifacts carry the same plist and are signed in the release workflow.
