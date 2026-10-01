//go:build linux

package cli

import (
	"context"
	"io"
	"strings"

	"github.com/steipete/camsnap/internal/uvc"
)

// V4L2 controls use the video node directly; no separate AVFoundation stream is needed.
func openNativePTZSession(context.Context, string) (io.Closer, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func openNativePTZController(device string) (ptzController, error) { return uvc.Open(device) }
