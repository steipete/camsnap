//go:build darwin && cgo

package cli

import "github.com/steipete/camsnap/internal/uvc"

func openNativePTZController(uniqueID string) (ptzController, error) {
	return uvc.Open(uniqueID)
}
