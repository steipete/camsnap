package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/steipete/camsnap/internal/config"
)

func (s *fakePTZSession) CaptureFrame(warmup time.Duration, path string) error {
	if s.closed {
		return fmt.Errorf("capture session is closed")
	}
	s.captures = append(s.captures, path)
	s.captureWarmups = append(s.captureWarmups, warmup)
	if s.events != nil {
		*s.events = append(*s.events, "capture:"+filepath.Base(path))
	}
	return os.WriteFile(path, []byte("fake JPEG"), 0o600)
}

func TestSweepKeepsOneSessionOpenAndCapturesVerifiedFrames(t *testing.T) {
	var events []string
	controllers := sweepControllers([]int32{-36000, 0, 36000}, nil, &events)
	for index := 1; index < len(controllers); index += 2 {
		controllers[index].status.Tilt.Cur = 18000
	}
	controllers[3].status.Pan.Cur = 900
	session, restore := stubSweepBackend(t, controllers, &events)
	defer restore()

	outDir := filepath.Join(t.TempDir(), "frames")
	var stdout, stderr bytes.Buffer
	root := NewRootCommand("test")
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"sweep", "--device", "Link", "--from", "-10", "--to", "10", "--steps", "3",
		"--tilt", "5", "--warmup", "250ms", "--out-dir", outDir, "--json",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	if !session.closed {
		t.Fatal("capture session was not closed")
	}
	if got := countSweepEvents(events, "session-open"); got != 1 {
		t.Fatalf("capture session opens = %d, want 1: %v", got, events)
	}
	if len(events) == 0 || events[len(events)-1] != "session-close" {
		t.Fatalf("capture session did not outlive every step: %v", events)
	}
	if got, want := session.captureWarmups, []time.Duration{250 * time.Millisecond, 0, 0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("capture warmups = %v, want %v", got, want)
	}
	for index, raw := range []int32{-36000, 0, 36000} {
		writer := controllers[index*2]
		verifier := controllers[index*2+1]
		if writer.setPan != raw || writer.setTilt != 18000 {
			t.Errorf("step %d write = (%d, %d), want (%d, 18000)", index, writer.setPan, writer.setTilt, raw)
		}
		if !writer.closed || !verifier.closed || verifier.statusCalls < 2 {
			t.Errorf("step %d controller lifecycle: writer closed=%t verifier closed=%t verifier reads=%d", index, writer.closed, verifier.closed, verifier.statusCalls)
		}
	}
	assertSweepStepOrdering(t, events, 3)

	manifestData, err := os.ReadFile(filepath.Join(outDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout.Bytes(), manifestData) {
		t.Fatalf("JSON stdout differs from manifest file:\nstdout: %s\nmanifest: %s", stdout.String(), manifestData)
	}
	var manifest sweepManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Device.Name != "Insta360 Link" || manifest.FromDegrees != -10 || manifest.ToDegrees != 10 || manifest.TiltDegrees != 5 || manifest.RequestedSteps != 3 {
		t.Fatalf("manifest metadata = %#v", manifest)
	}
	if len(manifest.Steps) != 3 {
		t.Fatalf("manifest steps = %d, want 3", len(manifest.Steps))
	}
	for index, step := range manifest.Steps {
		wantPan := int32(-36000 + index*36000)
		wantObserved := wantPan
		if index == 1 {
			wantObserved = 900
		}
		if step.Index != index || step.Requested.Pan.Raw != wantPan || step.Observed.Pan.Raw != wantObserved {
			t.Errorf("step %d pan = requested %#v, observed %#v", index, step.Requested.Pan, step.Observed.Pan)
		}
		if step.Requested.Tilt.Raw != 18000 || step.Observed.Tilt.Raw != 18000 || !step.Verified {
			t.Errorf("step %d tilt/verification = %#v", index, step)
		}
		if filepath.Base(step.FramePath) != fmt.Sprintf("step-%03d-pan-%+.3f.jpg", index, step.Requested.Pan.Degrees) {
			t.Errorf("step %d frame path = %q", index, step.FramePath)
		}
		if _, err := os.Stat(step.FramePath); err != nil {
			t.Errorf("step %d frame is missing: %v", index, err)
		}
	}
}

func TestSweepVerificationFailureContinuesAndWritesManifest(t *testing.T) {
	controllers := sweepControllers([]int32{-36000, 0, 36000}, map[int]int32{1: 18000}, nil)
	session, restore := stubSweepBackend(t, controllers, nil)
	defer restore()

	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	root := NewRootCommand("test")
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"sweep", "--device", "Link", "--from", "-10", "--to", "10", "--steps", "3",
		"--out-dir", outDir, "--json",
	})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "sweep verification failed for steps 1") {
		t.Fatalf("sweep error = %v", err)
	}
	if !strings.Contains(stderr.String(), "Sweep step 1 verification failed") {
		t.Fatalf("verification failure missing from stderr: %s", stderr.String())
	}
	if len(session.captures) != 3 {
		t.Fatalf("captured frames = %d, want all 3", len(session.captures))
	}

	var manifest sweepManifest
	if err := json.Unmarshal(stdout.Bytes(), &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Steps) != 3 || !manifest.Steps[0].Verified || manifest.Steps[1].Verified || !manifest.Steps[2].Verified {
		t.Fatalf("verification results = %#v", manifest.Steps)
	}
	failed := manifest.Steps[1]
	if failed.Requested.Pan.Raw != 0 || failed.Observed.Pan.Raw != 18000 || failed.VerificationError == "" {
		t.Fatalf("failed step did not preserve the requested and observed positions: %#v", failed)
	}
	manifestData, err := os.ReadFile(filepath.Join(outDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(manifestData, stdout.Bytes()) {
		t.Fatal("failed sweep stdout differs from its saved manifest")
	}
}

func TestSweepFailFastStopsAfterFailedStep(t *testing.T) {
	controllers := sweepControllers([]int32{-36000, 0, 36000}, map[int]int32{1: 18000}, nil)
	session, restore := stubSweepBackend(t, controllers, nil)
	defer restore()

	outDir := t.TempDir()
	var stderr bytes.Buffer
	root := NewRootCommand("test")
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"sweep", "--device", "Link", "--from", "-10", "--to", "10", "--steps", "3",
		"--out-dir", outDir, "--fail-fast",
	})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "steps 1") {
		t.Fatalf("sweep error = %v", err)
	}
	if len(session.captures) != 2 || controllers[4].panTiltSets != 0 {
		t.Fatalf("fail-fast captured %d frames and moved the third writer %d times", len(session.captures), controllers[4].panTiltSets)
	}
	manifestData, err := os.ReadFile(filepath.Join(outDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest sweepManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.RequestedSteps != 3 || len(manifest.Steps) != 2 || manifest.Steps[1].Verified {
		t.Fatalf("partial fail-fast manifest = %#v", manifest)
	}
}

func TestSweepValidatesFlagsBeforeOpeningCamera(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing from", args: []string{"--to", "10", "--steps", "2", "--out-dir", "frames"}, want: "--from is required"},
		{name: "missing to", args: []string{"--from", "-10", "--steps", "2", "--out-dir", "frames"}, want: "--to is required"},
		{name: "too few steps", args: []string{"--from", "-10", "--to", "10", "--steps", "1", "--out-dir", "frames"}, want: "--steps must be at least 2"},
		{name: "missing directory", args: []string{"--from", "-10", "--to", "10", "--steps", "2"}, want: "--out-dir is required"},
		{name: "nonfinite from", args: []string{"--from", "NaN", "--to", "10", "--steps", "2", "--out-dir", "frames"}, want: "--from must be a finite number"},
		{name: "nonfinite to", args: []string{"--from", "0", "--to", "+Inf", "--steps", "2", "--out-dir", "frames"}, want: "--to must be a finite number"},
		{name: "nonfinite tilt", args: []string{"--from", "0", "--to", "10", "--tilt", "NaN", "--steps", "2", "--out-dir", "frames"}, want: "--tilt must be a finite number"},
		{name: "invalid timing", args: []string{"--from", "0", "--to", "10", "--steps", "2", "--out-dir", "frames", "--settle", "5s", "--timeout", "5s"}, want: "--settle must be less than --timeout"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := NewRootCommand("test")
			root.SetArgs(append([]string{"sweep", "--device", "Link"}, test.args...))
			if err := root.Execute(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Execute error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestSweepRejectsUnsupportedCameraAndBackend(t *testing.T) {
	t.Run("no absolute pan tilt", func(t *testing.T) {
		controller := newFakePTZController()
		controller.capabilities.PanTiltAbsolute = false
		session, restore := stubPTZBackendWithSession(t, controller)
		defer restore()

		root := NewRootCommand("test")
		root.SetArgs([]string{"sweep", "--device", "Link", "--from", "0", "--to", "1", "--steps", "2", "--out-dir", t.TempDir()})
		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), `camera "Insta360 Link" does not support absolute UVC pan/tilt control`) {
			t.Fatalf("unsupported camera error = %v", err)
		}
		if !session.closed {
			t.Fatal("capture session remained open for an unsupported camera")
		}
	})

	t.Run("ffmpeg backend", func(t *testing.T) {
		root := NewRootCommand("test")
		root.SetArgs([]string{"sweep", "--device", "Link", "--from", "0", "--to", "1", "--steps", "2", "--out-dir", t.TempDir(), "--local-backend", "ffmpeg"})
		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "sweep requires the native local capture backend") {
			t.Fatalf("backend error = %v", err)
		}
	})

	t.Run("output path is file", func(t *testing.T) {
		outPath := filepath.Join(t.TempDir(), "existing")
		if err := os.WriteFile(outPath, []byte("existing"), 0o600); err != nil {
			t.Fatal(err)
		}
		root := NewRootCommand("test")
		root.SetArgs([]string{"sweep", "--device", "Link", "--from", "0", "--to", "1", "--steps", "2", "--out-dir", outPath})
		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "create sweep output directory") {
			t.Fatalf("output directory error = %v", err)
		}
	})
}

func TestSweepUsesSavedCameraAndClampsEndpoints(t *testing.T) {
	controllers := sweepControllers([]int32{-324000, 324000}, nil, nil)
	_, restore := stubSweepBackend(t, controllers, nil)
	defer restore()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := config.Save(configPath, config.Config{Cameras: []config.Camera{{Name: "desk", Protocol: "local", Device: "Link"}}}); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	root := NewRootCommand("test")
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"--config", configPath, "sweep", "desk", "--from", "-180", "--to", "180", "--steps", "2", "--out-dir", outDir})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(outDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest sweepManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.FromDegrees != -90 || manifest.ToDegrees != 90 || manifest.TiltDegrees != -2 {
		t.Fatalf("clamped sweep metadata = %#v", manifest)
	}
}

func sweepControllers(pan []int32, failures map[int]int32, events *[]string) []*fakePTZController {
	controllers := make([]*fakePTZController, 0, len(pan)*2)
	for index, requested := range pan {
		writer := newFakePTZController()
		writer.events = events
		verifier := newFakePTZController()
		verifier.events = events
		verifier.status.Pan.Cur = requested
		if observed, failed := failures[index]; failed {
			verifier.status.Pan.Cur = observed
		}
		controllers = append(controllers, writer, verifier)
	}
	return controllers
}

func stubSweepBackend(t *testing.T, controllers []*fakePTZController, events *[]string) (*fakePTZSession, func()) {
	t.Helper()
	session, restore := stubPTZBackendWithSession(t, controllers[0])
	session.events = events
	opens := 0
	ptzOpenController = func(uniqueID string) (ptzController, error) {
		if uniqueID != "camera-id" {
			t.Fatalf("controller device ID = %q", uniqueID)
		}
		if events != nil {
			*events = append(*events, "controller-open")
		}
		if opens >= len(controllers) {
			t.Fatalf("unexpected controller open %d", opens)
		}
		controller := controllers[opens]
		opens++
		return controller, nil
	}
	return session, restore
}

func countSweepEvents(events []string, wanted string) int {
	count := 0
	for _, event := range events {
		if event == wanted {
			count++
		}
	}
	return count
}

func assertSweepStepOrdering(t *testing.T, events []string, steps int) {
	t.Helper()
	var filtered []string
	for _, event := range events {
		if event == "set-pan-tilt" || event == "controller-close" || event == "status" || strings.HasPrefix(event, "capture:") {
			filtered = append(filtered, event)
		}
	}
	if len(filtered) == 0 || filtered[0] != "status" {
		t.Fatalf("initial position was not read before moving: %v", events)
	}
	filtered = filtered[1:]
	for index := range steps {
		if len(filtered) < 6 || filtered[0] != "set-pan-tilt" || filtered[1] != "controller-close" || filtered[2] != "status" || filtered[3] != "status" || filtered[4] != "controller-close" || !strings.HasPrefix(filtered[5], "capture:") {
			t.Fatalf("step %d did not move, close its writer, verify, and capture in order: %v", index, events)
		}
		filtered = filtered[6:]
	}
	if len(filtered) != 0 {
		t.Fatalf("unexpected sweep control events: %v", filtered)
	}
}
