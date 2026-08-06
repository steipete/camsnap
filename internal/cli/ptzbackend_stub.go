//go:build !darwin || !cgo

package cli

import "github.com/steipete/camsnap/internal/uvc"

func openNativePTZController(string) (ptzController, error) {
	return nil, uvc.ErrUnsupported
}
