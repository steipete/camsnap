//go:build linux

package uvc

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	panID  uint32 = 0x009a0908
	tiltID uint32 = 0x009a0909
	zoomID uint32 = 0x009a090d
)

type controlInfo struct {
	ID, Type                uint32
	Name                    [32]byte
	Min, Max, Step, Default int32
	Flags                   uint32
	Reserved                [2]uint32
}

type controlValue struct {
	ID    uint32
	Value int32
}

// Controller accesses the standard V4L2 camera controls without USB permissions.
type Controller struct {
	file     *os.File
	controls map[uint32]Range
}

// Open queries writable V4L2 controls on a camera node.
func Open(path string) (*Controller, error) {
	f, err := os.OpenFile(path, os.O_RDWR|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("open V4L2 camera: %w", err)
	}
	c := &Controller{file: f, controls: make(map[uint32]Range)}
	for _, id := range []uint32{panID, tiltID, zoomID} {
		info := controlInfo{ID: id}
		err := c.ioctl(0xc0445624, unsafe.Pointer(&info))
		if errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOTTY) {
			continue
		}
		if err != nil {
			_ = c.Close()
			return nil, fmt.Errorf("query V4L2 control: %w", err)
		}
		if info.Flags&5 != 0 {
			continue
		} // disabled or read-only
		c.controls[id] = Range{Min: info.Min, Max: info.Max, Res: info.Step, Def: info.Default}
	}
	return c, nil
}

var controlIOCTL = func(fd uintptr, request uintptr, ptr unsafe.Pointer) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, request, uintptr(ptr))
	if errno != 0 {
		return errno
	}
	return nil
}

func (c *Controller) ioctl(request uintptr, ptr unsafe.Pointer) error {
	return controlIOCTL(c.file.Fd(), request, ptr)
}

// Capabilities reports supported writable absolute controls.
func (c *Controller) Capabilities() Capabilities {
	_, pan := c.controls[panID]
	_, tilt := c.controls[tiltID]
	_, zoom := c.controls[zoomID]
	return Capabilities{PanTiltAbsolute: pan && tilt, ZoomAbsolute: zoom}
}

// Status reads current control positions and ranges.
func (c *Controller) Status() (Status, error) {
	status := Status{}
	for _, axis := range []struct {
		id  uint32
		dst **AxisStatus
	}{{panID, &status.Pan}, {tiltID, &status.Tilt}, {zoomID, &status.Zoom}} {
		r, ok := c.controls[axis.id]
		if !ok {
			continue
		}
		value := controlValue{ID: axis.id}
		if err := c.ioctl(0xc008561b, unsafe.Pointer(&value)); err != nil {
			return Status{}, err
		}
		*axis.dst = &AxisStatus{Cur: value.Value, Range: r}
	}
	return status, nil
}

// PanTiltRange returns the supported pan and tilt ranges.
func (c *Controller) PanTiltRange() (Range, Range, error) {
	if !c.Capabilities().PanTiltAbsolute {
		return Range{}, Range{}, fmt.Errorf("camera has no writable absolute pan/tilt controls")
	}
	return c.controls[panID], c.controls[tiltID], nil
}

// ZoomRange returns the supported zoom range.
func (c *Controller) ZoomRange() (Range, error) {
	r, ok := c.controls[zoomID]
	if !ok {
		return Range{}, fmt.Errorf("camera has no writable absolute zoom control")
	}
	return r, nil
}
func (c *Controller) set(id uint32, value int32) (int32, error) {
	r, ok := c.controls[id]
	if !ok {
		return 0, fmt.Errorf("camera does not support control %#x", id)
	}
	v := controlValue{ID: id, Value: r.Clamp(value)}
	if err := c.ioctl(0xc008561c, unsafe.Pointer(&v)); err != nil {
		return 0, err
	}
	return v.Value, nil
}

// SetPanTilt clamps and applies absolute pan and tilt positions.
func (c *Controller) SetPanTilt(pan, tilt int32) (int32, int32, error) {
	if _, _, err := c.PanTiltRange(); err != nil {
		return 0, 0, err
	}
	p, err := c.set(panID, pan)
	if err != nil {
		return 0, 0, err
	}
	t, err := c.set(tiltID, tilt)
	return p, t, err
}

// SetZoom clamps and applies an absolute zoom position.
func (c *Controller) SetZoom(zoom int32) (int32, error) { return c.set(zoomID, zoom) }

// Home restores advertised control defaults.
func (c *Controller) Home() (Status, error) {
	if c.Capabilities().PanTiltAbsolute {
		if _, _, err := c.SetPanTilt(c.controls[panID].Def, c.controls[tiltID].Def); err != nil {
			return Status{}, err
		}
	}
	if c.Capabilities().ZoomAbsolute {
		if _, err := c.SetZoom(c.controls[zoomID].Def); err != nil {
			return Status{}, err
		}
	}
	return c.Status()
}

// Close releases the camera control descriptor.
func (c *Controller) Close() error { return c.file.Close() }
