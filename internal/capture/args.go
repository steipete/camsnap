package capture

import (
	"fmt"
	"time"
)

// SnapArgs returns ffmpeg arguments for a single-frame capture.
func SnapArgs(options Options, outputPath string) []string {
	return []string{
		"-y",
		"-rtsp_transport", options.Transport,
		"-i", options.URL,
		"-frames:v", "1",
		"-q:v", "2",
		outputPath,
	}
}

// ClipArgs returns ffmpeg arguments for a timed clip capture.
func ClipArgs(options Options, duration time.Duration, outputPath string) []string {
	args := []string{
		"-y",
		"-rtsp_transport", options.Transport,
		"-i", options.URL,
		"-t", fmt.Sprintf("%.0f", duration.Seconds()),
		"-c:v", "copy",
	}
	if options.NoAudio {
		args = append(args, "-an")
	} else {
		args = append(args, "-c:a", options.AudioCodec)
	}
	return append(args, outputPath)
}

// WatchArgs returns ffmpeg arguments for scene-change detection.
func WatchArgs(options Options, threshold float64) []string {
	return []string{
		"-hide_banner",
		"-loglevel", "info",
		"-rtsp_transport", options.Transport,
		"-i", options.URL,
		"-an",
		"-sn",
		"-dn",
		"-vf", fmt.Sprintf("select='gt(scene\\,%0.3f)',metadata=print", threshold),
		"-f", "null",
		"-",
	}
}

// ProbeArgs returns ffmpeg arguments for a short RTSP health probe.
func ProbeArgs(options Options) []string {
	return []string{
		"-hide_banner",
		"-loglevel", "error",
		"-rtsp_transport", options.Transport,
		"-i", options.URL,
		"-t", "1",
		"-f", "null",
		"-",
	}
}
