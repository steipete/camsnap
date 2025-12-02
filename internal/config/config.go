package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Camera represents a single camera entry stored on disk.
type Camera struct {
	Name          string `yaml:"name"`
	Host          string `yaml:"host"`
	Port          int    `yaml:"port"`
	Protocol      string `yaml:"protocol"`
	Username      string `yaml:"username"`
	Password      string `yaml:"password"`
	RTSPTransport string `yaml:"rtsp_transport,omitempty"` // tcp|udp
	Stream        string `yaml:"stream,omitempty"`         // stream1|stream2
	RTSPClient    string `yaml:"rtsp_client,omitempty"`    // ffmpeg|gortsplib
	NoAudio       bool   `yaml:"no_audio,omitempty"`
	AudioCodec    string `yaml:"audio_codec,omitempty"` // e.g., aac
}

// Config is the root configuration struct.
type Config struct {
	Cameras []Camera `yaml:"cameras"`
}

// DefaultConfigPath returns the OS-specific config file path.
func DefaultConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(dir, "camsnap", "config.yaml"), nil
}

// Load reads a config file; returns empty config if the file is absent.
func Load(path string) (Config, error) {
	cfg := Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// Save writes the config to disk, creating parent directories as needed.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// UpsertCamera inserts or updates a camera by name. Returns the updated config and true if new.
func UpsertCamera(cfg Config, cam Camera) (Config, bool) {
	for i, existing := range cfg.Cameras {
		if existing.Name == cam.Name {
			cfg.Cameras[i] = cam
			return cfg, false
		}
	}
	cfg.Cameras = append(cfg.Cameras, cam)
	return cfg, true
}

// FindCamera returns a camera by name.
func FindCamera(cfg Config, name string) (Camera, bool) {
	for _, cam := range cfg.Cameras {
		if cam.Name == name {
			return cam, true
		}
	}
	return Camera{}, false
}
