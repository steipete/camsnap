package cli

import "testing"

func TestAppendStream(t *testing.T) {
	base := "rtsp://user:pass@192.168.0.10:554/stream1"

	got := appendStream(base, "stream2")
	want := "rtsp://user:pass@192.168.0.10:554/stream2"
	if got != want {
		t.Fatalf("appendStream stream2: got %s want %s", got, want)
	}

	got = appendStream(base, "/stream2")
	if got != want {
		t.Fatalf("appendStream /stream2: got %s want %s", got, want)
	}

	got = appendStream(base, "")
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

func TestTransportFlag(t *testing.T) {
	cases := []struct {
		in     string
		ok     bool
		expect string
	}{
		{"", true, "tcp"},
		{"tcp", true, "tcp"},
		{"udp", true, "udp"},
		{"something", false, ""},
	}
	for _, c := range cases {
		got, ok := transportFlag(c.in)
		if ok != c.ok || got != c.expect {
			t.Fatalf("transportFlag(%s) got (%s,%v) want (%s,%v)", c.in, got, ok, c.expect, c.ok)
		}
	}
}

func TestAppendPath(t *testing.T) {
	base := "rtsp://192.168.1.1:7447/stream1"
	got := appendPath(base, "/Bfy47")
	want := "rtsp://192.168.1.1:7447/Bfy47"
	if got != want {
		t.Fatalf("appendPath absolute: got %s want %s", got, want)
	}
	got = appendPath(base, "Bfy47")
	if got != want {
		t.Fatalf("appendPath no slash: got %s want %s", got, want)
	}
	got = appendPath(base, "")
	if got != base {
		t.Fatalf("appendPath empty: got %s want %s", got, base)
	}
}
