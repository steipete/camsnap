//go:build !darwin || !cgo

package cli

import (
	"context"
	"io"

	"github.com/steipete/camsnap/internal/uvc"
)

func openNativePTZSession(context.Context, string) (io.Closer, error) {
	return nil, uvc.ErrUnsupported
}

func openNativePTZController(string) (ptzController, error) {
	return nil, uvc.ErrUnsupported
}
