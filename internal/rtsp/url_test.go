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
