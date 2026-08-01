package capture

import (
	"testing"

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
