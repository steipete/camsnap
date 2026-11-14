package cli

import (
	"bytes"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/steipete/camsnap/internal/config"
)

func TestDoctorDialsBuildURLHost(t *testing.T) {
	t.Run("bracketed ipv6", func(t *testing.T) {
		port := listenLocal(t, "tcp", "[::1]:0")
		n, err := strconv.Atoi(port)
		if err != nil {
			t.Fatal(err)
		}
		out := runDoctor(t, config.Camera{Name: "v6", Host: "[::1]", Port: n})
		want := "reachable at [::1]:" + port
		if !strings.Contains(out, want) {
			t.Fatalf("output = %q, want %q", out, want)
		}
	})

	t.Run("host already has port", func(t *testing.T) {
		port := listenLocal(t, "tcp", "127.0.0.1:0")
		out := runDoctor(t, config.Camera{Name: "ported", Host: "127.0.0.1:" + port, Port: 554})
		want := "reachable at 127.0.0.1:" + port
		if !strings.Contains(out, want) {
			t.Fatalf("output = %q, want %q", out, want)
		}
	})

	t.Run("plain host", func(t *testing.T) {
		port := listenLocal(t, "tcp", "127.0.0.1:0")
		n, err := strconv.Atoi(port)
		if err != nil {
			t.Fatal(err)
		}
		out := runDoctor(t, config.Camera{Name: "plain", Host: "127.0.0.1", Port: n})
		want := "reachable at 127.0.0.1:" + port
		if !strings.Contains(out, want) {
			t.Fatalf("output = %q, want %q", out, want)
		}
	})
}

func listenLocal(t *testing.T, network, address string) string {
	t.Helper()
	ln, err := net.Listen(network, address)
	if err != nil {
		t.Skipf("listen %s %s: %v", network, address, err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	return port
}

func runDoctor(t *testing.T, cam config.Camera) string {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := config.Save(cfgPath, config.Config{Cameras: []config.Camera{cam}}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	root := NewRootCommand("test")
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--config", cfgPath, "doctor", "--timeout", "200ms"})
	if err := root.Execute(); err != nil {
		t.Fatalf("doctor: %v\n%s", err, out.String())
	}
	return out.String()
}
