package cli

import (
	"testing"

	"github.com/steipete/camsnap/internal/config"
	"github.com/steipete/camsnap/internal/rtsp"
)

func TestHostOnly(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"ipv4 with port", "192.168.0.177:80", "192.168.0.177"},
		{"ipv6 bracketed with port", "[fe80::20c:29ff:fe37:3729]:80", "fe80::20c:29ff:fe37:3729"},
		{"zone-qualified ipv6 bracketed with port", "[fe80::1%en0]:80", "fe80::1%en0"},
		{"no port falls back verbatim", "192.168.0.177", "192.168.0.177"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hostOnly(tc.in); got != tc.want {
				t.Errorf("hostOnly(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSafeName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"ipv4 has no colons", "192.168.0.177", "192.168.0.177"},
		{"ipv6 internal colons become dashes", "fe80::20c:29ff:fe37:3729", "fe80--20c-29ff-fe37-3729"},
		{"zone-qualified ipv6 keeps zone", "fe80::1%en0", "fe80--1%en0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := safeName(tc.in); got != tc.want {
				t.Errorf("safeName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestDiscoverSuggestedCommandIsAddable is a regression test for the bug
// this file fixes: before the fix, the suggested `add` command for an
// ONVIF-discovered device embedded the discovery (ONVIF/SOAP) port into
// --host verbatim, which `add` stores as a literal, malformed host string
// (its own --port flag, defaulting to RTSP's 554, is a separate field) --
// and for bracketed IPv6 addresses, the generated --name was truncated at
// the address's first internal colon (e.g. "cam-[fe80"), an invalid,
// unusable name. Both symptoms trace to the same root cause: treating the
// discovery Device.Host ("host:port" from the ONVIF XAddr) as if it were
// already the bare host `add` expects.
func TestDiscoverSuggestedCommandIsAddable(t *testing.T) {
	cases := []struct {
		name         string
		deviceHost   string // discovery.Device.Host: "host:port" from XAddr
		wantHost     string // what should go after --host in the suggestion
		wantNameTail string // what should go after "cam-" in --name
	}{
		{
			name:         "ipv4 onvif device",
			deviceHost:   "192.168.0.177:80",
			wantHost:     "192.168.0.177",
			wantNameTail: "192.168.0.177",
		},
		{
			name:         "ipv6 link-local onvif device",
			deviceHost:   "[fe80::20c:29ff:fe37:3729]:80",
			wantHost:     "fe80::20c:29ff:fe37:3729",
			wantNameTail: "fe80--20c-29ff-fe37-3729",
		},
		{
			name:         "zone-qualified ipv6 link-local onvif device",
			deviceHost:   "[fe80::1%en0]:80",
			wantHost:     "fe80::1%en0",
			wantNameTail: "fe80--1%en0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			host := hostOnly(tc.deviceHost)
			if host != tc.wantHost {
				t.Errorf("hostOnly(%q) = %q, want %q", tc.deviceHost, host, tc.wantHost)
			}
			nameTail := safeName(host)
			if nameTail != tc.wantNameTail {
				t.Errorf("safeName(hostOnly(%q)) = %q, want %q", tc.deviceHost, nameTail, tc.wantNameTail)
			}
		})
	}
}

// TestDiscoverScopedIPv6HostBuildsRTSPURL is the discover → add → BuildURL
// path for a zone-qualified link-local XAddr. hostOnly strips the ONVIF
// port and leaves "fe80::1%en0"; BuildURL must JoinHostPort that host and
// emit the RFC 6874 zone ("%25") so the URL is parseable.
func TestDiscoverScopedIPv6HostBuildsRTSPURL(t *testing.T) {
	deviceHost := "[fe80::1%en0]:80"
	host := hostOnly(deviceHost)
	got, err := rtsp.BuildURL(config.Camera{Host: host})
	if err != nil {
		t.Fatalf("BuildURL: %v", err)
	}
	want := "rtsp://[fe80::1%25en0]:554/stream1"
	if got != want {
		t.Fatalf("hostOnly(%q) then BuildURL = %q, want %q", deviceHost, got, want)
	}
}
