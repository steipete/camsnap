package cli

import (
	"fmt"
	"strconv"
	"strings"
)

func resolveNativeDevice(devices []localDevice, selector string) (localDevice, error) {
	if index, err := strconv.Atoi(selector); err == nil {
		if index < 0 || index >= len(devices) {
			return localDevice{}, fmt.Errorf("native camera index %d is out of range (found %d devices)", index, len(devices))
		}
		return devices[index], nil
	}

	for _, device := range devices {
		if device.ID == selector {
			return device, nil
		}
	}
	for _, device := range devices {
		if device.Name == selector {
			return device, nil
		}
	}

	var matches []localDevice
	for _, device := range devices {
		if strings.EqualFold(device.Name, selector) {
			matches = append(matches, device)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return localDevice{}, fmt.Errorf("native camera %q is ambiguous; matches: %s", selector, formatNativeDevices(matches))
	}

	available := "none"
	if len(devices) > 0 {
		available = formatNativeDevices(devices)
	}
	return localDevice{}, fmt.Errorf("native camera %q not found; available: %s", selector, available)
}

func defaultNativeDevice(devices []localDevice) (localDevice, bool) {
	for _, device := range devices {
		if device.IsDefault {
			return device, true
		}
	}
	return localDevice{}, false
}

func nativeFFmpegFallbackSelector(device localDevice) string { return device.Name }

func formatNativeDevices(devices []localDevice) string {
	formatted := make([]string, len(devices))
	for i, device := range devices {
		formatted[i] = fmt.Sprintf("%s (%s)", device.Name, device.ID)
	}
	return strings.Join(formatted, ", ")
}
