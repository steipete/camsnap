//go:build darwin && cgo

package uvc

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

// Controller controls the camera terminal of one USB UVC device.
type Controller struct {
	mu           sync.Mutex
	handle       *C.UVCBridgeController
	capabilities Capabilities
}

// Open finds the USB camera represented by an AVFoundation unique ID.
func Open(uniqueID string) (*Controller, error) {
	locationID, vendorID, productID, err := ParseUSBUniqueID(uniqueID)
	if err != nil {
		return nil, err
	}

	var handle *C.UVCBridgeController
	var controls C.uint32_t
	var cErr *C.char
	if C.uvc_open(
		C.uint32_t(locationID),
		C.uint16_t(vendorID),
		C.uint16_t(productID),
		&handle,
		&controls,
		&cErr,
	) == 0 {
		return nil, bridgeError(cErr, "open USB camera")
	}

	controller := &Controller{
		handle:       handle,
		capabilities: capabilitiesFromControls(uint32(controls)),
	}
	runtime.SetFinalizer(controller, (*Controller).finalize)
	return controller, nil
}

// Capabilities returns the PTZ controls advertised by the camera terminal.
func (c *Controller) Capabilities() Capabilities {
	if c == nil {
		return Capabilities{}
	}
	return c.capabilities
}

// Status reads the current values and ranges of available absolute controls.
func (c *Controller) Status() (Status, error) {
	if c == nil {
		return Status{}, fmt.Errorf("uvc: controller is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.statusLocked()
}

// PanTiltRange reads the ranges for the paired absolute pan and tilt control.
func (c *Controller) PanTiltRange() (pan, tilt Range, err error) {
	if c == nil {
		return Range{}, Range{}, fmt.Errorf("uvc: controller is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.panTiltRangeLocked()
}

// ZoomRange reads the range for the absolute zoom control.
func (c *Controller) ZoomRange() (Range, error) {
	if c == nil {
		return Range{}, fmt.Errorf("uvc: controller is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.zoomRangeLocked()
}

// SetPanTilt clamps and writes the paired absolute pan and tilt values.
func (c *Controller) SetPanTilt(pan, tilt int32) (appliedPan, appliedTilt int32, err error) {
	if c == nil {
		return 0, 0, fmt.Errorf("uvc: controller is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	panRange, tiltRange, err := c.panTiltRangeLocked()
	if err != nil {
		return 0, 0, err
	}
	appliedPan = panRange.Clamp(pan)
	appliedTilt = tiltRange.Clamp(tilt)
	if err := c.controlLocked(selPanTiltAbsolute, reqSetCur, encodePanTilt(appliedPan, appliedTilt)); err != nil {
		return 0, 0, err
	}
	return appliedPan, appliedTilt, nil
}

// SetZoom clamps and writes an absolute zoom value.
func (c *Controller) SetZoom(zoom int32) (int32, error) {
	if c == nil {
		return 0, fmt.Errorf("uvc: controller is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	zoomRange, err := c.zoomRangeLocked()
	if err != nil {
		return 0, err
	}
	applied := zoomRange.Clamp(zoom)
	if err := c.controlLocked(selZoomAbsolute, reqSetCur, encodeZoom(applied)); err != nil {
		return 0, err
	}
	return applied, nil
}

// Home resets available absolute controls to their advertised defaults.
func (c *Controller) Home() (Status, error) {
	if c == nil {
		return Status{}, fmt.Errorf("uvc: controller is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.capabilities.PanTiltAbsolute && !c.capabilities.ZoomAbsolute {
		return Status{}, fmt.Errorf("uvc: camera has no absolute PTZ controls")
	}
	if c.capabilities.PanTiltAbsolute {
		panRange, tiltRange, err := c.panTiltRangeLocked()
		if err != nil {
			return Status{}, err
		}
		if err := c.controlLocked(selPanTiltAbsolute, reqSetCur, encodePanTilt(panRange.Def, tiltRange.Def)); err != nil {
			return Status{}, err
		}
	}
	if c.capabilities.ZoomAbsolute {
		zoomRange, err := c.zoomRangeLocked()
		if err != nil {
			return Status{}, err
		}
		if err := c.controlLocked(selZoomAbsolute, reqSetCur, encodeZoom(zoomRange.Def)); err != nil {
			return Status{}, err
		}
	}
	return c.statusLocked()
}

// Close releases the IOKit interfaces held by the controller.
func (c *Controller) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handle != nil {
		C.uvc_close(c.handle)
		c.handle = nil
	}
	runtime.SetFinalizer(c, nil)
	return nil
}

func (c *Controller) finalize() {
	_ = c.Close()
}

func (c *Controller) statusLocked() (Status, error) {
	if err := c.ensureOpenLocked(); err != nil {
		return Status{}, err
	}

	var status Status
	if c.capabilities.PanTiltAbsolute {
		panRange, tiltRange, err := c.panTiltRangeLocked()
		if err != nil {
			return Status{}, err
		}
		pan, tilt, err := c.panTiltLocked(reqGetCur)
		if err != nil {
			return Status{}, err
		}
		status.Pan = &AxisStatus{Cur: pan, Range: panRange}
		status.Tilt = &AxisStatus{Cur: tilt, Range: tiltRange}
	}
	if c.capabilities.ZoomAbsolute {
		zoomRange, err := c.zoomRangeLocked()
		if err != nil {
			return Status{}, err
		}
		zoom, err := c.zoomLocked(reqGetCur)
		if err != nil {
			return Status{}, err
		}
		status.Zoom = &AxisStatus{Cur: zoom, Range: zoomRange}
	}
	return status, nil
}

func (c *Controller) panTiltRangeLocked() (Range, Range, error) {
	if !c.capabilities.PanTiltAbsolute {
		return Range{}, Range{}, fmt.Errorf("uvc: camera does not support absolute pan/tilt")
	}
	minPan, minTilt, err := c.panTiltLocked(reqGetMin)
	if err != nil {
		return Range{}, Range{}, err
	}
	maxPan, maxTilt, err := c.panTiltLocked(reqGetMax)
	if err != nil {
		return Range{}, Range{}, err
	}
	resPan, resTilt, err := c.panTiltLocked(reqGetRes)
	if err != nil {
		return Range{}, Range{}, err
	}
	defPan, defTilt, err := c.panTiltLocked(reqGetDef)
	if err != nil {
		defPan, defTilt = 0, 0
	}
	pan := Range{Min: minPan, Max: maxPan, Res: resPan}
	tilt := Range{Min: minTilt, Max: maxTilt, Res: resTilt}
	pan.Def = pan.Clamp(defPan)
	tilt.Def = tilt.Clamp(defTilt)
	return pan, tilt, nil
}

func (c *Controller) zoomRangeLocked() (Range, error) {
	if !c.capabilities.ZoomAbsolute {
		return Range{}, fmt.Errorf("uvc: camera does not support absolute zoom")
	}
	minimum, err := c.zoomLocked(reqGetMin)
	if err != nil {
		return Range{}, err
	}
	maximum, err := c.zoomLocked(reqGetMax)
	if err != nil {
		return Range{}, err
	}
	res, err := c.zoomLocked(reqGetRes)
	if err != nil {
		return Range{}, err
	}
	def, err := c.zoomLocked(reqGetDef)
	if err != nil {
		def = minimum
	}
	zoomRange := Range{Min: minimum, Max: maximum, Res: res}
	zoomRange.Def = zoomRange.Clamp(def)
	return zoomRange, nil
}

func (c *Controller) panTiltLocked(request byte) (int32, int32, error) {
	buf := make([]byte, 8)
	if err := c.controlLocked(selPanTiltAbsolute, request, buf); err != nil {
		return 0, 0, err
	}
	return decodePanTilt(buf)
}

func (c *Controller) zoomLocked(request byte) (int32, error) {
	buf := make([]byte, 2)
	if err := c.controlLocked(selZoomAbsolute, request, buf); err != nil {
		return 0, err
	}
	return decodeZoom(buf)
}

func (c *Controller) controlLocked(selector, request byte, data []byte) error {
	if err := c.ensureOpenLocked(); err != nil {
		return err
	}
	var cErr *C.char
	if C.uvc_control(
		c.handle,
		C.uint8_t(selector),
		C.uint8_t(request),
		unsafe.Pointer(&data[0]),
		C.uint16_t(len(data)),
		&cErr,
	) == 0 {
		return bridgeError(cErr, "perform camera control request")
	}
	return nil
}

func (c *Controller) ensureOpenLocked() error {
	if c.handle == nil {
		return fmt.Errorf("uvc: controller is closed")
	}
	return nil
}

func bridgeError(cErr *C.char, fallback string) error {
	if cErr == nil {
		return fmt.Errorf("uvc: %s", fallback)
	}
	defer C.free(unsafe.Pointer(cErr))
	return fmt.Errorf("uvc: %s", C.GoString(cErr))
}
