//go:build darwin && cgo

package cli

import "testing"

func TestNativeFFmpegFallbackRequestUsesDeviceName(t *testing.T) {
	device := localDevice{ID: "camera-id", Index: "3", Name: "Stable Camera Name"}
	request := localCaptureRequest{}
	fallbackRequest := withFFmpegFallbackDevice(request, nativeFFmpegFallbackSelector(device))
	if got := fallbackRequest.options.Device; got != device.Name {
		t.Fatalf("fallback request device = %q, want device name %q", got, device.Name)
	}
}
