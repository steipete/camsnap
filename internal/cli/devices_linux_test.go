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

func TestLinuxCaptureCapabilities(t *testing.T) {
	for _, tt := range []struct {
		name                     string
		capabilities, deviceCaps uint32
		want                     bool
	}{
		{"single-planar capture", 0x04000001, 0, true},
		{"multi-planar capture", 0x04001000, 0, true},
		{"capture without streaming", 0x00000001, 0, false},
		{"device capture without streaming", 0x84000001, 0x00000001, false},
		{"output only", 0x00000002, 0, false},
		{"metadata only", 0x00800000, 0, false},
		{"device single-planar capture", 0x80000000, 0x04000001, true},
		{"device multi-planar capture", 0x80000000, 0x04001000, true},
		{"device caps override aggregate capture", 0x80000001, 0x00800000, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := linuxCaptureCapabilities(tt.capabilities, tt.deviceCaps); got != tt.want {
				t.Fatalf("linuxCaptureCapabilities(%#x, %#x) = %v, want %v", tt.capabilities, tt.deviceCaps, got, tt.want)
			}
		})
	}
}
