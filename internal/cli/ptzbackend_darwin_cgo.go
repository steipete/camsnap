//go:build darwin && cgo

package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/steipete/camsnap/internal/avf"
	"github.com/steipete/camsnap/internal/uvc"
)

func openNativePTZSession(ctx context.Context, uniqueID string) (io.Closer, error) {
	if err := preflightNativeCamera(ctx, true, nil); err != nil {
		return nil, err
	}
	session, err := avf.OpenSession(uniqueID)
	if err != nil && isNativePermissionError(err) {
		return nil, fmt.Errorf("%w\n%s", err, cameraPermissionRemediation)
	}
	return session, err
}

func openNativePTZController(uniqueID string) (ptzController, error) {
	return uvc.Open(uniqueID)
}
