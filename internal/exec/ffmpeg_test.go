package mediaexec

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestClassifyError(t *testing.T) {
	cases := []struct {
		err  string
		want string
	}{
		{"401 Unauthorized", "auth"},
		{"Server returned 401 unauthorized", "auth"},
		{"Operation not permitted", "permission"},
		{"Failed to create AVCaptureDeviceInput", "permission"},
		{"Not Authorized To Capture Video", "permission"},
		{"Connection refused", "network-refused"},
		{"timed out", "network-timeout"},
		{"not found", "not-found"},
		{"/dev/video9: No such file or directory", "not-found"},
		{"weird message", "unknown"},
	}
	for _, c := range cases {
		if got := ClassifyError(c.err); got != c.want {
			t.Fatalf("ClassifyError(%q) got %s want %s", c.err, got, c.want)
		}
	}
}

func TestStderrReadFailureStopsFFmpeg(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture requires a POSIX shell")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\nwhile :; do printf '" + strings.Repeat("x", 1024) + "'; done >&2\n"
	if err := os.WriteFile(filepath.Join(dir, "ffmpeg"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _, err := RunFFmpegWithStderrLines(ctx, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "read ffmpeg logs") {
		t.Fatalf("expected a log read error, got %v", err)
	}
	if ctx.Err() != nil {
		t.Fatal("ffmpeg remained running after its stderr reader failed")
	}
}

func TestWithTimeoutZero(t *testing.T) {
	ctx, cancel := WithTimeout(context.Background(), 0)
	defer cancel()
	select {
	case <-ctx.Done():
		t.Fatalf("zero timeout context should not be canceled immediately")
	default:
	}
}

func TestHasBinary(t *testing.T) {
	if !HasBinary("go") {
		t.Fatalf("expected 'go' to be found in PATH")
	}
	if HasBinary("definitely_missing_binary_xyz") {
		t.Fatalf("expected missing binary to return false")
	}
}

func TestRedactRTSPCredentials(t *testing.T) {
	input := "open rtsp://camera-user:camera-password@192.0.2.10:554/stream1 and RTSPS://token:secret@example.test/live"
	got := redactRTSPCredentials(input)
	for _, secret := range []string{"camera-user", "camera-password", "token", "secret"} {
		if strings.Contains(got, secret) {
			t.Fatalf("redacted output contains %q: %s", secret, got)
		}
	}
	if strings.Count(got, "REDACTED") != 2 {
		t.Fatalf("expected both URLs to be redacted: %s", got)
	}
	malformed := "rtsp://user:password@example.test/%ZZ"
	if got := redactRTSPCredentials(malformed); strings.Contains(got, "password") {
		t.Fatalf("malformed URL leaked credentials: %s", got)
	}

	plain := "rtsp://192.0.2.10:554/stream1"
	if got := redactRTSPCredentials(plain); got != plain {
		t.Fatalf("credential-free URL changed: got %q want %q", got, plain)
	}
}
