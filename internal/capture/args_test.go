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
	got, err := SnapArgs(options, "shot.jpg", "darwin")
	if err != nil {
		t.Fatalf("SnapArgs: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
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
			got, err := ClipArgs(tt.options, 5*time.Second, "clip.mp4", "darwin")
			if err != nil {
				t.Fatalf("ClipArgs: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
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
	got, err := WatchArgs(options, 0.2, "darwin")
	if err != nil {
		t.Fatalf("WatchArgs: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WatchArgs = %#v, want %#v", got, want)
	}
}

func TestProbeArgs(t *testing.T) {
	options := Options{URL: "rtsp://camera.test/stream1", Transport: "tcp"}
	want := []string{
		"-hide_banner", "-loglevel", "error", "-rtsp_transport", "tcp",
		"-i", "rtsp://camera.test/stream1", "-t", "1", "-f", "null", "-",
	}
	got, err := ProbeArgs(options, "darwin")
	if err != nil {
		t.Fatalf("ProbeArgs: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ProbeArgs = %#v, want %#v", got, want)
	}
}

func TestLocalInputArgs(t *testing.T) {
	options := Options{Kind: KindLocal, Device: "2", Framerate: 24, VideoSize: "1280x720"}
	tests := []struct {
		goos string
		want []string
	}{
		{
			goos: "darwin",
			want: []string{"-f", "avfoundation", "-framerate", "24", "-video_size", "1280x720", "-i", "2"},
		},
		{
			goos: "linux",
			want: []string{"-f", "v4l2", "-framerate", "24", "-video_size", "1280x720", "-i", "2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			got, err := InputArgs(options, tt.goos)
			if err != nil {
				t.Fatalf("InputArgs: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("InputArgs = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestLocalCommandArgs(t *testing.T) {
	options := Options{Kind: KindLocal, Device: "0", Framerate: 30, VideoSize: "1920x1080", Warmup: 1500 * time.Millisecond}

	snap, err := SnapArgs(options, "shot.jpg", "darwin")
	if err != nil {
		t.Fatalf("SnapArgs: %v", err)
	}
	wantSnap := []string{
		"-y", "-f", "avfoundation", "-framerate", "30", "-video_size", "1920x1080", "-i", "0",
		"-t", "1.5", "-update", "1", "-q:v", "2", "shot.jpg",
	}
	if !reflect.DeepEqual(snap, wantSnap) {
		t.Fatalf("SnapArgs = %#v, want %#v", snap, wantSnap)
	}

	clip, err := ClipArgs(options, 5*time.Second, "clip.mp4", "linux")
	if err != nil {
		t.Fatalf("ClipArgs: %v", err)
	}
	wantClip := []string{
		"-y", "-f", "v4l2", "-framerate", "30", "-video_size", "1920x1080", "-i", "0", "-t", "5",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", "veryfast", "-movflags", "+faststart", "-an", "clip.mp4",
	}
	if !reflect.DeepEqual(clip, wantClip) {
		t.Fatalf("ClipArgs = %#v, want %#v", clip, wantClip)
	}

	watch, err := WatchArgs(options, 0.2, "darwin")
	if err != nil {
		t.Fatalf("WatchArgs: %v", err)
	}
	wantWatch := []string{
		"-hide_banner", "-loglevel", "info", "-f", "avfoundation", "-framerate", "30", "-video_size", "1920x1080", "-i", "0",
		"-an", "-sn", "-dn", "-vf", "select='gt(scene\\,0.200)',metadata=print", "-f", "null", "-",
	}
	if !reflect.DeepEqual(watch, wantWatch) {
		t.Fatalf("WatchArgs = %#v, want %#v", watch, wantWatch)
	}
}

func TestInputArgsRejectsUnsupportedOS(t *testing.T) {
	_, err := InputArgs(Options{Kind: KindLocal, Device: "camera", Framerate: 30}, "windows")
	if err == nil || err.Error() != "local webcam capture is unsupported on windows" {
		t.Fatalf("InputArgs error = %v", err)
	}
}
