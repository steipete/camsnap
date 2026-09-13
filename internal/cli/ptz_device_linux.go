//go:build linux

package cli

import (
	"fmt"
	"path/filepath"
)

func resolveNativePTZDevice(selector string) (localDevice, error) {
	devices, err := linuxLocalDevices()
	if err != nil {
		return localDevice{}, err
	}
	if selector == "" && len(devices) > 0 {
		return devices[0], nil
	}
	path, err := linuxDeviceSelector(selector)
	if err != nil {
		return localDevice{}, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(path); resolveErr == nil {
		path = resolved
	}
	for _, d := range devices {
		if d.ID == path {
			return d, nil
		}
	}
	return localDevice{}, fmt.Errorf("V4L2 capture camera %q not found", selector)
}
