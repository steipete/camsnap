//go:build darwin && cgo

package avf

import (
	"os"
	"strings"
	"testing"
)

func TestAuthorizationStatusString(t *testing.T) {
	tests := []struct {
		status int
		want   string
	}{
		{status: 0, want: "notDetermined"},
		{status: 1, want: "restricted"},
		{status: 2, want: "denied"},
		{status: 3, want: "authorized"},
		{status: 99, want: "unknown"},
	}

	for _, test := range tests {
		if got := authorizationStatusString(test.status); got != test.want {
			t.Errorf("authorizationStatusString(%d) = %q, want %q", test.status, got, test.want)
		}
	}
}

func TestInfoPlistContainsCameraIdentity(t *testing.T) {
	data, err := os.ReadFile("Info.plist")
	if err != nil {
		t.Fatal(err)
	}
	plist := string(data)
	for _, want := range []string{
		"NSCameraUsageDescription",
		"com.steipete.camsnap",
		"CFBundleName",
		"NSCameraUseContinuityCameraDeviceType",
	} {
		if !strings.Contains(plist, want) {
			t.Errorf("Info.plist does not contain %q", want)
		}
	}
}

func TestCaptureFrameRequiresOutputPath(t *testing.T) {
	if err := CaptureFrame("", 0, ""); err == nil {
		t.Fatal("CaptureFrame returned nil error for an empty output path")
	}
}
