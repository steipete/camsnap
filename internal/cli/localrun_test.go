package cli

import (
	"errors"
	"strings"
	"testing"
)

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
