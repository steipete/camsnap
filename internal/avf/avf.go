//go:build darwin && cgo

// Package avf captures frames from local macOS cameras through AVFoundation.
package avf

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -fblocks
#cgo LDFLAGS: -framework AVFoundation -framework CoreMedia -framework CoreVideo -framework ImageIO -framework CoreGraphics -framework Foundation
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// DefaultWarmup lets auto-exposure and auto-white-balance settle before capture.
const DefaultWarmup = time.Second

// Device describes a local video capture device.
type Device struct {
	UniqueID  string
	Name      string
	IsDefault bool
}

// Session keeps a camera's video stream active without saving any frames.
type Session struct {
	mu     sync.Mutex
	handle unsafe.Pointer
}

var (
	accessSequence atomic.Uint64
	accessMu       sync.Mutex
	accessWaiters  = make(map[uint64]chan bool)
)

// ListDevices returns built-in wide-angle, external, and Continuity Camera devices.
func ListDevices() ([]Device, error) {
	var cDevices *C.AVFDevice
	var count C.size_t
	var cErr *C.char
	if C.avf_list_devices(&cDevices, &count, &cErr) == 0 {
		return nil, bridgeError(cErr, "list video capture devices")
	}
	defer C.avf_free_devices(cDevices, count)

	if count == 0 {
		return []Device{}, nil
	}

	items := unsafe.Slice(cDevices, int(count))
	devices := make([]Device, len(items))
	for i := range items {
		devices[i] = Device{
			UniqueID:  C.GoString(items[i].unique_id),
			Name:      C.GoString(items[i].name),
			IsDefault: items[i].is_default != 0,
		}
	}
	return devices, nil
}

// AuthorizationStatus reports macOS camera authorization as notDetermined,
// restricted, denied, authorized, or unknown for an unexpected future value.
func AuthorizationStatus() string {
	return authorizationStatusString(int(C.avf_authorization_status()))
}

// RequestAccess triggers the macOS camera permission prompt when authorization
// has not yet been determined and waits for AVFoundation's completion handler.
func RequestAccess(ctx context.Context) (bool, error) {
	if ctx == nil {
		return false, errors.New("avf: nil context")
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}

	token := accessSequence.Add(1)
	result := make(chan bool, 1)
	accessMu.Lock()
	accessWaiters[token] = result
	accessMu.Unlock()
	defer func() {
		accessMu.Lock()
		delete(accessWaiters, token)
		accessMu.Unlock()
	}()

	var cErr *C.char
	if C.avf_request_access(C.ulonglong(token), &cErr) == 0 {
		return false, bridgeError(cErr, "request camera access")
	}

	select {
	case granted := <-result:
		return granted, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

// OpenSession starts a video capture session for deviceID and waits for its
// first frame. An empty deviceID selects the default camera.
func OpenSession(deviceID string) (*Session, error) {
	cDeviceID := C.CString(deviceID)
	defer C.free(unsafe.Pointer(cDeviceID))

	var cErr *C.char
	handle := C.avf_open_session(cDeviceID, &cErr)
	if handle == nil {
		return nil, bridgeError(cErr, "open capture session")
	}
	return &Session{handle: handle}, nil
}

// Close stops the video capture session and releases its camera resources.
func (s *Session) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.handle == nil {
		return nil
	}

	handle := s.handle
	s.handle = nil
	var cErr *C.char
	if C.avf_close_session(handle, &cErr) == 0 {
		return bridgeError(cErr, "close capture session")
	}
	return nil
}

// CaptureFrame starts a capture session for deviceID and writes one JPEG frame
// to outPath. An empty deviceID selects the default camera. A non-positive
// warmup uses DefaultWarmup.
func CaptureFrame(deviceID string, warmup time.Duration, outPath string) error {
	if outPath == "" {
		return errors.New("avf: output path is required")
	}
	if warmup <= 0 {
		warmup = DefaultWarmup
	}

	cDeviceID := C.CString(deviceID)
	defer C.free(unsafe.Pointer(cDeviceID))
	cOutPath := C.CString(outPath)
	defer C.free(unsafe.Pointer(cOutPath))

	var cErr *C.char
	if C.avf_capture_frame(cDeviceID, C.double(warmup.Seconds()), cOutPath, &cErr) == 0 {
		return bridgeError(cErr, "capture frame")
	}
	return nil
}

func authorizationStatusString(status int) string {
	switch status {
	case 0:
		return "notDetermined"
	case 1:
		return "restricted"
	case 2:
		return "denied"
	case 3:
		return "authorized"
	default:
		return "unknown"
	}
}

func bridgeError(cErr *C.char, fallback string) error {
	if cErr == nil {
		return fmt.Errorf("avf: %s", fallback)
	}
	defer C.free(unsafe.Pointer(cErr))
	return fmt.Errorf("avf: %s", C.GoString(cErr))
}

//export goAVFAccessResult
func goAVFAccessResult(token C.ulonglong, granted C.int) {
	accessMu.Lock()
	waiter := accessWaiters[uint64(token)]
	accessMu.Unlock()
	if waiter == nil {
		return
	}

	select {
	case waiter <- granted != 0:
	default:
	}
}
