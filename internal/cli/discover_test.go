package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/steipete/camsnap/internal/config"
	"github.com/steipete/camsnap/internal/rtsp"
)

func TestDiscoverSuggestedHostCanBeSaved(t *testing.T) {
	cases := []struct {
		name, deviceHost, wantHost, wantName, wantURL string
	}{
		{"ipv4 with ONVIF port", "192.0.2.1:80", "192.0.2.1", "cam-192.0.2.1", "rtsp://192.0.2.1:554/stream1"},
		{"ipv4 without port", "192.0.2.1", "192.0.2.1", "cam-192.0.2.1", "rtsp://192.0.2.1:554/stream1"},
		{"hostname", "camera.example:8080", "camera.example", "cam-camera.example", "rtsp://camera.example:554/stream1"},
		{"ipv6 with ONVIF port", "[2001:db8::1]:80", "2001:db8::1", "cam-2001-db8--1", "rtsp://[2001:db8::1]:554/stream1"},
		{"ipv6 without port", "[2001:db8::1]", "2001:db8::1", "cam-2001-db8--1", "rtsp://[2001:db8::1]:554/stream1"},
		{"scoped ipv6", "[fe80::1%en0]:80", "fe80::1%en0", "cam-fe80--1%en0", "rtsp://[fe80::1%25en0]:554/stream1"},
		{"scoped ipv6 without port", "[fe80::1%en0]", "fe80::1%en0", "cam-fe80--1%en0", "rtsp://[fe80::1%25en0]:554/stream1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			host := hostOnly(tc.deviceHost)
			name := "cam-" + safeName(host)
			if host != tc.wantHost || name != tc.wantName {
				t.Fatalf("suggestion host/name = %q/%q, want %q/%q", host, name, tc.wantHost, tc.wantName)
			}
			cfgPath := filepath.Join(t.TempDir(), "config.yaml")
			cmd := NewRootCommand("test")
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetArgs([]string{"--config", cfgPath, "add", "--name", name, "--host", host})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				t.Fatal(err)
			}
			cam, found := config.FindCamera(cfg, name)
			if !found || cam.Host != tc.wantHost || cam.Port != 554 {
				t.Fatalf("saved camera = %+v, found = %v", cam, found)
			}
			got, err := rtsp.BuildURL(cam)
			if err != nil || got != tc.wantURL {
				t.Fatalf("BuildURL = %q, %v; want %q", got, err, tc.wantURL)
			}
		})
	}
}
