package mediaexec

import (
	"context"
	"strings"
	"testing"
)

func TestClassifyError(t *testing.T) {
	cases := []struct {
		err  string
		want string
	}{
		{"401 Unauthorized", "auth"},
		{"Server returned 401 unauthorized", "auth"},
		{"Connection refused", "network-refused"},
		{"timed out", "network-timeout"},
		{"not found", "not-found"},
		{"weird message", "unknown"},
	}
	for _, c := range cases {
		if got := ClassifyError(c.err); got != c.want {
			t.Fatalf("ClassifyError(%q) got %s want %s", c.err, got, c.want)
		}
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
