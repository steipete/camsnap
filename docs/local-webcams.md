# Local webcams

camsnap captures local webcams on macOS and Linux. Windows builds support RTSP cameras but not local video devices, and the Docker image does not expose direct local-device capture.

## List devices

```sh
camsnap devices
camsnap devices --json
```

Official macOS builds use native AVFoundation and show `INDEX`, `ID`, `NAME`, and `DEFAULT`. A macOS device selector can be its unique ID, case-insensitive name, or listed integer index. Prefer the unique ID or name in saved configuration: native and ffmpeg device indices can differ, and indices can change when hardware is attached or removed.

Linux queries V4L2 capture capabilities directly, listing nodes that permit read/write access and support streaming in either single-planar or multi-planar mode, excluding ISP subdevices and metadata/output nodes. Select a camera by `/dev/videoN`, its numeric node index, or a case-insensitive unambiguous card name. Stable `/dev/v4l/by-id/...` paths work for capture and PTZ and are preferable in saved configuration. macOS builds without cgo use ffmpeg-backed enumeration.

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

Bare `camsnap snap` selects the default local camera in a native macOS build and the first accessible V4L2 capture device on Linux. Explicitly selecting `--local-backend ffmpeg` requires a camera or device, as do `clip` and `watch`.

Local snapshots warm up the camera before keeping the final frame so auto-exposure can settle. Local clips encode H.264 video without requesting microphone access. Linux capture uses wall-clock arrival timestamps so stale timestamps from a V4L2 relay do not break warmup or clip duration. If access is denied, inspect `getfacl /dev/videoN` and ensure your active desktop session has camera access.

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

Some gimbal webcams service UVC controls only while their video stream is active: without a stream, position reads can be stale and accepted movement commands can be silently ignored. On macOS, every `ptz` subcommand starts a temporary AVFoundation capture session before accessing UVC, keeps the stream active throughout the operation, and stops it afterward without saving a frame. This requires the same macOS Camera permission as native snapshots.

`goto` uses absolute pan and tilt angles in degrees and zoom from 0–100 percent. `move` takes degree deltas and zoom percentage-point deltas. Values are clamped and snapped to the ranges reported by the camera, and the command prints the observed positions after they stabilize. `home` uses each control's UVC default, falling back to zero pan/tilt and minimum zoom when a device does not report defaults. Every subcommand supports `--json`.

The motion commands accept `--settle` (default `2s`) to give the gimbal time to reach its target and `--timeout` (default `5s`) for the overall verification wait. Verification uses a fresh UVC connection that never issued the movement command, because the original connection can echo an uncommitted setpoint even when the camera did not move. Repeated intermediate positions are polled until the target is reached or the timeout expires. If the final observed position differs from the requested target by more than the camera's reported control resolution, the command fails. Confirm that the camera can stream and disable any on-camera AI framing or tracking that overrides manual positioning before retrying.

The status table shows raw UVC ranges: pan and tilt use arcseconds, while zoom units are device-specific. Relative moves are implemented as a current-position read followed by a clamped absolute write because native UVC relative-speed controls vary between devices.

On macOS, PTZ requires a cgo-enabled build and a directly attached USB UVC camera with absolute controls. Built-in cameras, Studio Display cameras, Continuity Camera, and devices that do not expose UVC PTZ controls return a named unsupported-camera error.

On Linux, PTZ uses standard writable V4L2 absolute controls on the video node, without cgo, separate USB access, or an AVFoundation stream. Pan and tilt are supported independently when only one axis is advertised. JSON includes `pan_absolute` and `tilt_absolute` for supported Linux axes; `pan_tilt_absolute` continues to mean both axes. Pan and tilt use arcseconds; zoom uses the reported device range. The same degree/percentage flags and fresh-connection readback apply. Physical motion is not yet hardware-verified: please [report](https://github.com/steipete/camsnap/issues) your camera model, advertised controls, and motion results. A driver readback alone does not prove physical movement.

## Capture a pan sweep

On macOS, `camsnap sweep` moves an absolute-pan/tilt USB camera through evenly spaced positions and captures a JPEG at each stop. It keeps one native AVFoundation session streaming throughout the entire sweep, including movement, fresh-connection verification, and frame capture:

```sh
camsnap sweep --device 0 --from -45 --to 45 --steps 7 --out-dir panorama
camsnap sweep desk --from -60 --to 60 --steps 9 --tilt -5 \
  --settle 3s --timeout 6s --out-dir desk-panorama --json
camsnap sweep --device 0 --from 30 --to -30 --steps 5 \
  --out-dir reverse-panorama --fail-fast
```

The start and end positions are clamped and snapped to the camera's reported pan range and resolution. `--tilt` sets a fixed, clamped tilt for every frame; when omitted, the sweep preserves the current tilt. `--steps` must be at least two and includes both endpoints. `--warmup` applies once before the first frame; `--video-size` and `--framerate` follow the existing native-snapshot behavior. Sweeps require the native backend because ffmpeg cannot capture from the same already-open AVFoundation stream.

Each output directory contains stable names such as `step-000-pan--45.000.jpg` and `manifest.json`. The manifest includes the selected device, effective start/end/tilt angles, requested step count, and one entry per captured frame:

```json
{
  "index": 0,
  "requested": {
    "pan": { "degrees": -45, "raw": -162000 },
    "tilt": { "degrees": -5, "raw": -18000 }
  },
  "observed": {
    "pan": { "degrees": -45, "raw": -162000 },
    "tilt": { "degrees": -5, "raw": -18000 }
  },
  "frame_path": "panorama/step-000-pan--45.000.jpg",
  "verified": true
}
```

A position that misses the camera's reported control resolution is still photographed and recorded with `"verified": false` and a `verification_error`. By default the remaining steps continue, then the command exits non-zero and names the failed step indices. `--fail-fast` stops after capturing the first failed step and still writes the partial manifest. `--json` prints the exact same manifest bytes that are saved to disk.

## Building the native backend

`make build` embeds the Camera usage-description plist and applies an ad-hoc signature. Set `CAMSNAP_CODESIGN_IDENTITY` to use a local Developer ID identity instead.

```sh
make build
otool -s __TEXT __info_plist ./camsnap
codesign --verify --verbose ./camsnap
```

Official macOS release artifacts carry the same plist and are signed in the release workflow.
