// Package uvc controls pan/tilt/zoom on local USB UVC cameras.
package uvc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Camera terminal bmControls bit positions (UVC 1.5, table 3-6).
const (
	ctrlBitZoomAbsolute    = 9
	ctrlBitZoomRelative    = 10
	ctrlBitPanTiltAbsolute = 11
	ctrlBitPanTiltRelative = 12
)

const arcsecPerDegree = 3600

const (
	descriptorTypeClassInterface = 0x24
	descriptorSubtypeVCHeader    = 0x01
	descriptorSubtypeInputTerm   = 0x02
)

// ErrUnsupported reports that PTZ control is unavailable in this build.
var ErrUnsupported = errors.New("uvc: PTZ control requires a cgo-enabled macOS build")

// Capabilities lists the motion controls a camera advertises.
type Capabilities struct {
	PanTiltAbsolute bool `json:"pan_tilt_absolute"`
	PanTiltRelative bool `json:"pan_tilt_relative"`
	ZoomAbsolute    bool `json:"zoom_absolute"`
	ZoomRelative    bool `json:"zoom_relative"`
}

// Any reports whether the camera advertises any PTZ control.
func (c Capabilities) Any() bool {
	return c.PanTiltAbsolute || c.PanTiltRelative || c.ZoomAbsolute || c.ZoomRelative
}

func capabilitiesFromControls(controls uint32) Capabilities {
	return Capabilities{
		PanTiltAbsolute: controls&(1<<ctrlBitPanTiltAbsolute) != 0,
		PanTiltRelative: controls&(1<<ctrlBitPanTiltRelative) != 0,
		ZoomAbsolute:    controls&(1<<ctrlBitZoomAbsolute) != 0,
		ZoomRelative:    controls&(1<<ctrlBitZoomRelative) != 0,
	}
}

// Range describes the values a control accepts in device units.
type Range struct {
	Min int32 `json:"min"`
	Max int32 `json:"max"`
	Res int32 `json:"res"`
	Def int32 `json:"def"`
}

// Clamp bounds v to the range and snaps it to the control resolution.
func (r Range) Clamp(v int32) int32 {
	if r.Res > 1 {
		steps := math.Round(float64(int64(v)-int64(r.Min)) / float64(r.Res))
		snapped := int64(r.Min) + int64(steps)*int64(r.Res)
		if snapped < int64(r.Min) {
			return r.Min
		}
		if snapped > int64(r.Max) {
			return r.Max
		}
		v = int32(snapped)
	}
	if v < r.Min {
		return r.Min
	}
	if v > r.Max {
		return r.Max
	}
	return v
}

// PercentOf maps a device-unit value to 0-100 across the range.
func (r Range) PercentOf(v int32) float64 {
	if r.Max <= r.Min {
		return 0
	}
	v = r.Clamp(v)
	return float64(int64(v)-int64(r.Min)) / float64(int64(r.Max)-int64(r.Min)) * 100
}

// FromPercent maps 0-100 to a clamped device-unit value.
func (r Range) FromPercent(p float64) int32 {
	if r.Max <= r.Min {
		return r.Min
	}
	p = math.Max(0, math.Min(100, p))
	v := float64(r.Min) + p/100*float64(int64(r.Max)-int64(r.Min))
	return r.Clamp(int32(math.Round(v)))
}

// AxisStatus pairs a control's current value with its range.
type AxisStatus struct {
	Cur   int32 `json:"cur"`
	Range Range `json:"range"`
}

// Status contains the current values and ranges for available absolute controls.
type Status struct {
	Pan  *AxisStatus `json:"pan,omitempty"`
	Tilt *AxisStatus `json:"tilt,omitempty"`
	Zoom *AxisStatus `json:"zoom,omitempty"`
}

// DegreesToArcsec converts degrees to the UVC pan/tilt wire unit.
func DegreesToArcsec(deg float64) int32 {
	if deg >= float64(math.MaxInt32)/arcsecPerDegree {
		return math.MaxInt32
	}
	if deg <= float64(math.MinInt32)/arcsecPerDegree {
		return math.MinInt32
	}
	return int32(math.Round(deg * arcsecPerDegree))
}

// ArcsecToDegrees converts the UVC pan/tilt wire unit to degrees.
func ArcsecToDegrees(arcsec int32) float64 {
	return float64(arcsec) / arcsecPerDegree
}

// ParseUSBUniqueID splits an AVFoundation unique ID such as
// "0x21100002e1a4c06" into its USB location ID, vendor ID, and product ID.
// AVFoundation builds these IDs as 0x<locationID><vendorID><productID>.
func ParseUSBUniqueID(id string) (locationID uint32, vendorID, productID uint16, err error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(id), "0x")
	if trimmed == "" || len(trimmed) > 16 {
		return 0, 0, 0, fmt.Errorf("uvc: %q is not a USB camera unique ID", id)
	}
	value, parseErr := strconv.ParseUint(trimmed, 16, 64)
	if parseErr != nil {
		return 0, 0, 0, fmt.Errorf("uvc: %q is not a USB camera unique ID", id)
	}
	locationID = uint32(value >> 32)
	if locationID == 0 {
		return 0, 0, 0, fmt.Errorf("uvc: %q does not contain a USB location ID", id)
	}
	return locationID, uint16(value >> 16), uint16(value), nil
}

func encodePanTilt(pan, tilt int32) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(pan))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(tilt))
	return buf
}

func decodePanTilt(buf []byte) (pan, tilt int32, err error) {
	if len(buf) != 8 {
		return 0, 0, fmt.Errorf("uvc: pan/tilt payload is %d bytes, want 8", len(buf))
	}
	pan = int32(binary.LittleEndian.Uint32(buf[0:4]))
	tilt = int32(binary.LittleEndian.Uint32(buf[4:8]))
	return pan, tilt, nil
}

func encodeZoom(v int32) []byte {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(v))
	return buf
}

func decodeZoom(buf []byte) (int32, error) {
	if len(buf) != 2 {
		return 0, fmt.Errorf("uvc: zoom payload is %d bytes, want 2", len(buf))
	}
	return int32(binary.LittleEndian.Uint16(buf)), nil
}

func parseCameraTerminalDescriptor(descriptors []byte) (unitID uint8, controls uint32, ok bool) {
	if len(descriptors) < 7 || descriptors[0] < 7 || descriptors[1] != descriptorTypeClassInterface || descriptors[2] != descriptorSubtypeVCHeader {
		return 0, 0, false
	}

	totalLength := int(binary.LittleEndian.Uint16(descriptors[5:7]))
	if totalLength > len(descriptors) {
		totalLength = len(descriptors)
	}
	for offset := 0; offset < totalLength; {
		length := int(descriptors[offset])
		if length == 0 || offset+length > totalLength {
			return 0, 0, false
		}
		descriptor := descriptors[offset : offset+length]
		if len(descriptor) >= 15 && descriptor[1] == descriptorTypeClassInterface && descriptor[2] == descriptorSubtypeInputTerm {
			controlSize := int(descriptor[14])
			if 15+controlSize > len(descriptor) {
				return 0, 0, false
			}
			for i := 0; i < controlSize && i < 4; i++ {
				controls |= uint32(descriptor[15+i]) << (8 * i)
			}
			return descriptor[3], controls, true
		}
		offset += length
	}
	return 0, 0, false
}
