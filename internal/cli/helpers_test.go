package cli

import (
	"testing"

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
