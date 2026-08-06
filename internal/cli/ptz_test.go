package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/steipete/camsnap/internal/uvc"
)

type fakePTZController struct {
	capabilities uvc.Capabilities
	status       uvc.Status
	setPan       int32
	setTilt      int32
	setZoom      int32
	panTiltSets  int
	zoomSets     int
	closed       bool
}

func (f *fakePTZController) Capabilities() uvc.Capabilities { return f.capabilities }
func (f *fakePTZController) Status() (uvc.Status, error)    { return f.status, nil }
func (f *fakePTZController) SetPanTilt(pan, tilt int32) (int32, int32, error) {
	f.setPan = f.status.Pan.Range.Clamp(pan)
	f.setTilt = f.status.Tilt.Range.Clamp(tilt)
	f.status.Pan.Cur = f.setPan
	f.status.Tilt.Cur = f.setTilt
	f.panTiltSets++
	return f.setPan, f.setTilt, nil
}
func (f *fakePTZController) SetZoom(zoom int32) (int32, error) {
	f.setZoom = f.status.Zoom.Range.Clamp(zoom)
	f.status.Zoom.Cur = f.setZoom
	f.zoomSets++
	return f.setZoom, nil
}
func (f *fakePTZController) Home() (uvc.Status, error) {
	if f.status.Pan != nil {
		f.status.Pan.Cur = f.status.Pan.Range.Def
		f.status.Tilt.Cur = f.status.Tilt.Range.Def
	}
	if f.status.Zoom != nil {
		f.status.Zoom.Cur = f.status.Zoom.Range.Def
	}
	return f.status, nil
}
func (f *fakePTZController) Close() error {
	f.closed = true
	return nil
}

func TestPTZMotionRequiresAnAxis(t *testing.T) {
	tests := []string{"goto", "move"}
	for _, subcommand := range tests {
		t.Run(subcommand, func(t *testing.T) {
			root := NewRootCommand("test")
			root.SetArgs([]string{"ptz", subcommand})
			err := root.Execute()
			if err == nil || !strings.Contains(err.Error(), "at least one of --pan, --tilt, or --zoom is required") {
				t.Fatalf("Execute error = %v", err)
			}
		})
	}
}

func TestPTZMotionRejectsNonFiniteValues(t *testing.T) {
	for _, value := range []string{"NaN", "+Inf", "-Inf"} {
		root := NewRootCommand("test")
		root.SetArgs([]string{"ptz", "goto", "--pan", value})
		err := root.Execute()
		if err == nil || !strings.Contains(err.Error(), "--pan must be a finite number") {
			t.Fatalf("Execute --pan %s error = %v", value, err)
		}
	}
}

func TestPTZCommandsRejectArguments(t *testing.T) {
	tests := [][]string{
		{"ptz", "status", "extra"},
		{"ptz", "goto", "extra", "--pan", "1"},
		{"ptz", "move", "extra", "--zoom", "1"},
		{"ptz", "home", "extra"},
	}
	for _, args := range tests {
		root := NewRootCommand("test")
		root.SetArgs(args)
		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "unknown command") && !strings.Contains(err.Error(), "accepts 0 arg") {
			t.Errorf("Execute(%v) error = %v", args, err)
		}
	}
}

func TestPTZGotoOnlyTouchesSpecifiedAxis(t *testing.T) {
	controller := newFakePTZController()
	restore := stubPTZBackend(t, controller)
	defer restore()

	root := NewRootCommand("test")
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"ptz", "goto", "--device", "Link", "--pan", "12.5", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if controller.setPan != 45000 || controller.setTilt != -7200 {
		t.Fatalf("SetPanTilt = %d, %d, want 45000, -7200", controller.setPan, controller.setTilt)
	}
	if controller.panTiltSets != 1 || controller.zoomSets != 0 {
		t.Fatalf("set counts = pan/tilt %d, zoom %d", controller.panTiltSets, controller.zoomSets)
	}
	if !controller.closed {
		t.Fatal("controller was not closed")
	}

	var status ptzStatusOutput
	if err := json.Unmarshal(output.Bytes(), &status); err != nil {
		t.Fatalf("decode output: %v\n%s", err, output.String())
	}
	if status.Pan == nil || status.Pan.Degrees != 12.5 {
		t.Fatalf("pan output = %#v", status.Pan)
	}
}

func TestPTZMoveZoomUsesPercentagePointDelta(t *testing.T) {
	controller := newFakePTZController()
	restore := stubPTZBackend(t, controller)
	defer restore()

	root := NewRootCommand("test")
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"ptz", "move", "--zoom", "25"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if controller.setZoom != 400 {
		t.Fatalf("SetZoom = %d, want 400", controller.setZoom)
	}
	if controller.panTiltSets != 0 || controller.zoomSets != 1 {
		t.Fatalf("set counts = pan/tilt %d, zoom %d", controller.panTiltSets, controller.zoomSets)
	}
}

func TestWritePTZStatusTable(t *testing.T) {
	controller := newFakePTZController()
	status := makePTZStatusOutput(
		localDevice{ID: "camera-id", Name: "Insta360 Link", IsDefault: true},
		controller.capabilities,
		controller.status,
	)
	var output bytes.Buffer
	if err := writePTZStatus(&output, false, status); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Device: Insta360 Link (camera-id)",
		"CONTROL  ABSOLUTE  RELATIVE  VALUE",
		"PAN      true      true      1.00°",
		"TILT     true      true      -2.00°",
		"ZOOM     true      true      50.00%",
	} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("table missing %q:\n%s", want, output.String())
		}
	}
}

func TestOpenPTZNamesUnsupportedCamera(t *testing.T) {
	oldResolve := ptzResolveDevice
	oldOpen := ptzOpenController
	ptzResolveDevice = func(string) (localDevice, error) {
		return localDevice{ID: "continuity-id", Name: "Continuity Camera"}, nil
	}
	ptzOpenController = func(string) (ptzController, error) {
		return &fakePTZController{}, nil
	}
	t.Cleanup(func() {
		ptzResolveDevice = oldResolve
		ptzOpenController = oldOpen
	})

	_, _, err := openPTZ("")
	if err == nil || !strings.Contains(err.Error(), `camera "Continuity Camera" does not advertise any UVC PTZ controls`) {
		t.Fatalf("openPTZ error = %v", err)
	}
}

func newFakePTZController() *fakePTZController {
	panRange := uvc.Range{Min: -324000, Max: 324000, Res: 900, Def: 0}
	tiltRange := uvc.Range{Min: -162000, Max: 162000, Res: 900, Def: 0}
	zoomRange := uvc.Range{Min: 100, Max: 500, Res: 10, Def: 100}
	return &fakePTZController{
		capabilities: uvc.Capabilities{
			PanTiltAbsolute: true,
			PanTiltRelative: true,
			ZoomAbsolute:    true,
			ZoomRelative:    true,
		},
		status: uvc.Status{
			Pan:  &uvc.AxisStatus{Cur: 3600, Range: panRange},
			Tilt: &uvc.AxisStatus{Cur: -7200, Range: tiltRange},
			Zoom: &uvc.AxisStatus{Cur: 300, Range: zoomRange},
		},
	}
}

func stubPTZBackend(t *testing.T, controller ptzController) func() {
	t.Helper()
	oldResolve := ptzResolveDevice
	oldOpen := ptzOpenController
	ptzResolveDevice = func(selector string) (localDevice, error) {
		if selector != "" && selector != "Link" {
			t.Fatalf("device selector = %q", selector)
		}
		return localDevice{ID: "camera-id", Index: "0", Name: "Insta360 Link", IsDefault: true}, nil
	}
	ptzOpenController = func(uniqueID string) (ptzController, error) {
		if uniqueID != "camera-id" {
			t.Fatalf("unique ID = %q", uniqueID)
		}
		return controller, nil
	}
	return func() {
		ptzResolveDevice = oldResolve
		ptzOpenController = oldOpen
	}
}
