//go:build linux

package cli

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

// VIDIOC_QUERYCAP is defined by linux/videodev2.h. The capability struct is
// 104 bytes; device_caps is authoritative when V4L2_CAP_DEVICE_CAPS is set.
func linuxCameraName(path string) (string, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return "", err
	}
	defer func() { _ = unix.Close(fd) }()
	var capability struct {
		Driver                            [16]byte
		Card                              [32]byte
		Bus                               [32]byte
		Version, Capabilities, DeviceCaps uint32
		Reserved                          [3]uint32
	}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), 0x80685600, uintptr(unsafe.Pointer(&capability)))
	if errno != 0 {
		return "", errno
	}
	flags := capability.Capabilities
	if flags&0x80000000 != 0 {
		flags = capability.DeviceCaps
	}
	if flags&1 == 0 {
		return "", fmt.Errorf("not a V4L2 video capture device")
	}
	return strings.TrimRight(string(capability.Card[:]), "\x00"), nil
}

func linuxLocalDevices() ([]localDevice, error) {
	paths, err := filepath.Glob("/dev/video*")
	if err != nil {
		return nil, err
	}
	sort.Slice(paths, func(i, j int) bool {
		a, _ := strconv.Atoi(strings.TrimPrefix(paths[i], "/dev/video"))
		b, _ := strconv.Atoi(strings.TrimPrefix(paths[j], "/dev/video"))
		return a < b
	})
	devices := []localDevice{}
	for _, path := range paths {
		name, err := linuxCameraName(path)
		if err != nil {
			continue
		} // ISP subdevices and inaccessible nodes cannot capture.
		devices = append(devices, localDevice{ID: path, Index: strings.TrimPrefix(path, "/dev/video"), Name: name, IsDefault: len(devices) == 0})
	}
	return devices, nil
}

func linuxDeviceSelector(selector string) (string, error) {
	if filepath.IsAbs(selector) {
		return selector, nil
	}
	if index, err := strconv.Atoi(selector); err == nil {
		if index < 0 {
			return "", fmt.Errorf("camera index must be nonnegative")
		}
		return fmt.Sprintf("/dev/video%d", index), nil
	}
	devices, err := linuxLocalDevices()
	if err != nil {
		return "", err
	}
	var matches []localDevice
	for _, d := range devices {
		if strings.EqualFold(d.Name, selector) {
			matches = append(matches, d)
		}
	}
	if len(matches) == 1 {
		return matches[0].ID, nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("camera name %q is ambiguous; use a /dev/video path", selector)
	}
	return "", fmt.Errorf("camera %q not found; run camsnap devices and use its index, name, or /dev/video path", selector)
}

func linuxDefaultCamera() (localDevice, bool, error) {
	devices, err := linuxLocalDevices()
	if err != nil || len(devices) == 0 {
		return localDevice{}, false, err
	}
	return devices[0], true, nil
}
