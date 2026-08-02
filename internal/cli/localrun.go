package cli

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/steipete/camsnap/internal/capture"
	mediaexec "github.com/steipete/camsnap/internal/exec"
)

type localCaptureOperation uint8

const (
	localSnap localCaptureOperation = iota
	localClip
	localWatch
	localProbe
)

type localCaptureRequest struct {
	operation localCaptureOperation
	options   capture.Options
	output    string
	duration  time.Duration
	threshold float64
	onLine    func(string)
}

// runLocalCapture is the single execution seam for local camera captures. A
// future native backend can replace this dispatcher without changing commands.
func runLocalCapture(ctx context.Context, request localCaptureRequest) error {
	var (
		args []string
		err  error
	)
	switch request.operation {
	case localSnap:
		args, err = capture.SnapArgs(request.options, request.output, runtime.GOOS)
	case localClip:
		args, err = capture.ClipArgs(request.options, request.duration, request.output, runtime.GOOS)
	case localWatch:
		args, err = capture.WatchArgs(request.options, request.threshold, runtime.GOOS)
	case localProbe:
		args, err = capture.ProbeArgs(request.options, runtime.GOOS)
	default:
		return fmt.Errorf("unknown local capture operation")
	}
	if err != nil {
		return err
	}

	if request.operation == localWatch {
		var evidence []string
		logTail, exitErr, streamErr := mediaexec.RunFFmpegWithStderrLines(ctx, args, func(line string) {
			if mediaexec.ClassifyError(line) != "unknown" || isSupportedModeLine(line) {
				evidence = append(evidence, line)
			}
			if request.onLine != nil {
				request.onLine(line)
			}
		})
		if streamErr != nil {
			return streamErr
		}
		if exitErr != nil && ctx.Err() == nil {
			evidence = append(evidence, logTail)
			return localCaptureFailure(exitErr, strings.Join(evidence, "\n"))
		}
		return nil
	}

	output, err := mediaexec.RunFFmpegWithOutput(ctx, args...)
	if err != nil {
		return localCaptureFailure(err, output)
	}
	return nil
}

func localCaptureFailure(err error, output string) error {
	class := mediaexec.ClassifyError(output)
	details := ""
	if modes := supportedModeText(output); modes != "" && !strings.Contains(err.Error(), modes) {
		details += "\n" + modes
	}
	if class == "permission" {
		details += "\nGrant Camera access to the launching terminal in System Settings → Privacy & Security → Camera. Over SSH there is no permission prompt; run `tccutil reset Camera` locally to re-prompt."
	}
	return fmt.Errorf("local capture failed (%s): %w%s", class, err, details)
}

func supportedModeText(output string) string {
	lines := strings.Split(output, "\n")
	var relevant []string
	inModes := false
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "selected framerate") && strings.Contains(lower, "not supported") {
			relevant = append(relevant, line)
		}
		if strings.Contains(lower, "supported modes:") {
			inModes = true
			relevant = append(relevant, line)
			continue
		}
		if inModes {
			if strings.TrimSpace(line) == "" {
				break
			}
			relevant = append(relevant, line)
		}
	}
	return strings.Join(relevant, "\n")
}

func isSupportedModeLine(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "supported modes:") ||
		(strings.Contains(lower, "selected framerate") && strings.Contains(lower, "not supported"))
}
