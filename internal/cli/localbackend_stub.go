//go:build !darwin || !cgo

package cli

import (
	"context"
	"errors"
	"time"

	"github.com/steipete/camsnap/internal/capture"
)

var errNativeLocalBackendUnavailable = errors.New("native local capture backend is not available in this build")

func defaultLocalBackend() string { return capture.LocalBackendFFmpeg }

func nativeLocalBackendAvailable() bool { return false }

func nativeCameraAuthorizationStatus() (string, error) {
	return "unavailable", errNativeLocalBackendUnavailable
}

func nativeRequestCameraAccess(context.Context) (bool, error) {
	return false, errNativeLocalBackendUnavailable
}

func nativeEnumerateLocalDevices() ([]localDevice, error) {
	return nil, errNativeLocalBackendUnavailable
}

func nativeResolveCaptureDevice(string) (localDevice, error) {
	return localDevice{}, errNativeLocalBackendUnavailable
}

func nativeCaptureFrame(localDevice, time.Duration, string) error {
	return errNativeLocalBackendUnavailable
}
