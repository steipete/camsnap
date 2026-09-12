//go:build linux

package cli

import "testing"

func TestLinuxDeviceSelector(t *testing.T) {
	for input, want := range map[string]string{"50": "/dev/video50", "0": "/dev/video0", "/dev/v4l/by-id/camera": "/dev/v4l/by-id/camera"} {
		got, err := linuxDeviceSelector(input)
		if err != nil || got != want {
			t.Fatalf("%q: got %q, %v; want %q", input, got, err, want)
		}
	}
	if _, err := linuxDeviceSelector("-1"); err == nil {
		t.Fatal("negative index accepted")
	}
	if _, err := linuxCameraName("/dev/null"); err == nil {
		t.Fatal("non-camera accepted")
	}
}
