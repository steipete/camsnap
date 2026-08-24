package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

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
	statusCalls  int
	statusValues []uvc.Status
	ignoreWrites bool
	closed       bool
	events       *[]string
}

type fakePTZSession struct {
	closed bool
	events *[]string
	sleeps []time.Duration
}

func (s *fakePTZSession) Close() error {
	s.closed = true
	if s.events != nil {
		*s.events = append(*s.events, "session-close")
	}
	return nil
}

func (f *fakePTZController) record(event string) {
	if f.events != nil {
		*f.events = append(*f.events, event)
	}
}

func (f *fakePTZController) Capabilities() uvc.Capabilities {
	f.record("capabilities")
	return f.capabilities
}

func (f *fakePTZController) Status() (uvc.Status, error) {
	f.record("status")
	f.statusCalls++
	if len(f.statusValues) > 0 {
		status := f.statusValues[0]
		f.statusValues = f.statusValues[1:]
		return clonePTZStatus(status), nil
	}
	return clonePTZStatus(f.status), nil
}

func (f *fakePTZController) SetPanTilt(pan, tilt int32) (int32, int32, error) {
	f.record("set-pan-tilt")
	f.setPan = f.status.Pan.Range.Clamp(pan)
	f.setTilt = f.status.Tilt.Range.Clamp(tilt)
	if !f.ignoreWrites {
		f.status.Pan.Cur = f.setPan
		f.status.Tilt.Cur = f.setTilt
	}
	f.panTiltSets++
	return f.setPan, f.setTilt, nil
}
func (f *fakePTZController) SetZoom(zoom int32) (int32, error) {
	f.record("set-zoom")
	f.setZoom = f.status.Zoom.Range.Clamp(zoom)
	if !f.ignoreWrites {
		f.status.Zoom.Cur = f.setZoom
	}
	f.zoomSets++
	return f.setZoom, nil
}
func (f *fakePTZController) Home() (uvc.Status, error) {
	f.record("home")
	if !f.ignoreWrites {
		if f.status.Pan != nil {
			f.status.Pan.Cur = f.status.Pan.Range.Def
			f.status.Tilt.Cur = f.status.Tilt.Range.Def
		}
		if f.status.Zoom != nil {
			f.status.Zoom.Cur = f.status.Zoom.Range.Def
		}
	}
	return clonePTZStatus(f.status), nil
}
func (f *fakePTZController) Close() error {
	f.record("controller-close")
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

func TestPTZMotionReportsObservedSettledPosition(t *testing.T) {
	controller := newFakePTZController()
	initial := clonePTZStatus(controller.status)
	observed := clonePTZStatus(controller.status)
	observed.Pan.Cur = 45900
	controller.statusValues = []uvc.Status{initial, observed, observed}
	restore := stubPTZBackend(t, controller)
	defer restore()

	root := NewRootCommand("test")
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"ptz", "goto", "--pan", "12.5", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	var status ptzStatusOutput
	if err := json.Unmarshal(output.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Pan == nil || status.Pan.Raw != 45900 {
		t.Fatalf("observed pan = %#v, want raw 45900", status.Pan)
	}
}

func TestPTZMotionRejectsSettledMismatch(t *testing.T) {
	controller := newFakePTZController()
	controller.ignoreWrites = true
	restore := stubPTZBackend(t, controller)
	defer restore()

	root := NewRootCommand("test")
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"ptz", "goto", "--pan", "12.5"})
	err := root.Execute()
	if err == nil {
		t.Fatal("Execute succeeded despite the camera ignoring its requested position")
	}
	for _, want := range []string{
		"pan observed 1.00° (3600), requested 12.50° (45000)",
		"no video stream reached it",
		"AI framing/tracking",
		"then retry",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q: %v", want, err)
		}
	}
	if output.Len() != 0 {
		t.Fatalf("mismatched motion printed a success table: %s", output.String())
	}
	if !controller.closed {
		t.Fatal("controller was not closed after the verification failure")
	}
}

func TestPTZMotionRejectsSetpointEchoFromWriteConnection(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		requested string
	}{
		{name: "goto", args: []string{"ptz", "goto", "--pan", "140"}, requested: "requested 140.00° (504000)"},
		{name: "move", args: []string{"ptz", "move", "--pan", "240"}, requested: "requested 140.00° (504000)"},
		{name: "home", args: []string{"ptz", "home"}, requested: "requested 0.00° (0)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var events []string
			writer := newFakePTZController()
			writer.status.Pan.Range.Min = -648000
			writer.status.Pan.Range.Max = 648000
			writer.status.Pan.Cur = -360000
			writer.events = &events

			verifier := newFakePTZController()
			verifier.status = clonePTZStatus(writer.status)
			verifier.events = &events
			session, restore := stubPTZBackendWithSession(t, writer, verifier)
			defer restore()
			session.events = &events

			root := NewRootCommand("test")
			root.SetOut(&bytes.Buffer{})
			root.SetArgs(test.args)
			err := root.Execute()
			if err == nil {
				t.Fatal("Execute accepted the write connection's echoed setpoint despite an unmoved camera")
			}
			for _, want := range []string{"did not reach its requested PTZ position", "pan observed -100.00° (-360000)", test.requested} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error missing %q: %v", want, err)
				}
			}
			if writer.status.Pan.Cur == verifier.status.Pan.Cur {
				t.Fatal("fake did not model a write-only setpoint echo")
			}
			if verifier.statusCalls < 2 {
				t.Fatalf("fresh connection status calls = %d, want consecutive verification samples", verifier.statusCalls)
			}
			if !writer.closed || !verifier.closed || !session.closed {
				t.Fatalf("resource closure: writer=%t verifier=%t session=%t", writer.closed, verifier.closed, session.closed)
			}
			if len(events) < 6 || events[0] != "session-open" || events[len(events)-1] != "session-close" {
				t.Fatalf("capture session did not outlive both controller connections: %v", events)
			}
			opens, writerClosed := 0, false
			for _, event := range events {
				switch event {
				case "controller-close":
					writerClosed = true
				case "controller-open":
					opens++
					if opens == 2 && !writerClosed {
						t.Fatalf("verification connection opened before the writer closed: %v", events)
					}
				}
			}
			if opens != 2 {
				t.Fatalf("controller opens = %d, want write and fresh verification connections: %v", opens, events)
			}
		})
	}
}

func TestPTZMotionAcceptsCommittedPositionFromFreshConnection(t *testing.T) {
	writer := newFakePTZController()
	writer.status.Pan.Range.Min = -648000
	writer.status.Pan.Cur = 0
	verifier := newFakePTZController()
	verifier.status = clonePTZStatus(writer.status)
	verifier.status.Pan.Cur = -360000
	session, restore := stubPTZBackendWithSession(t, writer, verifier)
	defer restore()

	var output bytes.Buffer
	root := NewRootCommand("test")
	root.SetOut(&output)
	root.SetArgs([]string{"ptz", "goto", "--pan", "-100"})
	if err := root.Execute(); err != nil {
		t.Fatalf("committed movement failed fresh-connection verification: %v", err)
	}
	if writer.statusCalls != 1 || verifier.statusCalls < 2 {
		t.Fatalf("status calls: writer=%d verifier=%d", writer.statusCalls, verifier.statusCalls)
	}
	if !strings.Contains(output.String(), "-360000") {
		t.Fatalf("output omitted the committed position: %s", output.String())
	}
	if !writer.closed || !verifier.closed || !session.closed {
		t.Fatalf("resource closure: writer=%t verifier=%t session=%t", writer.closed, verifier.closed, session.closed)
	}
}

func TestPTZMotionWaitsForConsecutiveStableSamples(t *testing.T) {
	controller := newFakePTZController()
	initial := clonePTZStatus(controller.status)
	moving := clonePTZStatus(controller.status)
	moving.Pan.Cur = 18000
	settled := clonePTZStatus(controller.status)
	settled.Pan.Cur = 45000
	controller.statusValues = []uvc.Status{initial, moving, settled, settled}
	session, restore := stubPTZBackendWithSession(t, controller)
	defer restore()

	root := NewRootCommand("test")
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"ptz", "goto", "--pan", "12.5", "--settle", "2ms", "--timeout", "500ms"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if controller.statusCalls != 4 {
		t.Fatalf("status calls = %d, want initial read plus three verification samples", controller.statusCalls)
	}
	if len(session.sleeps) != 3 || session.sleeps[0] != 2*time.Millisecond {
		t.Fatalf("verification waits = %v, want settle then two polling intervals", session.sleeps)
	}
}

func TestPTZMotionTimesOutWhilePositionIsStillChanging(t *testing.T) {
	controller := newFakePTZController()
	initial := clonePTZStatus(controller.status)
	first := clonePTZStatus(controller.status)
	first.Pan.Cur = 18000
	second := clonePTZStatus(controller.status)
	second.Pan.Cur = 27000
	controller.statusValues = []uvc.Status{initial, first, second}
	session, restore := stubPTZBackendWithSession(t, controller)
	defer restore()

	root := NewRootCommand("test")
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"ptz", "goto", "--pan", "12.5", "--settle", "0s", "--timeout", "2ms"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "did not settle within 2ms") {
		t.Fatalf("timeout error = %v", err)
	}
	if !strings.Contains(err.Error(), "pan observed 7.50° (27000), requested 12.50° (45000)") {
		t.Fatalf("timeout omitted the last observed position: %v", err)
	}
	if !session.closed || !controller.closed {
		t.Fatal("capture session or controller remained open after the verification timeout")
	}
}

func TestPTZOperationsKeepSessionOpenAroundUVCWork(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "status", args: []string{"ptz", "status"}},
		{name: "goto", args: []string{"ptz", "goto", "--pan", "12.5"}},
		{name: "move", args: []string{"ptz", "move", "--zoom", "25"}},
		{name: "home", args: []string{"ptz", "home"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var events []string
			controller := newFakePTZController()
			controller.events = &events
			session, restore := stubPTZBackendWithSession(t, controller)
			defer restore()
			session.events = &events

			root := NewRootCommand("test")
			root.SetOut(&bytes.Buffer{})
			root.SetArgs(test.args)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if len(events) < 4 || events[0] != "session-open" || events[1] != "controller-open" {
				t.Fatalf("session did not open before UVC work: %v", events)
			}
			if events[len(events)-2] != "controller-close" || events[len(events)-1] != "session-close" {
				t.Fatalf("session did not close after UVC work: %v", events)
			}
			if !session.closed {
				t.Fatal("capture session was not closed")
			}
		})
	}
}

func TestPTZMotionRejectsInvalidTiming(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"ptz", "goto", "--pan", "1", "--settle", "-1ms"}, want: "--settle must not be negative"},
		{args: []string{"ptz", "move", "--pan", "1", "--timeout", "0s"}, want: "--timeout must be greater than zero"},
		{args: []string{"ptz", "home", "--settle", "5s", "--timeout", "5s"}, want: "--settle must be less than --timeout"},
	}
	for _, test := range tests {
		root := NewRootCommand("test")
		root.SetArgs(test.args)
		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("Execute(%v) error = %v, want %q", test.args, err, test.want)
		}
	}
}

func TestPTZSessionFailurePreservesCameraPermissionRemediation(t *testing.T) {
	controller := newFakePTZController()
	restore := stubPTZBackend(t, controller)
	defer restore()
	ptzOpenSession = func(context.Context, string) (io.Closer, error) {
		return nil, errors.New("camera access is denied\n" + cameraPermissionRemediation)
	}

	root := NewRootCommand("test")
	root.SetArgs([]string{"ptz", "status"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), cameraPermissionRemediation) {
		t.Fatalf("permission error = %v", err)
	}
	if controller.statusCalls != 0 {
		t.Fatal("UVC status was queried after the capture session failed")
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
	oldSession := ptzOpenSession
	oldOpen := ptzOpenController
	session := &fakePTZSession{}
	ptzResolveDevice = func(string) (localDevice, error) {
		return localDevice{ID: "continuity-id", Name: "Continuity Camera"}, nil
	}
	ptzOpenSession = func(context.Context, string) (io.Closer, error) {
		return session, nil
	}
	ptzOpenController = func(string) (ptzController, error) {
		return &fakePTZController{}, nil
	}
	t.Cleanup(func() {
		ptzResolveDevice = oldResolve
		ptzOpenSession = oldSession
		ptzOpenController = oldOpen
	})

	_, _, _, err := openPTZ(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), `camera "Continuity Camera" does not advertise any UVC PTZ controls`) {
		t.Fatalf("openPTZ error = %v", err)
	}
	if !session.closed {
		t.Fatal("capture session was not closed for an unsupported camera")
	}
}

func TestPTZMotionAcceptsOnTargetJitterAtTimeout(t *testing.T) {
	controller := newFakePTZController()
	initial := clonePTZStatus(controller.status)
	onTarget := clonePTZStatus(controller.status)
	onTarget.Pan.Cur = 45000
	jittered := clonePTZStatus(controller.status)
	jittered.Pan.Cur = 44100
	controller.statusValues = []uvc.Status{initial, onTarget, jittered}
	_, restore := stubPTZBackendWithSession(t, controller)
	defer restore()

	output := &bytes.Buffer{}
	root := NewRootCommand("test")
	root.SetOut(output)
	root.SetArgs([]string{"ptz", "goto", "--pan", "12.5", "--settle", "0s", "--timeout", "2ms"})
	if err := root.Execute(); err != nil {
		t.Fatalf("readback jitter within the control resolution must not fail: %v", err)
	}
	if !strings.Contains(output.String(), "44100") {
		t.Fatalf("output must report the observed position, got: %s", output.String())
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

func clonePTZStatus(status uvc.Status) uvc.Status {
	result := uvc.Status{}
	if status.Pan != nil {
		pan := *status.Pan
		result.Pan = &pan
	}
	if status.Tilt != nil {
		tilt := *status.Tilt
		result.Tilt = &tilt
	}
	if status.Zoom != nil {
		zoom := *status.Zoom
		result.Zoom = &zoom
	}
	return result
}

func stubPTZBackend(t *testing.T, controller ptzController) func() {
	t.Helper()
	_, restore := stubPTZBackendWithSession(t, controller)
	return restore
}

func stubPTZBackendWithSession(t *testing.T, controller ptzController, verification ...ptzController) (*fakePTZSession, func()) {
	t.Helper()
	oldResolve := ptzResolveDevice
	oldSession := ptzOpenSession
	oldOpen := ptzOpenController
	oldNow := ptzNow
	oldSleep := ptzSleep
	session := &fakePTZSession{}
	now := time.Unix(0, 0)
	opens := 0
	ptzResolveDevice = func(selector string) (localDevice, error) {
		if selector != "" && selector != "Link" {
			t.Fatalf("device selector = %q", selector)
		}
		return localDevice{ID: "camera-id", Index: "0", Name: "Insta360 Link", IsDefault: true}, nil
	}
	ptzOpenSession = func(_ context.Context, uniqueID string) (io.Closer, error) {
		if uniqueID != "camera-id" {
			t.Fatalf("session unique ID = %q", uniqueID)
		}
		if session.events != nil {
			*session.events = append(*session.events, "session-open")
		}
		return session, nil
	}
	ptzOpenController = func(uniqueID string) (ptzController, error) {
		if uniqueID != "camera-id" {
			t.Fatalf("unique ID = %q", uniqueID)
		}
		if session.events != nil {
			*session.events = append(*session.events, "controller-open")
		}
		opens++
		if opens > 1 && len(verification) > 0 {
			return verification[0], nil
		}
		return controller, nil
	}
	ptzNow = func() time.Time { return now }
	ptzSleep = func(duration time.Duration) {
		session.sleeps = append(session.sleeps, duration)
		now = now.Add(duration)
	}
	return session, func() {
		ptzResolveDevice = oldResolve
		ptzOpenSession = oldSession
		ptzOpenController = oldOpen
		ptzNow = oldNow
		ptzSleep = oldSleep
	}
}
