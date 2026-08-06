package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"text/tabwriter"

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
	ptzOpenController = openNativePTZController
)

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
			device, controller, err := openPTZ(options.device)
			if err != nil {
				return err
			}
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
	cmd := &cobra.Command{
		Use:   "goto",
		Short: "Move to absolute pan, tilt, or zoom positions",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			return validatePTZMotionFlags(cmd, pan, tilt, zoom)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPTZMotion(cmd, options, false, pan, tilt, zoom)
		},
	}
	cmd.Flags().Float64Var(&pan, "pan", 0, "Absolute pan angle in degrees")
	cmd.Flags().Float64Var(&tilt, "tilt", 0, "Absolute tilt angle in degrees")
	cmd.Flags().Float64Var(&zoom, "zoom", 0, "Absolute zoom position as a percentage")
	return cmd
}

func newPTZMoveCmd(options *ptzOptions) *cobra.Command {
	var pan, tilt, zoom float64
	cmd := &cobra.Command{
		Use:   "move",
		Short: "Move by relative pan, tilt, or zoom deltas",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			return validatePTZMotionFlags(cmd, pan, tilt, zoom)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPTZMotion(cmd, options, true, pan, tilt, zoom)
		},
	}
	cmd.Flags().Float64Var(&pan, "pan", 0, "Relative pan delta in degrees")
	cmd.Flags().Float64Var(&tilt, "tilt", 0, "Relative tilt delta in degrees")
	cmd.Flags().Float64Var(&zoom, "zoom", 0, "Relative zoom delta in percentage points")
	return cmd
}

func newPTZHomeCmd(options *ptzOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "home",
		Short: "Reset pan, tilt, and zoom to their defaults",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			device, controller, err := openPTZ(options.device)
			if err != nil {
				return err
			}
			defer func() { _ = controller.Close() }()

			status, err := controller.Home()
			if err != nil {
				return fmt.Errorf("home camera %q: %w", device.Name, err)
			}
			return writePTZStatus(cmd.OutOrStdout(), options.jsonOutput, makePTZStatusOutput(device, controller.Capabilities(), status))
		},
	}
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

func runPTZMotion(cmd *cobra.Command, options *ptzOptions, relative bool, pan, tilt, zoom float64) error {
	device, controller, err := openPTZ(options.device)
	if err != nil {
		return err
	}
	defer func() { _ = controller.Close() }()

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
		if _, _, err := controller.SetPanTilt(panValue, tiltValue); err != nil {
			return fmt.Errorf("set pan/tilt for camera %q: %w", device.Name, err)
		}
	}

	if zoomChanged {
		zoomPercent := zoom
		if relative {
			zoomPercent += status.Zoom.Range.PercentOf(status.Zoom.Cur)
		}
		if _, err := controller.SetZoom(status.Zoom.Range.FromPercent(zoomPercent)); err != nil {
			return fmt.Errorf("set zoom for camera %q: %w", device.Name, err)
		}
	}

	status, err = controller.Status()
	if err != nil {
		return fmt.Errorf("read applied PTZ status for camera %q: %w", device.Name, err)
	}
	return writePTZStatus(cmd.OutOrStdout(), options.jsonOutput, makePTZStatusOutput(device, capabilities, status))
}

func openPTZ(selector string) (localDevice, ptzController, error) {
	device, err := ptzResolveDevice(selector)
	if err != nil {
		name := selector
		if name == "" {
			name = "default camera"
		}
		return localDevice{}, nil, fmt.Errorf("resolve PTZ camera %q: %w", name, err)
	}
	controller, err := ptzOpenController(device.ID)
	if err != nil {
		return localDevice{}, nil, fmt.Errorf("camera %q does not support USB UVC PTZ control: %w", device.Name, err)
	}
	if !controller.Capabilities().Any() {
		_ = controller.Close()
		return localDevice{}, nil, fmt.Errorf("camera %q does not advertise any UVC PTZ controls", device.Name)
	}
	return device, controller, nil
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
