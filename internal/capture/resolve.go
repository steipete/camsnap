// Package capture resolves camera settings and builds media capture commands.
package capture

import (
	"fmt"
	"strings"

	"github.com/steipete/camsnap/internal/config"
	"github.com/steipete/camsnap/internal/rtsp"
)

const (
	defaultTransport  = "tcp"
	defaultClient     = "ffmpeg"
	defaultAudioCodec = "aac"
)

// Overrides contains only flag values explicitly supplied by the user.
// Nil fields allow camera settings to take precedence over CLI defaults.
type Overrides struct {
	Transport  *string
	Stream     *string
	Client     *string
	Path       *string
	NoAudio    *bool
	AudioCodec *string
}

// Options is the fully resolved capture configuration consumed by arg builders.
type Options struct {
	URL        string
	Transport  string
	Stream     string
	Client     string
	Path       string
	NoAudio    bool
	AudioCodec string
}

// Resolve applies explicit flag, camera config, and application defaults in order.
func Resolve(cam config.Camera, overrides Overrides) (Options, error) {
	if nonEmpty(overrides.Stream) && nonEmpty(overrides.Path) {
		return Options{}, fmt.Errorf("use --path for custom RTSP token URLs; omit --stream")
	}

	transport := stringValue(overrides.Transport, cam.RTSPTransport, defaultTransport)
	switch strings.ToLower(transport) {
	case "", defaultTransport:
		transport = defaultTransport
	case "udp":
		transport = "udp"
	default:
		return Options{}, fmt.Errorf("invalid --rtsp-transport (use tcp|udp)")
	}

	path := stringValue(overrides.Path, cam.Path, "")
	stream := stringValue(overrides.Stream, cam.Stream, "")
	if path != "" {
		stream = ""
	}

	resolvedCamera := cam
	resolvedCamera.Path = path
	url, err := rtsp.BuildURL(resolvedCamera)
	if err != nil {
		return Options{}, err
	}
	if path == "" {
		url = rtsp.ReplacePath(url, stream)
	}

	return Options{
		URL:        url,
		Transport:  transport,
		Stream:     stream,
		Client:     stringValue(overrides.Client, cam.RTSPClient, defaultClient),
		Path:       path,
		NoAudio:    boolValue(overrides.NoAudio, cam.NoAudio),
		AudioCodec: stringValue(overrides.AudioCodec, cam.AudioCodec, defaultAudioCodec),
	}, nil
}

func nonEmpty(value *string) bool {
	return value != nil && *value != ""
}

func stringValue(explicit *string, cameraValue, defaultValue string) string {
	if explicit != nil {
		return *explicit
	}
	if cameraValue != "" {
		return cameraValue
	}
	return defaultValue
}

func boolValue(explicit *bool, cameraValue bool) bool {
	if explicit != nil {
		return *explicit
	}
	return cameraValue
}
