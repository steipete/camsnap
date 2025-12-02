package exec

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// RunFFmpeg executes ffmpeg with a timeout, returning combined output on error.
func RunFFmpeg(ctx context.Context, args ...string) error {
	_, err := RunFFmpegWithOutput(ctx, args...)
	return err
}

// RunFFmpegWithOutput returns combined stdout/stderr output alongside error.
func RunFFmpegWithOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("ffmpeg %v failed: %w\n%s", args, err, string(output))
	}
	return string(output), nil
}

// WithTimeout returns a derived context that times out.
func WithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, d)
}

// HasBinary reports whether a named executable is available in PATH.
func HasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// ClassifyError inspects ffmpeg stderr to differentiate auth vs network errors.
func ClassifyError(stderr string) string {
	lower := strings.ToLower(stderr)
	switch {
	case strings.Contains(lower, "401") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "auth"):
		return "auth"
	case strings.Contains(lower, "connection refused"):
		return "network-refused"
	case strings.Contains(lower, "timed out") || strings.Contains(lower, "timeout"):
		return "network-timeout"
	case strings.Contains(lower, "not found"):
		return "not-found"
	default:
		return "unknown"
	}
}
