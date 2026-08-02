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

func nativeCaptureFrame(device string, warmup time.Duration, output string) (string, error) {
	devices, err := nativeEnumerateLocalDevices()
	if err != nil {
		return device, fmt.Errorf("enumerate native cameras: %w", err)
	}
	resolved, err := resolveNativeDevice(devices, device)
	if err != nil {
		return device, err
	}
	return resolved.Index, avf.CaptureFrame(resolved.ID, warmup, output)
}

func resolveNativeDevice(devices []localDevice, selector string) (localDevice, error) {
	if index, err := strconv.Atoi(selector); err == nil {
		if index < 0 || index >= len(devices) {
			return localDevice{}, fmt.Errorf("native camera index %d is out of range (found %d devices)", index, len(devices))
		}
		return devices[index], nil
	}
	for _, device := range devices {
		if device.ID == selector || device.Name == selector {
			return device, nil
		}
	}
	return localDevice{}, fmt.Errorf("native camera %q not found", selector)
}
