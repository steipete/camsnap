# Changelog

## 0.1.0
- Initial CLI: add/list cameras; snap; clip; motion watch; discover; doctor.
- Per-camera defaults for RTSP transport, stream, client, audio handling.
- Positional camera names; temp output when `--out` omitted.
- RTSP helper and config persistence with tests.
- gortsplib fallback client and Tapo-friendly UDP/stream controls.
- Colorized TTY output; lint/test Makefile; updated dependencies.
- Config now uses XDG path `~/.config/camsnap/config.yaml`.
