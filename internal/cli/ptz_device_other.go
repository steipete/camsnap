//go:build !linux

package cli

import (
	"fmt"
)

func resolveNativePTZDevice(selector string) (localDevice, error) {
	devices, err := nativeEnumerateLocalDevices()
	if err != nil {
		return localDevice{}, fmt.Errorf("enumerate native cameras: %w", err)
	}
	if selector != "" {
		return resolveNativeDevice(devices, selector)
	}
	if device, ok := defaultNativeDevice(devices); ok {
		return device, nil
	}
	return localDevice{}, fmt.Errorf("no default native camera is available")
}
