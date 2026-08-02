//go:build darwin && cgo

package cli

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/steipete/camsnap/internal/avf"
	"github.com/steipete/camsnap/internal/capture"
)

func defaultLocalBackend() string { return capture.LocalBackendNative }

func nativeLocalBackendAvailable() bool { return true }

func nativeCameraAuthorizationStatus() (string, error) { return avf.AuthorizationStatus(), nil }

func nativeRequestCameraAccess(ctx context.Context) (bool, error) { return avf.RequestAccess(ctx) }

func nativeEnumerateLocalDevices() ([]localDevice, error) {
	devices, err := avf.ListDevices()
	if err != nil {
		return nil, err
	}
	result := make([]localDevice, len(devices))
	for i, device := range devices {
		result[i] = localDevice{
			ID:        device.UniqueID,
			Index:     strconv.Itoa(i),
			Name:      device.Name,
			IsDefault: device.IsDefault,
		}
	}
	return result, nil
}

func nativeResolveCaptureDevice(selector string) (localDevice, error) {
	devices, err := nativeEnumerateLocalDevices()
	if err != nil {
		return localDevice{}, fmt.Errorf("enumerate native cameras: %w", err)
	}
	return resolveNativeDevice(devices, selector)
}

func nativeCaptureFrame(device localDevice, warmup time.Duration, output string) error {
	return avf.CaptureFrame(device.ID, warmup, output)
}
