# Development

Run `make all` to format, lint, and test the repository. Use `go build ./...` for a full build; the capture commands require `ffmpeg` on `PATH`.

## Package map

- `internal/capture`: capture option resolution and pure ffmpeg argument builders.
- `internal/cli`: Cobra commands, output, and command orchestration.
- `internal/config`: YAML config loading, saving, and camera lookup.
- `internal/discovery`: ONVIF camera discovery and device information.
- `internal/exec`: ffmpeg process execution and error classification.
- `internal/rtsp`: RTSP URL construction and path manipulation.
- `internal/rtspclient`: native RTSP frame capture through gortsplib.

The config file format and all CLI commands, flags, defaults, and output are public contracts **as shipped in tagged releases**; preserve that released behavior during refactors. Surface that has not appeared in any tagged release yet may still change freely before it first ships.
