//go:build !darwin || !cgo

package uvc

// Controller is unavailable outside cgo-enabled macOS builds.
type Controller struct{}

// Open reports that UVC control is unavailable in this build.
func Open(string) (*Controller, error) { return nil, ErrUnsupported }

// Capabilities returns no available controls in an unsupported build.
func (*Controller) Capabilities() Capabilities { return Capabilities{} }

// Status reports that UVC control is unavailable in this build.
func (*Controller) Status() (Status, error) { return Status{}, ErrUnsupported }

// PanTiltRange reports that UVC control is unavailable in this build.
func (*Controller) PanTiltRange() (Range, Range, error) { return Range{}, Range{}, ErrUnsupported }

// ZoomRange reports that UVC control is unavailable in this build.
func (*Controller) ZoomRange() (Range, error) { return Range{}, ErrUnsupported }

// SetPanTilt reports that UVC control is unavailable in this build.
func (*Controller) SetPanTilt(int32, int32) (int32, int32, error) { return 0, 0, ErrUnsupported }

// SetZoom reports that UVC control is unavailable in this build.
func (*Controller) SetZoom(int32) (int32, error) { return 0, ErrUnsupported }

// Home reports that UVC control is unavailable in this build.
func (*Controller) Home() (Status, error) { return Status{}, ErrUnsupported }

// Close is a no-op in an unsupported build.
func (*Controller) Close() error { return nil }
