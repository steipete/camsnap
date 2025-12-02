package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := Config{
		Cameras: []Camera{
			{
				Name:          "front",
				Host:          "192.168.1.10",
				Port:          554,
				Protocol:      "rtsp",
				Username:      "user",
				Password:      "pass",
				RTSPTransport: "udp",
				Stream:        "stream2",
				RTSPClient:    "gortsplib",
				NoAudio:       true,
				AudioCodec:    "aac",
			},
		},
	}

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded.Cameras) != 1 || loaded.Cameras[0].Name != "front" {
		t.Fatalf("round trip mismatch: %#v", loaded)
	}
	if loaded.Cameras[0].RTSPTransport != "udp" || loaded.Cameras[0].Stream != "stream2" || loaded.Cameras[0].RTSPClient != "gortsplib" || !loaded.Cameras[0].NoAudio || loaded.Cameras[0].AudioCodec != "aac" {
		t.Fatalf("round trip custom fields mismatch: %#v", loaded.Cameras[0])
	}
}

func TestDefaultConfigPathXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdgtest")
	path, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath: %v", err)
	}
	if path != "/tmp/xdgtest/camsnap/config.yaml" {
		t.Fatalf("unexpected path: %s", path)
	}
}

func TestLoadMissingOK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if len(cfg.Cameras) != 0 {
		t.Fatalf("expected empty config, got %#v", cfg)
	}
}

func TestUpsertCamera(t *testing.T) {
	cfg := Config{}
	cam := Camera{Name: "one", Host: "1.1.1.1"}

	cfg, created := UpsertCamera(cfg, cam)
	if !created {
		t.Fatal("expected created true")
	}
	if len(cfg.Cameras) != 1 {
		t.Fatalf("expected 1 camera, got %d", len(cfg.Cameras))
	}

	cam.Host = "2.2.2.2"
	cfg, created = UpsertCamera(cfg, cam)
	if created {
		t.Fatal("expected update not create")
	}
	if cfg.Cameras[0].Host != "2.2.2.2" {
		t.Fatalf("camera not updated: %+v", cfg.Cameras[0])
	}
}

func TestFindCamera(t *testing.T) {
	cfg := Config{
		Cameras: []Camera{{Name: "front"}, {Name: "back"}},
	}
	cam, ok := FindCamera(cfg, "back")
	if !ok || cam.Name != "back" {
		t.Fatalf("expected back camera, got %#v", cam)
	}
	_, ok = FindCamera(cfg, "none")
	if ok {
		t.Fatal("expected not found")
	}
}

// Ensure config file permissions are restrictive.
func TestSavePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")

	if err := Save(path, Config{}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected 0600 perms, got %o", perm)
	}
}
