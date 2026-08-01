// Package capture resolves camera settings and builds media capture commands.
package capture

import (
	"fmt"
	"strings"
	"time"

	"github.com/steipete/camsnap/internal/config"
	"github.com/steipete/camsnap/internal/rtsp"
)

const (
	defaultTransport  = "tcp"
	defaultClient     = "ffmpeg"
	defaultAudioCodec = "aac"
	defaultFramerate  = 30
	defaultWarmup     = time.Second
)

// Kind identifies the source feeding a capture operation.
type Kind uint8

const (
	// KindRTSP captures from a network RTSP source.
	KindRTSP Kind = iota
	// KindLocal captures from an operating-system video device.
	KindLocal
)

// Overrides contains only flag values explicitly supplied by the user.
// Nil fields allow camera settings to take precedence over CLI defaults.
type Overrides struct {
	Transport  *string
	Stream     *string
	Client     *string
	Path       *string
	RTSPAuth   *string
	NoAudio    *bool
	AudioCodec *string
	Device     *string
	Framerate  *int
	VideoSize  *string
	Warmup     *time.Duration
}

// Options is the fully resolved capture configuration consumed by arg builders.
type Options struct {
	Kind       Kind
	URL        string
	Transport  string
	Stream     string
	Client     string
	Path       string
	NoAudio    bool
	AudioCodec string
	Device     string
	Framerate  int
	VideoSize  string
	Warmup     time.Duration
}

// Resolve applies explicit flag, camera config, and application defaults in order.
func Resolve(cam config.Camera, overrides Overrides) (Options, error) {
	if strings.EqualFold(cam.Protocol, "local") {
		return resolveLocal(cam, overrides)
	}
	if overrides.Device != nil || overrides.Framerate != nil || overrides.VideoSize != nil || overrides.Warmup != nil {
		return Options{}, fmt.Errorf("--device, --framerate, --video-size, and --warmup are only valid for local cameras")
	}
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
		Kind:       KindRTSP,
		URL:        url,
		Transport:  transport,
		Stream:     stream,
		Client:     stringValue(overrides.Client, cam.RTSPClient, defaultClient),
		Path:       path,
		NoAudio:    boolValue(overrides.NoAudio, cam.NoAudio),
		AudioCodec: stringValue(overrides.AudioCodec, cam.AudioCodec, defaultAudioCodec),
	}, nil
}

func resolveLocal(cam config.Camera, overrides Overrides) (Options, error) {
	if overrides.Transport != nil || overrides.Stream != nil || overrides.Client != nil || overrides.Path != nil || overrides.RTSPAuth != nil || overrides.NoAudio != nil || overrides.AudioCodec != nil {
		return Options{}, fmt.Errorf("RTSP and audio flags are not valid for local cameras")
	}

	device := stringValue(overrides.Device, cam.Device, "")
	if device == "" {
		return Options{}, fmt.Errorf("local camera requires --device or a configured device")
	}
	framerate := intValue(overrides.Framerate, defaultFramerate)
	if framerate <= 0 {
		return Options{}, fmt.Errorf("--framerate must be > 0")
	}
	warmup := durationValue(overrides.Warmup, defaultWarmup)
	if warmup <= 0 {
		return Options{}, fmt.Errorf("--warmup must be > 0")
	}

	return Options{
		Kind:      KindLocal,
		Device:    device,
		Framerate: framerate,
		VideoSize: stringValue(overrides.VideoSize, "", ""),
		Warmup:    warmup,
		NoAudio:   true,
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

func intValue(explicit *int, defaultValue int) int {
	if explicit != nil {
		return *explicit
	}
	return defaultValue
}

func durationValue(explicit *time.Duration, defaultValue time.Duration) time.Duration {
	if explicit != nil {
		return *explicit
	}
	return defaultValue
}
