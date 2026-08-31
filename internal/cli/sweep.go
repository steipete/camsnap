package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/camsnap/internal/capture"
	"github.com/steipete/camsnap/internal/uvc"
)

type sweepOptions struct {
	from, to, tilt float64
	steps          int
	outDir         string
	jsonOutput     bool
	failFast       bool
	capture        captureFlagValues
	motion         ptzMotionOptions
}

type sweepAngle struct {
	Degrees float64 `json:"degrees"`
	Raw     int32   `json:"raw"`
}

type sweepPosition struct {
	Pan  sweepAngle `json:"pan"`
	Tilt sweepAngle `json:"tilt"`
}

type sweepStep struct {
	Index             int           `json:"index"`
	Requested         sweepPosition `json:"requested"`
	Observed          sweepPosition `json:"observed"`
	FramePath         string        `json:"frame_path"`
	Verified          bool          `json:"verified"`
	VerificationError string        `json:"verification_error,omitempty"`
}

type sweepManifest struct {
	Device         localDevice `json:"device"`
	FromDegrees    float64     `json:"from_degrees"`
	ToDegrees      float64     `json:"to_degrees"`
	TiltDegrees    float64     `json:"tilt_degrees"`
	RequestedSteps int         `json:"requested_steps"`
	Steps          []sweepStep `json:"steps"`
}

func newSweepCmd() *cobra.Command {
	options := &sweepOptions{}
	cmd := &cobra.Command{
		Use:   "sweep [camera]",
		Short: "Capture frames across a verified pan/tilt camera sweep",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			for _, flag := range []string{"from", "to"} {
				if !cmd.Flags().Changed(flag) {
					return fmt.Errorf("--%s is required", flag)
				}
			}
			for flag, value := range map[string]float64{"from": options.from, "to": options.to, "tilt": options.tilt} {
				if cmd.Flags().Changed(flag) && (math.IsNaN(value) || math.IsInf(value, 0)) {
					return fmt.Errorf("--%s must be a finite number", flag)
				}
			}
			if options.steps < 2 {
				return fmt.Errorf("--steps must be at least 2")
			}
			if options.outDir == "" {
				return fmt.Errorf("--out-dir is required")
			}
			return validatePTZTiming(&options.motion)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSweep(cmd, args, options)
		},
	}

	cmd.Flags().Float64Var(&options.from, "from", 0, "Starting pan angle in degrees")
	cmd.Flags().Float64Var(&options.to, "to", 0, "Ending pan angle in degrees")
	cmd.Flags().IntVar(&options.steps, "steps", 0, "Number of evenly spaced capture positions (at least 2)")
	cmd.Flags().StringVar(&options.outDir, "out-dir", "", "Directory for captured frames and manifest.json")
	cmd.Flags().Float64Var(&options.tilt, "tilt", 0, "Fixed tilt angle in degrees (default: current camera tilt)")
	cmd.Flags().BoolVar(&options.failFast, "fail-fast", false, "Stop after the first position verification failure")
	cmd.Flags().BoolVar(&options.jsonOutput, "json", false, "Print the sweep manifest as JSON")
	cmd.Flags().StringVar(&options.capture.device, "device", "", "Local video device index, ID, or name (default: default camera)")
	cmd.Flags().IntVar(&options.capture.framerate, "framerate", 30, "Local capture framerate")
	cmd.Flags().StringVar(&options.capture.videoSize, "video-size", "", "Local capture size (e.g., 1280x720)")
	cmd.Flags().DurationVar(&options.capture.warmup, "warmup", time.Second, "One-time local camera auto-exposure warmup")
	cmd.Flags().StringVar(&options.capture.localBackend, "local-backend", "", "Local capture backend (sweeps require native)")
	addPTZTimingFlags(cmd, &options.motion)
	return cmd
}

func runSweep(cmd *cobra.Command, args []string, options *sweepOptions) error {
	camera, _, err := selectCaptureCameraWithDefault(cmd, args, "", options.capture.device, true)
	if err != nil {
		return err
	}
	if !strings.EqualFold(camera.Protocol, "local") {
		return fmt.Errorf("camera %q is not a local camera and does not support absolute UVC pan/tilt control", camera.Name)
	}
	captureOptions, err := capture.Resolve(camera, options.capture.overrides(cmd))
	if err != nil {
		return err
	}
	if captureOptions.LocalBackend == capture.LocalBackendFFmpeg {
		return fmt.Errorf("sweep requires the native local capture backend; ffmpeg cannot reuse the active camera stream")
	}
	if err := os.MkdirAll(options.outDir, 0o755); err != nil {
		return fmt.Errorf("create sweep output directory %q: %w", options.outDir, err)
	}

	device, session, controller, err := openPTZ(cmd.Context(), captureOptions.Device)
	if err != nil {
		return err
	}
	defer func() { _ = session.Close() }()
	defer func() {
		if controller != nil {
			_ = controller.Close()
		}
	}()

	captureSession, ok := session.(interface {
		CaptureFrame(time.Duration, string) error
	})
	if !ok {
		return fmt.Errorf("capture session for camera %q cannot save frames", device.Name)
	}
	capabilities := controller.Capabilities()
	status, err := controller.Status()
	if err != nil {
		return fmt.Errorf("read PTZ status for camera %q: %w", device.Name, err)
	}
	if !capabilities.PanTiltAbsolute || status.Pan == nil || status.Tilt == nil {
		return fmt.Errorf("camera %q does not support absolute UVC pan/tilt control", device.Name)
	}

	from := status.Pan.Range.Clamp(uvc.DegreesToArcsec(options.from))
	to := status.Pan.Range.Clamp(uvc.DegreesToArcsec(options.to))
	tilt := status.Tilt.Cur
	if cmd.Flags().Changed("tilt") {
		tilt = status.Tilt.Range.Clamp(uvc.DegreesToArcsec(options.tilt))
	}
	manifest := sweepManifest{
		Device:         device,
		FromDegrees:    uvc.ArcsecToDegrees(from),
		ToDegrees:      uvc.ArcsecToDegrees(to),
		TiltDegrees:    uvc.ArcsecToDegrees(tilt),
		RequestedSteps: options.steps,
		Steps:          make([]sweepStep, 0, options.steps),
	}
	failed := make([]string, 0)
	indexWidth := max(3, len(strconv.Itoa(options.steps-1)))

	for index := range options.steps {
		if controller == nil {
			controller, err = ptzOpenController(device.ID)
			if err != nil {
				return fmt.Errorf("open PTZ control connection for camera %q: %w", device.Name, err)
			}
		}
		pan := int32(math.Round(float64(from) + float64(int64(to)-int64(from))*float64(index)/float64(options.steps-1)))
		appliedPan, appliedTilt, setErr := controller.SetPanTilt(pan, tilt)
		if setErr != nil {
			return fmt.Errorf("set pan/tilt for camera %q at sweep step %d: %w", device.Name, index, setErr)
		}
		if err := controller.Close(); err != nil {
			return fmt.Errorf("close PTZ control connection for camera %q before verification: %w", device.Name, err)
		}
		controller = nil

		target := ptzTarget{pan: appliedPan, tilt: appliedTilt, checkPan: true, checkTilt: true}
		observed, verificationErr := settlePTZMotion(cmd.Context(), ptzOpenController, device, target, &options.motion)
		if observed.Pan == nil || observed.Tilt == nil {
			if verificationErr != nil {
				return verificationErr
			}
			return fmt.Errorf("read applied PTZ status for camera %q at sweep step %d: pan or tilt is unavailable", device.Name, index)
		}

		requested := sweepPosition{
			Pan:  sweepAngle{Degrees: uvc.ArcsecToDegrees(appliedPan), Raw: appliedPan},
			Tilt: sweepAngle{Degrees: uvc.ArcsecToDegrees(appliedTilt), Raw: appliedTilt},
		}
		framePath := filepath.Join(options.outDir, fmt.Sprintf("step-%0*d-pan-%+.3f.jpg", indexWidth, index, requested.Pan.Degrees))
		warmup := time.Duration(0)
		if index == 0 {
			warmup = captureOptions.Warmup
		}
		if err := captureSession.CaptureFrame(warmup, framePath); err != nil {
			return fmt.Errorf("capture sweep frame for camera %q at step %d: %w", device.Name, index, err)
		}
		step := sweepStep{
			Index:     index,
			Requested: requested,
			Observed: sweepPosition{
				Pan:  sweepAngle{Degrees: uvc.ArcsecToDegrees(observed.Pan.Cur), Raw: observed.Pan.Cur},
				Tilt: sweepAngle{Degrees: uvc.ArcsecToDegrees(observed.Tilt.Cur), Raw: observed.Tilt.Cur},
			},
			FramePath: framePath,
			Verified:  verificationErr == nil,
		}
		if verificationErr != nil {
			step.VerificationError = verificationErr.Error()
			failed = append(failed, strconv.Itoa(index))
			cmd.PrintErrf("Sweep step %d verification failed: %v\n", index, verificationErr)
		}
		manifest.Steps = append(manifest.Steps, step)
		if verificationErr != nil && options.failFast {
			break
		}
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode sweep manifest: %w", err)
	}
	data = append(data, '\n')
	manifestPath := filepath.Join(options.outDir, "manifest.json")
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		return fmt.Errorf("write sweep manifest %q: %w", manifestPath, err)
	}
	if options.jsonOutput {
		if _, err := cmd.OutOrStdout().Write(data); err != nil {
			return fmt.Errorf("write sweep manifest: %w", err)
		}
	} else {
		cmd.Printf("Captured %d sweep frames in %s (manifest: %s)\n", len(manifest.Steps), options.outDir, manifestPath)
	}
	if len(failed) > 0 {
		return fmt.Errorf("sweep verification failed for steps %s", strings.Join(failed, ", "))
	}
	return nil
}
