package capture

import (
	"fmt"
	"strconv"
	"time"
)

// InputArgs returns source-specific ffmpeg input arguments.
func InputArgs(options Options, goos string) ([]string, error) {
	if options.Kind == KindRTSP {
		return []string{"-rtsp_transport", options.Transport, "-i", options.URL}, nil
	}

	var args []string
	switch goos {
	case "darwin":
		args = []string{"-f", "avfoundation", "-framerate", strconv.Itoa(options.Framerate)}
	case "linux":
		args = []string{"-f", "v4l2"}
		if options.Framerate > 0 {
			args = append(args, "-framerate", strconv.Itoa(options.Framerate))
		}
	default:
		return nil, fmt.Errorf("local webcam capture is unsupported on %s", goos)
	}
	if options.VideoSize != "" {
		args = append(args, "-video_size", options.VideoSize)
	}
	return append(args, "-i", options.Device), nil
}

// SnapArgs returns ffmpeg arguments for a single-frame capture.
func SnapArgs(options Options, outputPath, goos string) ([]string, error) {
	input, err := InputArgs(options, goos)
	if err != nil {
		return nil, err
	}
	args := append([]string{"-y"}, input...)
	if options.Kind == KindLocal {
		return append(args, "-t", formatDuration(options.Warmup), "-update", "1", "-q:v", "2", outputPath), nil
	}
	return append(args, "-frames:v", "1", "-q:v", "2", outputPath), nil
}

// ClipArgs returns ffmpeg arguments for a timed clip capture.
func ClipArgs(options Options, duration time.Duration, outputPath, goos string) ([]string, error) {
	input, err := InputArgs(options, goos)
	if err != nil {
		return nil, err
	}
	args := append([]string{"-y"}, input...)
	args = append(args, "-t", fmt.Sprintf("%.0f", duration.Seconds()))
	if options.Kind == KindLocal {
		return append(args,
			"-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", "veryfast",
			"-movflags", "+faststart", "-an", outputPath,
		), nil
	}
	args = append(args, "-c:v", "copy")
	if options.NoAudio {
		args = append(args, "-an")
	} else {
		args = append(args, "-c:a", options.AudioCodec)
	}
	return append(args, outputPath), nil
}

// WatchArgs returns ffmpeg arguments for scene-change detection.
func WatchArgs(options Options, threshold float64, goos string) ([]string, error) {
	input, err := InputArgs(options, goos)
	if err != nil {
		return nil, err
	}
	args := []string{
		"-hide_banner", "-loglevel", "info",
	}
	args = append(args, input...)
	return append(args,
		"-an",
		"-sn",
		"-dn",
		"-vf", fmt.Sprintf("select='gt(scene\\,%0.3f)',metadata=print", threshold),
		"-f", "null",
		"-",
	), nil
}

// ProbeArgs returns ffmpeg arguments for a short health probe.
func ProbeArgs(options Options, goos string) ([]string, error) {
	input, err := InputArgs(options, goos)
	if err != nil {
		return nil, err
	}
	args := []string{"-hide_banner", "-loglevel", "error"}
	args = append(args, input...)
	if options.Kind == KindLocal {
		return append(args, "-frames:v", "1", "-f", "null", "-"), nil
	}
	return append(args, "-t", "1", "-f", "null", "-"), nil
}

func formatDuration(duration time.Duration) string {
	return strconv.FormatFloat(duration.Seconds(), 'f', -1, 64)
}
