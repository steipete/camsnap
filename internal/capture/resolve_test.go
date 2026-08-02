package capture

import (
	"strings"
	"testing"
	"time"

	"github.com/steipete/camsnap/internal/config"
)

func TestResolvePrecedence(t *testing.T) {
	cam := config.Camera{
		Host:          "192.0.2.10",
		RTSPTransport: "udp",
		Stream:        "stream2",
		RTSPClient:    "gortsplib",
		NoAudio:       true,
		AudioCodec:    "opus",
	}
	tcp := "tcp"
	stream := "stream3"
	client := "ffmpeg"
	noAudio := false
	codec := "aac"

	got, err := Resolve(cam, Overrides{
		Transport:  &tcp,
		Stream:     &stream,
		Client:     &client,
		NoAudio:    &noAudio,
		AudioCodec: &codec,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if got.Transport != "tcp" || got.Stream != "stream3" || got.Client != "ffmpeg" || got.NoAudio || got.AudioCodec != "aac" {
		t.Fatalf("explicit values did not win: %#v", got)
	}
	if want := "rtsp://192.0.2.10:554/stream3"; got.URL != want {
		t.Fatalf("URL = %q, want %q", got.URL, want)
	}
}

func TestResolveLocalDefaultsAndOverrides(t *testing.T) {
	framerate := 60
	videoSize := "1920x1080"
	warmup := 2 * time.Second
	got, err := Resolve(config.Camera{Protocol: "local", Device: "0"}, Overrides{
		Framerate: &framerate,
		VideoSize: &videoSize,
		Warmup:    &warmup,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := Options{
		Kind: KindLocal, Device: "0", Framerate: 60, VideoSize: "1920x1080", Warmup: 2 * time.Second, NoAudio: true,
	}
	if got != want {
		t.Fatalf("Resolve = %#v, want %#v", got, want)
	}

	got, err = Resolve(config.Camera{Protocol: "LOCAL", Device: "FaceTime HD Camera"}, Overrides{})
	if err != nil {
		t.Fatalf("Resolve defaults: %v", err)
	}
	if got.Framerate != 30 || got.Warmup != time.Second || got.Device != "FaceTime HD Camera" {
		t.Fatalf("local defaults not applied: %#v", got)
	}
}

func TestResolveLocalValidation(t *testing.T) {
	tcp := "tcp"
	device := "0"
	zero := 0
	zeroDuration := time.Duration(0)
	invalidBackend := "quicktime"
	tests := []struct {
		name      string
		cam       config.Camera
		overrides Overrides
		contains  string
	}{
		{name: "missing device", cam: config.Camera{Protocol: "local"}, contains: "requires --device"},
		{name: "RTSP flag", cam: config.Camera{Protocol: "local", Device: "0"}, overrides: Overrides{Transport: &tcp}, contains: "RTSP and audio flags"},
		{name: "local flag on RTSP", cam: config.Camera{Host: "192.0.2.1"}, overrides: Overrides{Device: &device}, contains: "only valid for local cameras"},
		{name: "zero framerate", cam: config.Camera{Protocol: "local", Device: "0"}, overrides: Overrides{Framerate: &zero}, contains: "--framerate must be > 0"},
		{name: "zero warmup", cam: config.Camera{Protocol: "local", Device: "0"}, overrides: Overrides{Warmup: &zeroDuration}, contains: "--warmup must be > 0"},
		{name: "invalid backend", cam: config.Camera{Protocol: "local", Device: "0"}, overrides: Overrides{LocalBackend: &invalidBackend}, contains: "invalid --local-backend"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Resolve(tt.cam, tt.overrides)
			if err == nil || !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("Resolve error = %v, want substring %q", err, tt.contains)
			}
		})
	}
}

func TestResolveLocalBackendPrecedence(t *testing.T) {
	ffmpeg := LocalBackendFFmpeg
	got, err := Resolve(config.Camera{Protocol: "local", Device: "0", LocalBackend: LocalBackendNative}, Overrides{LocalBackend: &ffmpeg})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.LocalBackend != LocalBackendFFmpeg {
		t.Fatalf("LocalBackend = %q, want %q", got.LocalBackend, LocalBackendFFmpeg)
	}
}

func TestResolveCameraAndApplicationDefaults(t *testing.T) {
	tests := []struct {
		name string
		cam  config.Camera
		want Options
	}{
		{
			name: "application defaults",
			cam:  config.Camera{Host: "192.0.2.10"},
			want: Options{
				URL: "rtsp://192.0.2.10:554/stream1", Transport: "tcp", Client: "ffmpeg", AudioCodec: "aac",
			},
		},
		{
			name: "camera defaults",
			cam: config.Camera{
				Host: "192.0.2.10", RTSPTransport: "udp", Stream: "stream2", RTSPClient: "gortsplib", NoAudio: true, AudioCodec: "pcm_alaw",
			},
			want: Options{
				URL: "rtsp://192.0.2.10:554/stream2", Transport: "udp", Stream: "stream2", Client: "gortsplib", NoAudio: true, AudioCodec: "pcm_alaw",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.cam, Overrides{})
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Resolve = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestResolvePathOverridesStream(t *testing.T) {
	stream := "stream3"
	cam := config.Camera{Host: "192.0.2.10", Path: "/token", Stream: "stream2"}

	got, err := Resolve(cam, Overrides{Stream: &stream})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Path != "/token" || got.Stream != "" || got.URL != "rtsp://192.0.2.10:554/token" {
		t.Fatalf("camera path did not override stream: %#v", got)
	}
}

func TestResolveExplicitEmptyPathClearsCameraPath(t *testing.T) {
	empty := ""
	cam := config.Camera{Host: "192.0.2.10", Path: "/token", Stream: "stream2"}

	got, err := Resolve(cam, Overrides{Path: &empty})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Path != "" || got.Stream != "stream2" || got.URL != "rtsp://192.0.2.10:554/stream2" {
		t.Fatalf("explicit empty path did not clear camera path: %#v", got)
	}
}

func TestResolveRejectsExplicitStreamAndPath(t *testing.T) {
	stream := "stream2"
	path := "/token"
	_, err := Resolve(config.Camera{Host: "192.0.2.10"}, Overrides{Stream: &stream, Path: &path})
	if err == nil || err.Error() != "use --path for custom RTSP token URLs; omit --stream" {
		t.Fatalf("Resolve error = %v", err)
	}
}

func TestResolveRejectsInvalidTransport(t *testing.T) {
	transport := "http"
	_, err := Resolve(config.Camera{Host: "192.0.2.10"}, Overrides{Transport: &transport})
	if err == nil || err.Error() != "invalid --rtsp-transport (use tcp|udp)" {
		t.Fatalf("Resolve error = %v", err)
	}
}

func TestResolveTransportValues(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{"", "tcp"},
		{"tcp", "tcp"},
		{"TCP", "tcp"},
		{"udp", "udp"},
		{"UDP", "udp"},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			value := tt.value
			got, err := Resolve(config.Camera{Host: "192.0.2.10"}, Overrides{Transport: &value})
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got.Transport != tt.want {
				t.Fatalf("Transport = %q, want %q", got.Transport, tt.want)
			}
		})
	}
}
