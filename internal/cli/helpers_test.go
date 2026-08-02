package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/steipete/camsnap/internal/config"
	"github.com/steipete/camsnap/internal/rtsp"
)

func TestAppendStream(t *testing.T) {
	base := "rtsp://user:pass@192.168.0.10:554/stream1"

	got := rtsp.ReplacePath(base, "stream2")
	want := "rtsp://user:pass@192.168.0.10:554/stream2"
	if got != want {
		t.Fatalf("appendStream stream2: got %s want %s", got, want)
	}

	got = rtsp.ReplacePath(base, "/stream2")
	if got != want {
		t.Fatalf("appendStream /stream2: got %s want %s", got, want)
	}

	got = rtsp.ReplacePath(base, "")
	if got != base {
		t.Fatalf("appendStream empty: got %s want %s", got, base)
	}
}

func TestParseRTSPAuth(t *testing.T) {
	cases := []struct {
		in     string
		ok     bool
		expect string
	}{
		{"auto", true, ""},
		{"basic", true, "basic"},
		{"digest", true, "digest"},
		{"", true, ""},
		{"weird", false, ""},
	}
	for _, c := range cases {
		got, ok := parseRTSPAuth(c.in)
		if ok != c.ok {
			t.Fatalf("parseRTSPAuth(%s) ok=%v want %v", c.in, ok, c.ok)
		}
		if got != c.expect {
			t.Fatalf("parseRTSPAuth(%s) got %s want %s", c.in, got, c.expect)
		}
	}
}

func TestAppendPath(t *testing.T) {
	base := "rtsp://192.168.1.1:7447/stream1"
	got := rtsp.ReplacePath(base, "/Bfy47")
	want := "rtsp://192.168.1.1:7447/Bfy47"
	if got != want {
		t.Fatalf("appendPath absolute: got %s want %s", got, want)
	}
	got = rtsp.ReplacePath(base, "Bfy47")
	if got != want {
		t.Fatalf("appendPath no slash: got %s want %s", got, want)
	}
	got = rtsp.ReplacePath(base, "")
	if got != base {
		t.Fatalf("appendPath empty: got %s want %s", got, base)
	}
}

func TestCustomPathOverrideIsNotDuplicated(t *testing.T) {
	cam := config.Camera{
		Name:     "custom",
		Host:     "192.168.1.10",
		Port:     554,
		Protocol: "rtsp",
		Path:     "/av_stream/ch0",
	}

	got, err := rtsp.BuildURL(cam)
	if err != nil {
		t.Fatalf("BuildURL: %v", err)
	}
	want := "rtsp://192.168.1.10:554/av_stream/ch0"
	if got != want {
		t.Fatalf("custom path duplicated: got %s want %s", got, want)
	}
}

func TestSelectCaptureCameraUsesNativeDefault(t *testing.T) {
	previous := nativeDefaultCamera
	nativeDefaultCamera = func() (localDevice, bool, error) {
		return localDevice{ID: "default-id", Name: "Default Camera", IsDefault: true}, true, nil
	}
	defer func() { nativeDefaultCamera = previous }()

	cmd := &cobra.Command{}
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cam, selectedName, err := selectCaptureCameraWithDefault(cmd, nil, "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if cam.Protocol != "local" || cam.Device != "default-id" || cam.Name != "Default Camera" {
		t.Fatalf("selected camera = %#v", cam)
	}
	if selectedName != "Default Camera" {
		t.Fatalf("selected name = %q", selectedName)
	}
	if got := stderr.String(); !strings.Contains(got, `No camera specified; using default camera "Default Camera"`) {
		t.Fatalf("stderr = %q", got)
	}
}

func TestSelectCaptureCameraKeepsDeviceRequiredWithoutDefault(t *testing.T) {
	previous := nativeDefaultCamera
	nativeDefaultCamera = func() (localDevice, bool, error) { return localDevice{}, false, nil }
	defer func() { nativeDefaultCamera = previous }()

	cmd := &cobra.Command{}
	for _, useNativeDefault := range []bool{false, true} {
		if _, _, err := selectCaptureCameraWithDefault(cmd, nil, "", "", useNativeDefault); err == nil || err.Error() != "--camera or --device is required" {
			t.Fatalf("useNativeDefault=%t error = %v", useNativeDefault, err)
		}
	}
}

func TestSelectCaptureCameraPropagatesEnumerationError(t *testing.T) {
	previous := nativeDefaultCamera
	nativeDefaultCamera = func() (localDevice, bool, error) {
		return localDevice{}, false, fmt.Errorf("enumerate native cameras: bridge failure")
	}
	defer func() { nativeDefaultCamera = previous }()

	cmd := &cobra.Command{}
	_, _, err := selectCaptureCameraWithDefault(cmd, nil, "", "", true)
	if err == nil || !strings.Contains(err.Error(), "enumerate native cameras") {
		t.Fatalf("error = %v", err)
	}
}
