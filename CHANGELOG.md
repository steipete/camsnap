# Changelog

## 0.2.3 - Unreleased
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
