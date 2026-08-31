package rtsp

import (
	"testing"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/steipete/camsnap/internal/config"
)

func TestBuildURL(t *testing.T) {
	cam := config.Camera{
		Name:     "cam1",
		Host:     "192.168.1.50",
		Port:     554,
		Protocol: "rtsp",
		Username: "user",
		Password: "pass",
	}
	got, err := BuildURL(cam)
	if err != nil {
		t.Fatalf("BuildURL: %v", err)
	}
	want := "rtsp://user:pass@192.168.1.50:554/stream1"
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestBuildURLInvalidProtocol(t *testing.T) {
	cam := config.Camera{
		Name:     "cam3",
		Host:     "10.0.0.3",
		Protocol: "http",
	}
	if _, err := BuildURL(cam); err == nil {
		t.Fatalf("expected error for invalid protocol")
	}
}

func TestBuildURLDefaults(t *testing.T) {
	cam := config.Camera{
		Name: "cam2",
		Host: "10.0.0.2",
	}
	got, err := BuildURL(cam)
	if err != nil {
		t.Fatalf("BuildURL: %v", err)
	}
	want := "rtsp://10.0.0.2:554/stream1"
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestBuildURLWithPath(t *testing.T) {
	cam := config.Camera{
		Name:     "protect",
		Host:     "192.168.1.1",
		Port:     7447,
		Protocol: "rtsp",
		Path:     "/Bfy47SNWz9n2WRrw",
	}
	got, err := BuildURL(cam)
	if err != nil {
		t.Fatalf("BuildURL: %v", err)
	}
	want := "rtsp://192.168.1.1:7447/Bfy47SNWz9n2WRrw"
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestBuildURLPreservesCustomPath(t *testing.T) {
	for _, path := range []string{
		"/cam/realmonitor?channel=1&subtype=0",
		"/token%2Fsegment?value=a%2Bb&empty=",
		"/stream?",
	} {
		t.Run(path, func(t *testing.T) {
			got, err := BuildURL(config.Camera{Host: "::1%lo0", Port: 8554, Path: path})
			want := "rtsp://[::1%25lo0]:8554" + path
			if err != nil || got != want {
				t.Fatalf("BuildURL = %q, %v; want %q", got, err, want)
			}
		})
	}
}

func TestReplacePath(t *testing.T) {
	base := "rtsp://user:pass@192.168.0.10:554/stream1"
	want := "rtsp://user:pass@192.168.0.10:554/stream2"
	for _, path := range []string{"stream2", "/stream2"} {
		if got := ReplacePath(base, path); got != want {
			t.Fatalf("ReplacePath(%q) = %q, want %q", path, got, want)
		}
	}
	if got := ReplacePath(base, ""); got != base {
		t.Fatalf("ReplacePath empty = %q, want %q", got, base)
	}
}

// TestBuildURLBareIPv6 is a regression test: a bare IPv6 literal (as
// internal/cli.hostOnly now suggests for ONVIF-discovered devices) must be
// bracketed and get a port, exactly like a bare IPv4 literal or hostname.
// Before the fix, `strings.Contains(host, ":")` treated any IPv6 literal's
// internal colons as "already has a port" and left it untouched, producing
// an unbracketed, portless, unusable RTSP authority.
func TestBuildURLBareIPv6(t *testing.T) {
	cases := []struct {
		name string
		cam  config.Camera
		want string
	}{
		{
			name: "default port",
			cam:  config.Camera{Host: "fe80::20c:29ff:fe37:3729"},
			want: "rtsp://[fe80::20c:29ff:fe37:3729]:554/stream1",
		},
		{
			name: "configured port",
			cam:  config.Camera{Host: "fe80::1", Port: 8554},
			want: "rtsp://[fe80::1]:8554/stream1",
		},
		{
			name: "bracketed with configured port",
			cam:  config.Camera{Host: "[::1]", Port: 8554},
			want: "rtsp://[::1]:8554/stream1",
		},
		{
			name: "numeric zone 25 is not an encoded delimiter",
			cam:  config.Camera{Host: "fe80::1%25"},
			want: "rtsp://[fe80::1%2525]:554/stream1",
		},
		{
			name: "numeric zone starting with 25",
			cam:  config.Camera{Host: "fe80::1%250", Port: 8554},
			want: "rtsp://[fe80::1%25250]:8554/stream1",
		},
		{
			name: "existing bracketed escaped scope without port",
			cam:  config.Camera{Host: "[fe80::1%25en0]", Port: 8554},
			want: "rtsp://[fe80::1%25en0]:8554/stream1",
		},
		{
			name: "with auth",
			cam:  config.Camera{Host: "fe80::1", Port: 554, Username: "u", Password: "p"},
			want: "rtsp://u:p@[fe80::1]:554/stream1",
		},
		{
			// RFC 6874: the zone delimiter is "%25" in a URI. A raw "%en0"
			// is an invalid percent-escape (gortsplib base.ParseURL rejects it).
			name: "zone-qualified link-local",
			cam:  config.Camera{Host: "fe80::1%en0"},
			want: "rtsp://[fe80::1%25en0]:554/stream1",
		},
		{
			name: "zone-qualified with configured port",
			cam:  config.Camera{Host: "fe80::1%en0", Port: 8554},
			want: "rtsp://[fe80::1%25en0]:8554/stream1",
		},
		{
			name: "zone-qualified with auth",
			cam:  config.Camera{Host: "fe80::1%en0", Username: "u", Password: "p"},
			want: "rtsp://u:p@[fe80::1%25en0]:554/stream1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildURL(tc.cam)
			if err != nil {
				t.Fatalf("BuildURL: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

// TestBuildURLAlreadyCompleteAuthorityUnchanged confirms hosts that already
// carry their own port (IPv4:port or bracketed [IPv6]:port) pass through
// as the authority. A zone identifier is RFC 6874-encoded ("%25") in the
// URI; the port and brackets are otherwise unchanged.
func TestBuildURLAlreadyCompleteAuthorityUnchanged(t *testing.T) {
	cases := []struct {
		name string
		host string
		want string
	}{
		{"ipv4 with port", "192.168.1.50:8080", "rtsp://192.168.1.50:8080/stream1"},
		{"bracketed ipv6 with port", "[fe80::1]:8080", "rtsp://[fe80::1]:8080/stream1"},
		{"bracketed zone-qualified ipv6 with port", "[fe80::1%en0]:8080", "rtsp://[fe80::1%25en0]:8080/stream1"},
		{"existing escaped ipv6 scope with port", "[fe80::1%25en0]:8080", "rtsp://[fe80::1%25en0]:8080/stream1"},
		{"existing escaped numeric scope with port", "[fe80::1%2525]:8080", "rtsp://[fe80::1%2525]:8080/stream1"},
		{"raw numeric scope 25 with port", "[fe80::1%25]:8080", "rtsp://[fe80::1%2525]:8080/stream1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildURL(config.Camera{Host: tc.host})
			if err != nil {
				t.Fatalf("BuildURL: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

// TestBuildURLScopedIPv6AcceptedByGortsplib is the required parser proof:
// a zone-qualified host must serialize as RFC 6874 ("%25") so gortsplib
// base.ParseURL accepts it. The raw JoinHostPort spelling ("%en0") is
// rejected as an invalid percent-escape, and the native client never starts.
func TestBuildURLScopedIPv6AcceptedByGortsplib(t *testing.T) {
	if _, err := base.ParseURL("rtsp://[fe80::1%en0]:554/stream1"); err == nil {
		t.Fatal("gortsplib base.ParseURL unexpectedly accepted an unencoded zone delimiter")
	}

	cases := []config.Camera{
		{Host: "fe80::1%en0"},
		{Host: "fe80::1%en0", Port: 8554},
		{Host: "fe80::1%en0", Username: "u", Password: "p"},
		{Host: "[fe80::1%en0]:8080"},
	}
	for _, cam := range cases {
		got, err := BuildURL(cam)
		if err != nil {
			t.Fatalf("BuildURL(%+v): %v", cam, err)
		}
		if _, err := base.ParseURL(got); err != nil {
			t.Fatalf("gortsplib base.ParseURL(%q): %v", got, err)
		}
	}
}
