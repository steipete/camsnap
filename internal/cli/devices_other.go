//go:build !linux

package cli

import "fmt"

func linuxLocalDevices() ([]localDevice, error) {
	return nil, fmt.Errorf("V4L2 discovery requires Linux")
}
func linuxDeviceSelector(selector string) (string, error) { return selector, nil }
func linuxDefaultCamera() (localDevice, bool, error)      { return localDevice{}, false, nil }
