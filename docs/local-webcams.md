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

## Building the native backend

`make build` embeds the Camera usage-description plist and applies an ad-hoc signature. Set `CAMSNAP_CODESIGN_IDENTITY` to use a local Developer ID identity instead.

```sh
make build
otool -s __TEXT __info_plist ./camsnap
codesign --verify --verbose ./camsnap
```

Official macOS release artifacts carry the same plist and are signed in the release workflow.
