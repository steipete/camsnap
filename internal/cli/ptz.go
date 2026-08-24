package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/camsnap/internal/uvc"
)

type ptzController interface {
	Capabilities() uvc.Capabilities
	Status() (uvc.Status, error)
	SetPanTilt(pan, tilt int32) (int32, int32, error)
	SetZoom(zoom int32) (int32, error)
	Home() (uvc.Status, error)
	Close() error
}

type ptzOptions struct {
	device     string
	jsonOutput bool
}

type ptzMotionOptions struct {
	settle  time.Duration
	timeout time.Duration
}

type ptzTarget struct {
	pan, tilt, zoom                int32
	checkPan, checkTilt, checkZoom bool
}

type ptzAngleOutput struct {
	Degrees float64   `json:"degrees"`
	Raw     int32     `json:"raw"`
	Range   uvc.Range `json:"range"`
}

type ptzZoomOutput struct {
	Percent float64   `json:"percent"`
	Raw     int32     `json:"raw"`
	Range   uvc.Range `json:"range"`
}

type ptzStatusOutput struct {
	Device       localDevice      `json:"device"`
	Capabilities uvc.Capabilities `json:"capabilities"`
	Pan          *ptzAngleOutput  `json:"pan,omitempty"`
	Tilt         *ptzAngleOutput  `json:"tilt,omitempty"`
	Zoom         *ptzZoomOutput   `json:"zoom,omitempty"`
}

var (
	ptzResolveDevice  = resolveNativePTZDevice
	ptzOpenSession    = openNativePTZSession
	ptzOpenController = openNativePTZController
	ptzNow            = time.Now
	ptzSleep          = time.Sleep
)

const ptzPollInterval = 100 * time.Millisecond

func newPTZCmd() *cobra.Command {
	options := &ptzOptions{}
	cmd := &cobra.Command{
		Use:   "ptz",
		Short: "Control pan, tilt, and zoom on a local USB camera",
	}
	cmd.PersistentFlags().StringVar(&options.device, "device", "", "Local video device index, ID, or name (default: default camera)")
	cmd.PersistentFlags().BoolVar(&options.jsonOutput, "json", false, "Print PTZ status as JSON")
	cmd.AddCommand(
		newPTZStatusCmd(options),
		newPTZGotoCmd(options),
		newPTZMoveCmd(options),
		newPTZHomeCmd(options),
	)
	return cmd
}

func newPTZStatusCmd(options *ptzOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show PTZ capabilities, positions, and ranges",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			device, session, controller, err := openPTZ(cmd.Context(), options.device)
			if err != nil {
				return err
			}
			defer func() { _ = session.Close() }()
			defer func() { _ = controller.Close() }()

			status, err := controller.Status()
			if err != nil {
				return fmt.Errorf("read PTZ status for camera %q: %w", device.Name, err)
			}
			return writePTZStatus(cmd.OutOrStdout(), options.jsonOutput, makePTZStatusOutput(device, controller.Capabilities(), status))
		},
	}
}

func newPTZGotoCmd(options *ptzOptions) *cobra.Command {
	var pan, tilt, zoom float64
	motion := &ptzMotionOptions{}
	cmd := &cobra.Command{
		Use:   "goto",
		Short: "Move to absolute pan, tilt, or zoom positions",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := validatePTZMotionFlags(cmd, pan, tilt, zoom); err != nil {
				return err
			}
			return validatePTZTiming(motion)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPTZMotion(cmd, options, motion, false, pan, tilt, zoom)
		},
	}
	cmd.Flags().Float64Var(&pan, "pan", 0, "Absolute pan angle in degrees")
	cmd.Flags().Float64Var(&tilt, "tilt", 0, "Absolute tilt angle in degrees")
	cmd.Flags().Float64Var(&zoom, "zoom", 0, "Absolute zoom position as a percentage")
	addPTZTimingFlags(cmd, motion)
	return cmd
}

func newPTZMoveCmd(options *ptzOptions) *cobra.Command {
	var pan, tilt, zoom float64
	motion := &ptzMotionOptions{}
	cmd := &cobra.Command{
		Use:   "move",
		Short: "Move by relative pan, tilt, or zoom deltas",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := validatePTZMotionFlags(cmd, pan, tilt, zoom); err != nil {
				return err
			}
			return validatePTZTiming(motion)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPTZMotion(cmd, options, motion, true, pan, tilt, zoom)
		},
	}
	cmd.Flags().Float64Var(&pan, "pan", 0, "Relative pan delta in degrees")
	cmd.Flags().Float64Var(&tilt, "tilt", 0, "Relative tilt delta in degrees")
	cmd.Flags().Float64Var(&zoom, "zoom", 0, "Relative zoom delta in percentage points")
	addPTZTimingFlags(cmd, motion)
	return cmd
}

func newPTZHomeCmd(options *ptzOptions) *cobra.Command {
	motion := &ptzMotionOptions{}
	cmd := &cobra.Command{
		Use:   "home",
		Short: "Reset pan, tilt, and zoom to their defaults",
		Args:  cobra.NoArgs,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return validatePTZTiming(motion)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			device, session, controller, err := openPTZ(cmd.Context(), options.device)
			if err != nil {
				return err
			}
			defer func() { _ = session.Close() }()
			defer func() {
				if controller != nil {
					_ = controller.Close()
				}
			}()

			capabilities := controller.Capabilities()
			status, err := controller.Home()
			if err != nil {
				return fmt.Errorf("home camera %q: %w", device.Name, err)
			}
			target := ptzTarget{}
			if status.Pan != nil {
				target.pan = status.Pan.Range.Clamp(status.Pan.Range.Def)
				target.checkPan = true
			}
			if status.Tilt != nil {
				target.tilt = status.Tilt.Range.Clamp(status.Tilt.Range.Def)
				target.checkTilt = true
			}
			if status.Zoom != nil {
				target.zoom = status.Zoom.Range.Clamp(status.Zoom.Range.Def)
				target.checkZoom = true
			}
			if err := controller.Close(); err != nil {
				return fmt.Errorf("close PTZ control connection for camera %q before verification: %w", device.Name, err)
			}
			controller = nil
			status, err = settlePTZMotion(cmd.Context(), ptzOpenController, device, target, motion)
			if err != nil {
				return err
			}
			return writePTZStatus(cmd.OutOrStdout(), options.jsonOutput, makePTZStatusOutput(device, capabilities, status))
		},
	}
	addPTZTimingFlags(cmd, motion)
	return cmd
}

func addPTZTimingFlags(cmd *cobra.Command, options *ptzMotionOptions) {
	cmd.Flags().DurationVar(&options.settle, "settle", 2*time.Second, "Time to allow the camera position to settle")
	cmd.Flags().DurationVar(&options.timeout, "timeout", 5*time.Second, "Overall timeout for verifying the camera position")
}

func validatePTZTiming(options *ptzMotionOptions) error {
	if options.settle < 0 {
		return fmt.Errorf("--settle must not be negative")
	}
	if options.timeout <= 0 {
		return fmt.Errorf("--timeout must be greater than zero")
	}
	if options.settle >= options.timeout {
		return fmt.Errorf("--settle must be less than --timeout")
	}
	return nil
}

func validatePTZMotionFlags(cmd *cobra.Command, pan, tilt, zoom float64) error {
	changed := false
	for _, flag := range []string{"pan", "tilt", "zoom"} {
		if cmd.Flags().Changed(flag) {
			changed = true
		}
	}
	if !changed {
		return fmt.Errorf("at least one of --pan, --tilt, or --zoom is required")
	}
	for flag, value := range map[string]float64{"pan": pan, "tilt": tilt, "zoom": zoom} {
		if cmd.Flags().Changed(flag) && (math.IsNaN(value) || math.IsInf(value, 0)) {
			return fmt.Errorf("--%s must be a finite number", flag)
		}
	}
	return nil
}

func runPTZMotion(cmd *cobra.Command, options *ptzOptions, motion *ptzMotionOptions, relative bool, pan, tilt, zoom float64) error {
	device, session, controller, err := openPTZ(cmd.Context(), options.device)
	if err != nil {
		return err
	}
	defer func() { _ = session.Close() }()
	defer func() {
		if controller != nil {
			_ = controller.Close()
		}
	}()

	capabilities := controller.Capabilities()
	status, err := controller.Status()
	if err != nil {
		return fmt.Errorf("read PTZ status for camera %q: %w", device.Name, err)
	}

	panChanged := cmd.Flags().Changed("pan")
	tiltChanged := cmd.Flags().Changed("tilt")
	panTiltChanged := panChanged || tiltChanged
	zoomChanged := cmd.Flags().Changed("zoom")
	if panTiltChanged && (!capabilities.PanTiltAbsolute || status.Pan == nil || status.Tilt == nil) {
		return fmt.Errorf("camera %q does not support absolute UVC pan/tilt control", device.Name)
	}
	if zoomChanged && (!capabilities.ZoomAbsolute || status.Zoom == nil) {
		return fmt.Errorf("camera %q does not support absolute UVC zoom control", device.Name)
	}

	target := ptzTarget{}
	if panTiltChanged {
		panValue := status.Pan.Cur
		tiltValue := status.Tilt.Cur
		if panChanged {
			panValue = uvc.DegreesToArcsec(pan)
			if relative {
				panValue = addInt32(status.Pan.Cur, panValue)
			}
		}
		if tiltChanged {
			tiltValue = uvc.DegreesToArcsec(tilt)
			if relative {
				tiltValue = addInt32(status.Tilt.Cur, tiltValue)
			}
		}
		appliedPan, appliedTilt, setErr := controller.SetPanTilt(panValue, tiltValue)
		if setErr != nil {
			return fmt.Errorf("set pan/tilt for camera %q: %w", device.Name, setErr)
		}
		target.pan, target.tilt = appliedPan, appliedTilt
		target.checkPan, target.checkTilt = true, true
	}

	if zoomChanged {
		zoomPercent := zoom
		if relative {
			zoomPercent += status.Zoom.Range.PercentOf(status.Zoom.Cur)
		}
		appliedZoom, setErr := controller.SetZoom(status.Zoom.Range.FromPercent(zoomPercent))
		if setErr != nil {
			return fmt.Errorf("set zoom for camera %q: %w", device.Name, setErr)
		}
		target.zoom, target.checkZoom = appliedZoom, true
	}

	if err := controller.Close(); err != nil {
		return fmt.Errorf("close PTZ control connection for camera %q before verification: %w", device.Name, err)
	}
	controller = nil
	status, err = settlePTZMotion(cmd.Context(), ptzOpenController, device, target, motion)
	if err != nil {
		return err
	}
	return writePTZStatus(cmd.OutOrStdout(), options.jsonOutput, makePTZStatusOutput(device, capabilities, status))
}

func settlePTZMotion(ctx context.Context, openController func(string) (ptzController, error), device localDevice, target ptzTarget, options *ptzMotionOptions) (uvc.Status, error) {
	controller, err := openController(device.ID)
	if err != nil {
		return uvc.Status{}, fmt.Errorf("open fresh PTZ control connection for camera %q: %w", device.Name, err)
	}
	defer func() { _ = controller.Close() }()

	deadline := ptzNow().Add(options.timeout)
	ptzSleep(options.settle)

	var previous ptzTarget
	havePrevious := false
	for {
		if err := ctx.Err(); err != nil {
			return uvc.Status{}, err
		}

		observed, err := controller.Status()
		if err != nil {
			return uvc.Status{}, fmt.Errorf("read applied PTZ status for camera %q: %w", device.Name, err)
		}
		current := ptzReading(observed)
		if havePrevious && current == previous {
			if err := verifyPTZTarget(device.Name, observed, target); err != nil {
				return observed, err
			}
			return observed, nil
		}
		previous, havePrevious = current, true

		remaining := deadline.Sub(ptzNow())
		if remaining <= 0 {
			// Reaching the target is the contract; a readback that keeps
			// jittering within tolerance (AI framing nudging the gimbal) still
			// satisfies the request, so only a missed target is an error.
			if err := verifyPTZTarget(device.Name, observed, target); err != nil {
				return observed, fmt.Errorf("PTZ position did not settle within %s: %w", options.timeout, err)
			}
			return observed, nil
		}
		ptzSleep(min(ptzPollInterval, remaining))
	}
}

func ptzReading(status uvc.Status) ptzTarget {
	reading := ptzTarget{}
	if status.Pan != nil {
		reading.pan, reading.checkPan = status.Pan.Cur, true
	}
	if status.Tilt != nil {
		reading.tilt, reading.checkTilt = status.Tilt.Cur, true
	}
	if status.Zoom != nil {
		reading.zoom, reading.checkZoom = status.Zoom.Cur, true
	}
	return reading
}

func verifyPTZTarget(camera string, observed uvc.Status, target ptzTarget) error {
	var differences []string
	if target.checkPan {
		differences = appendPTZAxisDifference(differences, "pan", observed.Pan, target.pan, false)
	}
	if target.checkTilt {
		differences = appendPTZAxisDifference(differences, "tilt", observed.Tilt, target.tilt, false)
	}
	if target.checkZoom {
		differences = appendPTZAxisDifference(differences, "zoom", observed.Zoom, target.zoom, true)
	}
	if len(differences) == 0 {
		return nil
	}
	return fmt.Errorf("camera %q did not reach its requested PTZ position: %s; the camera may be ignoring UVC because no video stream reached it, or on-camera AI framing/tracking may be overriding UVC positioning; ensure the camera is streaming and disable AI framing/tracking, then retry", camera, strings.Join(differences, "; "))
}

func appendPTZAxisDifference(differences []string, axis string, observed *uvc.AxisStatus, requested int32, zoom bool) []string {
	if observed == nil {
		return append(differences, fmt.Sprintf("%s observed unavailable, requested raw %d", axis, requested))
	}
	difference := int64(observed.Cur) - int64(requested)
	if difference < 0 {
		difference = -difference
	}
	if difference <= max(int64(observed.Range.Res), 0) {
		return differences
	}
	if zoom {
		return append(differences, fmt.Sprintf("%s observed %.2f%% (%d), requested %.2f%% (%d)", axis, observed.Range.PercentOf(observed.Cur), observed.Cur, observed.Range.PercentOf(requested), requested))
	}
	return append(differences, fmt.Sprintf("%s observed %.2f° (%d), requested %.2f° (%d)", axis, uvc.ArcsecToDegrees(observed.Cur), observed.Cur, uvc.ArcsecToDegrees(requested), requested))
}

func openPTZ(ctx context.Context, selector string) (localDevice, io.Closer, ptzController, error) {
	device, err := ptzResolveDevice(selector)
	if err != nil {
		name := selector
		if name == "" {
			name = "default camera"
		}
		return localDevice{}, nil, nil, fmt.Errorf("resolve PTZ camera %q: %w", name, err)
	}
	session, err := ptzOpenSession(ctx, device.ID)
	if err != nil {
		return localDevice{}, nil, nil, fmt.Errorf("open video stream for PTZ camera %q: %w", device.Name, err)
	}
	controller, err := ptzOpenController(device.ID)
	if err != nil {
		_ = session.Close()
		return localDevice{}, nil, nil, fmt.Errorf("camera %q does not support USB UVC PTZ control: %w", device.Name, err)
	}
	if !controller.Capabilities().Any() {
		_ = controller.Close()
		_ = session.Close()
		return localDevice{}, nil, nil, fmt.Errorf("camera %q does not advertise any UVC PTZ controls", device.Name)
	}
	return device, session, controller, nil
}

func resolveNativePTZDevice(selector string) (localDevice, error) {
	devices, err := nativeEnumerateLocalDevices()
	if err != nil {
		return localDevice{}, fmt.Errorf("enumerate native cameras: %w", err)
	}
	if selector != "" {
		return resolveNativeDevice(devices, selector)
	}
	if device, ok := defaultNativeDevice(devices); ok {
		return device, nil
	}
	return localDevice{}, fmt.Errorf("no default native camera is available")
}

func makePTZStatusOutput(device localDevice, capabilities uvc.Capabilities, status uvc.Status) ptzStatusOutput {
	output := ptzStatusOutput{Device: device, Capabilities: capabilities}
	if status.Pan != nil {
		output.Pan = &ptzAngleOutput{
			Degrees: uvc.ArcsecToDegrees(status.Pan.Cur),
			Raw:     status.Pan.Cur,
			Range:   status.Pan.Range,
		}
	}
	if status.Tilt != nil {
		output.Tilt = &ptzAngleOutput{
			Degrees: uvc.ArcsecToDegrees(status.Tilt.Cur),
			Raw:     status.Tilt.Cur,
			Range:   status.Tilt.Range,
		}
	}
	if status.Zoom != nil {
		output.Zoom = &ptzZoomOutput{
			Percent: status.Zoom.Range.PercentOf(status.Zoom.Cur),
			Raw:     status.Zoom.Cur,
			Range:   status.Zoom.Range,
		}
	}
	return output
}

func writePTZStatus(output io.Writer, jsonOutput bool, status ptzStatusOutput) error {
	if jsonOutput {
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(status)
	}

	if _, err := fmt.Fprintf(output, "Device: %s (%s)\n", status.Device.Name, status.Device.ID); err != nil {
		return fmt.Errorf("write PTZ device: %w", err)
	}
	writer := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(writer, "CONTROL\tABSOLUTE\tRELATIVE\tVALUE\tRAW\tMIN\tMAX\tRES\tDEFAULT")
	writePTZRow(writer, "PAN", status.Capabilities.PanTiltAbsolute, status.Capabilities.PanTiltRelative, angleValue(status.Pan), angleRaw(status.Pan), angleRange(status.Pan))
	writePTZRow(writer, "TILT", status.Capabilities.PanTiltAbsolute, status.Capabilities.PanTiltRelative, angleValue(status.Tilt), angleRaw(status.Tilt), angleRange(status.Tilt))
	writePTZRow(writer, "ZOOM", status.Capabilities.ZoomAbsolute, status.Capabilities.ZoomRelative, zoomValue(status.Zoom), zoomRaw(status.Zoom), zoomRange(status.Zoom))
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("write PTZ table: %w", err)
	}
	return nil
}

func writePTZRow(writer io.Writer, control string, absolute, relative bool, value, raw string, valueRange *uvc.Range) {
	minimum, maximum, resolution, defaultValue := "-", "-", "-", "-"
	if valueRange != nil {
		minimum = fmt.Sprint(valueRange.Min)
		maximum = fmt.Sprint(valueRange.Max)
		resolution = fmt.Sprint(valueRange.Res)
		defaultValue = fmt.Sprint(valueRange.Def)
	}
	_, _ = fmt.Fprintf(writer, "%s\t%t\t%t\t%s\t%s\t%s\t%s\t%s\t%s\n", control, absolute, relative, value, raw, minimum, maximum, resolution, defaultValue)
}

func angleValue(status *ptzAngleOutput) string {
	if status == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f°", status.Degrees)
}

func angleRaw(status *ptzAngleOutput) string {
	if status == nil {
		return "-"
	}
	return fmt.Sprint(status.Raw)
}

func angleRange(status *ptzAngleOutput) *uvc.Range {
	if status == nil {
		return nil
	}
	return &status.Range
}

func zoomValue(status *ptzZoomOutput) string {
	if status == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f%%", status.Percent)
}

func zoomRaw(status *ptzZoomOutput) string {
	if status == nil {
		return "-"
	}
	return fmt.Sprint(status.Raw)
}

func zoomRange(status *ptzZoomOutput) *uvc.Range {
	if status == nil {
		return nil
	}
	return &status.Range
}

func addInt32(left, right int32) int32 {
	sum := int64(left) + int64(right)
	if sum > math.MaxInt32 {
		return math.MaxInt32
	}
	if sum < math.MinInt32 {
		return math.MinInt32
	}
	return int32(sum)
}
