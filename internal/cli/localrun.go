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
	notice    func(...any)
}

const cameraPermissionRemediation = "Grant Camera access in System Settings → Privacy & Security → Camera. For terminal launches, grant access to the launching terminal. Over SSH there is no permission prompt; run `tccutil reset Camera` locally to re-prompt."

// runLocalCapture is the single execution seam for local camera captures.
func runLocalCapture(ctx context.Context, request localCaptureRequest) error {
	backend := request.options.LocalBackend
	if backend == "" {
		backend = defaultLocalBackend()
	}
	if backend != capture.LocalBackendNative && backend != capture.LocalBackendFFmpeg {
		return fmt.Errorf("invalid local backend %q (use native|ffmpeg)", backend)
	}
	if backend == capture.LocalBackendNative && !nativeLocalBackendAvailable() {
		return fmt.Errorf("native local capture backend is not available in this build; use --local-backend ffmpeg")
	}

	if request.operation == localSnap && backend == capture.LocalBackendNative {
		resolved, err := prepareNativeSnapshot(ctx, request, nativeResolveCaptureDevice, preflightNativeCamera)
		if err != nil {
			return err
		}
		err = nativeCaptureFrame(resolved, request.options.Warmup, request.output)
		if err == nil {
			return nil
		}
		if isNativePermissionError(err) {
			return fmt.Errorf("native local capture failed: %w\n%s", err, cameraPermissionRemediation)
		}
		if !isNativeSessionFailure(err) || !mediaexec.HasBinary("ffmpeg") {
			return fmt.Errorf("native local capture failed: %w", err)
		}
		if request.notice != nil {
			request.notice(fmt.Sprintf("Native AVFoundation capture failed (%v); falling back to ffmpeg.", err))
		}
		request = withFFmpegFallbackDevice(request, nativeFFmpegFallbackSelector(resolved))
	}

	if request.operation != localSnap {
		if err := failIfNativeCameraDenied(); err != nil {
			return err
		}
	}
	return runLocalFFmpeg(ctx, request)
}

func prepareNativeSnapshot(
	ctx context.Context,
	request localCaptureRequest,
	resolve func(string) (localDevice, error),
	preflight func(context.Context, bool, func(...any)) error,
) (localDevice, error) {
	resolved, err := resolve(request.options.Device)
	if err != nil {
		return localDevice{}, fmt.Errorf("native local capture failed: %w", err)
	}
	if err := preflight(ctx, true, request.notice); err != nil {
		return localDevice{}, err
	}
	return resolved, nil
}

func withFFmpegFallbackDevice(request localCaptureRequest, device string) localCaptureRequest {
	request.options.Device = device
	return request
}

func runLocalFFmpeg(ctx context.Context, request localCaptureRequest) error {
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
		details += "\n" + cameraPermissionRemediation
	}
	return fmt.Errorf("local capture failed (%s): %w%s", class, err, details)
}

func preflightNativeCamera(ctx context.Context, requestAccess bool, notice func(...any)) error {
	status, err := nativeCameraAuthorizationStatus()
	if err != nil {
		return err
	}
	switch status {
	case "authorized":
		return nil
	case "notDetermined":
		if !requestAccess {
			return nil
		}
		if notice != nil {
			notice("A macOS Camera permission dialog may have appeared; waiting for your response.")
		}
		granted, requestErr := nativeRequestCameraAccess(ctx)
		if requestErr != nil {
			return fmt.Errorf("request camera access: %w", requestErr)
		}
		if granted {
			return nil
		}
		return fmt.Errorf("camera access was not granted\n%s", cameraPermissionRemediation)
	case "denied", "restricted":
		return fmt.Errorf("camera access is %s\n%s", status, cameraPermissionRemediation)
	default:
		return fmt.Errorf("camera authorization status is %s", status)
	}
}

func failIfNativeCameraDenied() error {
	if !nativeLocalBackendAvailable() {
		return nil
	}
	status, err := nativeCameraAuthorizationStatus()
	if err != nil {
		return nil
	}
	if status == "denied" || status == "restricted" {
		return fmt.Errorf("camera access is %s\n%s", status, cameraPermissionRemediation)
	}
	return nil
}

func isNativePermissionError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "camera access") || strings.Contains(message, "not permitted") || strings.Contains(message, "permission")
}

func isNativeSessionFailure(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "create device input") ||
		strings.Contains(message, "capture session") ||
		strings.Contains(message, "timed out waiting for a video frame") ||
		strings.Contains(message, "capture frame:")
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
