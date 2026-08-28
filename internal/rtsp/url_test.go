package rtsp

import (
	"testing"

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
			name: "with auth",
			cam:  config.Camera{Host: "fe80::1", Port: 554, Username: "u", Password: "p"},
			want: "rtsp://u:p@[fe80::1]:554/stream1",
		},
		{
			// net.ParseIP rejects zone IDs, so this used to skip JoinHostPort
			// and emit rtsp://fe80::1%en0/stream1 (unbracketed, portless).
			name: "zone-qualified link-local",
			cam:  config.Camera{Host: "fe80::1%en0"},
			want: "rtsp://[fe80::1%en0]:554/stream1",
		},
		{
			name: "zone-qualified with configured port",
			cam:  config.Camera{Host: "fe80::1%en0", Port: 8554},
			want: "rtsp://[fe80::1%en0]:8554/stream1",
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

// TestBuildURLAlreadyCompleteAuthorityUnchanged confirms the fix does not
// disturb hosts that already carry their own port (IPv4:port or the
// bracketed [IPv6]:port form, including a zone identifier) -- those must
// pass through untouched, same as before this fix.
func TestBuildURLAlreadyCompleteAuthorityUnchanged(t *testing.T) {
	cases := []struct {
		name string
		host string
	}{
		{"ipv4 with port", "192.168.1.50:8080"},
		{"bracketed ipv6 with port", "[fe80::1]:8080"},
		{"bracketed zone-qualified ipv6 with port", "[fe80::1%en0]:8080"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildURL(config.Camera{Host: tc.host})
			if err != nil {
				t.Fatalf("BuildURL: %v", err)
			}
			want := "rtsp://" + tc.host + "/stream1"
			if got != want {
				t.Fatalf("got %s, want %s", got, want)
			}
		})
	}
}
