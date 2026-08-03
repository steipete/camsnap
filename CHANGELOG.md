# Changelog

## 0.3.1 - Unreleased
- Rewrite the README around installation, first capture, and focused camera setup guides.

## 0.3.0 - 2026-08-02
- Show native webcam indices in the `devices` table alongside stable IDs, names, and default status.
- Validate native webcam selectors before requesting macOS Camera permission.
- Match native webcam names case-insensitively and report available or ambiguous matches.
- Use the default native macOS webcam when `snap` is run without a camera or device.
- Sort ffmpeg AVFoundation device indices numerically.
- Fix native-to-ffmpeg snapshot fallback selecting the wrong camera when backend device orders differ.
- Add native AVFoundation snapshots, device enumeration, and Camera permission handling to cgo-enabled macOS builds, plus plist-embedded and signed Darwin release artifacts.
- Add first-class local webcam capture through macOS AVFoundation and Linux v4l2, including device enumeration, warm snapshot capture, video-only clips, motion watch, doctor probes, and TCC-aware errors.
- Fix capture commands ignoring per-camera defaults when their flags had non-empty application defaults (PR #9).
- Redact RTSP credentials from ffmpeg failures, restore private camera-config permissions on rewrite, and safely finalize gortsplib frame buffers.
- Migrate the gortsplib RTSP backend to the maintained v5 release, adopt Go 1.26, and refresh runtime dependencies.

## 0.2.2
- Homebrew: install target-specific release binaries on macOS and Linux.

## 0.2.1
- Add Docker support with multi-arch GHCR publishing.
- Add GoReleaser-based release automation for GitHub releases, Homebrew tap updates, and linux/arm64 artifacts.
- Fix custom RTSP paths like `/av_stream/ch0` being duplicated when used by snap/clip/watch.
- Update Go dependencies and move the source build baseline to Go 1.25.
- Refresh release docs for Homebrew install verification, arm64 artifacts, and tap updates.

## 0.2.0
- Fix custom RTSP paths like `/av_stream/ch0` being duplicated when used by snap/clip/watch.
- Add explicit `path` support to store tokenized RTSP URLs (e.g., UniFi Protect) and wire it through add/snap/clip/watch.
- Preserve legacy stream handling while allowing custom paths and per-camera defaults.
- Document Protect setup and path usage; expanded README examples.

## 0.1.0
- Initial CLI: add/list cameras; snap; clip; motion watch; discover; doctor.
- Per-camera defaults for RTSP transport, stream, client, audio handling.
- Positional camera names; temp output when `--out` omitted.
- RTSP helper and config persistence with tests.
- gortsplib fallback client and Tapo-friendly UDP/stream controls.
- Colorized TTY output; lint/test Makefile; updated dependencies.
- Config now uses XDG path `~/.config/camsnap/config.yaml`.
