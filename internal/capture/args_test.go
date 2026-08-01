package capture

import (
	"reflect"
	"testing"
	"time"
)

func TestSnapArgs(t *testing.T) {
	options := Options{URL: "rtsp://camera.test/stream2", Transport: "udp"}
	want := []string{
		"-y", "-rtsp_transport", "udp", "-i", "rtsp://camera.test/stream2",
		"-frames:v", "1", "-q:v", "2", "shot.jpg",
	}
	if got := SnapArgs(options, "shot.jpg"); !reflect.DeepEqual(got, want) {
		t.Fatalf("SnapArgs = %#v, want %#v", got, want)
	}
}

func TestClipArgs(t *testing.T) {
	tests := []struct {
		name    string
		options Options
		want    []string
	}{
		{
			name:    "audio codec",
			options: Options{URL: "rtsp://camera.test/stream1", Transport: "tcp", AudioCodec: "aac"},
			want: []string{
				"-y", "-rtsp_transport", "tcp", "-i", "rtsp://camera.test/stream1",
				"-t", "5", "-c:v", "copy", "-c:a", "aac", "clip.mp4",
			},
		},
		{
			name:    "no audio",
			options: Options{URL: "rtsp://camera.test/stream1", Transport: "udp", NoAudio: true},
			want: []string{
				"-y", "-rtsp_transport", "udp", "-i", "rtsp://camera.test/stream1",
				"-t", "5", "-c:v", "copy", "-an", "clip.mp4",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClipArgs(tt.options, 5*time.Second, "clip.mp4"); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ClipArgs = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestWatchArgs(t *testing.T) {
	options := Options{URL: "rtsp://camera.test/stream2", Transport: "udp"}
	want := []string{
		"-hide_banner", "-loglevel", "info", "-rtsp_transport", "udp",
		"-i", "rtsp://camera.test/stream2", "-an", "-sn", "-dn",
		"-vf", "select='gt(scene\\,0.200)',metadata=print", "-f", "null", "-",
	}
	if got := WatchArgs(options, 0.2); !reflect.DeepEqual(got, want) {
		t.Fatalf("WatchArgs = %#v, want %#v", got, want)
	}
}

func TestProbeArgs(t *testing.T) {
	options := Options{URL: "rtsp://camera.test/stream1", Transport: "tcp"}
	want := []string{
		"-hide_banner", "-loglevel", "error", "-rtsp_transport", "tcp",
		"-i", "rtsp://camera.test/stream1", "-t", "1", "-f", "null", "-",
	}
	if got := ProbeArgs(options); !reflect.DeepEqual(got, want) {
		t.Fatalf("ProbeArgs = %#v, want %#v", got, want)
	}
}
