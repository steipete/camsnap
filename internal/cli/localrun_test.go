package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/steipete/camsnap/internal/capture"
)

func TestPrepareNativeSnapshotResolvesBeforePermissionPreflight(t *testing.T) {
	preflightCalled := false
	request := localCaptureRequest{options: capture.Options{Device: "9"}}
	_, err := prepareNativeSnapshot(
		context.Background(),
		request,
		func(string) (localDevice, error) {
			return localDevice{}, errors.New("native camera index 9 is out of range (found 5 devices)")
		},
		func(context.Context, bool, func(...any)) error {
			preflightCalled = true
			return nil
		},
	)
	if err == nil || !strings.Contains(err.Error(), "index 9 is out of range") {
		t.Fatalf("prepareNativeSnapshot() error = %v", err)
	}
	if preflightCalled {
		t.Fatal("permission preflight ran after device resolution failed")
	}
}

func TestLocalCapturePermissionRemediation(t *testing.T) {
	err := localCaptureFailure(errors.New("exit status 1"), "Failed to create AVCaptureDeviceInput: Operation not permitted")
	for _, want := range []string{"(permission)", "System Settings → Privacy & Security → Camera", "Over SSH", "tccutil reset Camera"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}
}

func TestLocalCaptureIncludesSupportedModes(t *testing.T) {
	output := "Selected framerate (60.000000) is not supported by the device.\nSupported modes:\n  1280x720@[1.000000 30.000000]fps\n"
	err := localCaptureFailure(errors.New("exit status 1"), output)
	if !strings.Contains(err.Error(), "1280x720@[1.000000 30.000000]fps") {
		t.Fatalf("supported mode missing from error: %v", err)
	}
}
