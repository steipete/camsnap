// Package mediaexec wraps external command execution helpers.
package mediaexec

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const ffmpegLogTailLines = 20

var rtspURLPattern = regexp.MustCompile(`(?i)rtsps?://[^\s]+`)

// RunFFmpeg executes ffmpeg with a timeout, returning combined output on error.
func RunFFmpeg(ctx context.Context, args ...string) error {
	_, err := RunFFmpegWithOutput(ctx, args...)
	return err
}

// RunFFmpegWithOutput returns combined stdout/stderr output alongside error.
func RunFFmpegWithOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	safeOutput := redactRTSPCredentials(string(output))
	if err != nil {
		safeArgs := make([]string, len(args))
		for i, arg := range args {
			safeArgs[i] = redactRTSPCredentials(arg)
		}
		return safeOutput, fmt.Errorf("ffmpeg %v failed: %w\n%s", safeArgs, err, safeOutput)
	}
	return safeOutput, nil
}

// RunFFmpegWithStderrLines streams ffmpeg stderr to a callback. It returns a
// short log tail, an ffmpeg exit error, and any process setup or log read error.
func RunFFmpegWithStderrLines(ctx context.Context, args []string, onLine func(string)) (string, error, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", nil, fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("start ffmpeg: %w", err)
	}

	logTail := make([]string, 0, ffmpegLogTailLines)
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		logTail = append(logTail, line)
		if len(logTail) > ffmpegLogTailLines {
			logTail = logTail[1:]
		}
		if onLine != nil {
			onLine(line)
		}
	}
	if err := scanner.Err(); err != nil {
		_ = cmd.Wait()
		return strings.Join(logTail, "\n"), nil, fmt.Errorf("read ffmpeg logs: %w", err)
	}
	return strings.Join(logTail, "\n"), cmd.Wait(), nil
}

func redactRTSPCredentials(text string) string {
	return rtspURLPattern.ReplaceAllStringFunc(text, func(rawURL string) string {
		schemeEnd := strings.Index(rawURL, "://")
		if schemeEnd < 0 {
			return rawURL
		}
		authorityStart := schemeEnd + len("://")
		tail := rawURL[authorityStart:]
		authorityEnd := len(tail)
		if i := strings.IndexAny(tail, "/?#"); i >= 0 {
			authorityEnd = i
		}
		at := strings.LastIndex(tail[:authorityEnd], "@")
		if at < 0 {
			return rawURL
		}
		return rawURL[:authorityStart] + "REDACTED@" + tail[at+1:]
	})
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
