package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/steipete/camsnap/internal/config"
)

func TestAddAndList(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfgPath := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "camsnap", "config.yaml")

	root := NewRootCommand("test")
	root.SetArgs([]string{"--config", cfgPath, "add", "--name", "t1", "--host", "1.1.1.1", "--user", "u", "--pass", "p"})
	if err := root.Execute(); err != nil {
		t.Fatalf("add execute: %v", err)
	}

	var buf bytes.Buffer
	root = NewRootCommand("test")
	root.SetOut(&buf)
	root.SetArgs([]string{"--config", cfgPath, "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("list execute: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("t1")) {
		t.Fatalf("expected camera name in list output, got: %s", buf.String())
	}
}

func TestSnapNoFFmpeg(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfgPath := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "camsnap", "config.yaml")
	// write config with one camera
	cfg := config.Config{
		Cameras: []config.Camera{{
			Name:          "cam",
			Host:          "127.0.0.1",
			Port:          554,
			Protocol:      "rtsp",
			Username:      "u",
			Password:      "p",
			RTSPTransport: "udp",
			Stream:        "stream1",
		}},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	// empty PATH to ensure ffmpeg not found
	t.Setenv("PATH", "")
	root := NewRootCommand("test")
	root.SetArgs([]string{"--config", cfgPath, "snap", "cam", "--out", filepath.Join(t.TempDir(), "snap.jpg")})
	if err := root.Execute(); err == nil {
		t.Fatalf("expected error due to missing ffmpeg")
	}
}

func TestSnapCreatesTempFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfgPath := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "camsnap", "config.yaml")
	cfg := config.Config{
		Cameras: []config.Camera{{
			Name:     "cam",
			Host:     "127.0.0.1",
			Port:     554,
			Protocol: "rtsp",
			Username: "u",
			Password: "p",
		}},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	ffmpegPath := makeStubFFmpeg(t)
	t.Setenv("PATH", ffmpegPath)

	root := NewRootCommand("test")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"--config", cfgPath, "snap", "cam"})
	if err := root.Execute(); err != nil {
		t.Fatalf("snap: %v", err)
	}
	out := buf.String()
	path := extractTempPath(t, out)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected temp snap file to exist: %v", err)
	}
}

func TestSnapCustomPathIsNotDuplicated(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfgPath := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "camsnap", "config.yaml")
	cfg := config.Config{
		Cameras: []config.Camera{{
			Name:     "cam",
			Host:     "127.0.0.1",
			Port:     554,
			Protocol: "rtsp",
			Path:     "/av_stream/ch0",
		}},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dir := t.TempDir()
	argsPath := filepath.Join(dir, "ffmpeg.args")
	script := filepath.Join(dir, "ffmpeg")
	content := []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" >\"" + argsPath + "\"\nout=\"\"\nfor last in \"$@\"; do out=\"$last\"; done\n: >\"$out\"\nexit 0\n")
	if err := os.WriteFile(script, content, 0o755); err != nil {
		t.Fatalf("write stub ffmpeg: %v", err)
	}
	t.Setenv("PATH", dir)

	root := NewRootCommand("test")
	root.SetArgs([]string{"--config", cfgPath, "snap", "cam", "--out", filepath.Join(t.TempDir(), "snap.jpg")})
	if err := root.Execute(); err != nil {
		t.Fatalf("snap: %v", err)
	}

	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read ffmpeg args: %v", err)
	}
	got := string(args)
	if !strings.Contains(got, "rtsp://127.0.0.1:554/av_stream/ch0") {
		t.Fatalf("missing custom path in ffmpeg args: %s", got)
	}
	if strings.Contains(got, "/av_stream/av_stream/ch0") {
		t.Fatalf("custom path was duplicated in ffmpeg args: %s", got)
	}
}

func TestSnapResolvesCameraDefaultsAndExplicitFlags(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfgPath := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "camsnap", "config.yaml")
	cfg := config.Config{Cameras: []config.Camera{{
		Name: "cam", Host: "127.0.0.1", RTSPTransport: "udp", Stream: "stream2",
	}}}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	ffmpegPath, argsPath := makeRecordingStubFFmpeg(t)
	t.Setenv("PATH", ffmpegPath)
	output := filepath.Join(t.TempDir(), "snap.jpg")

	root := NewRootCommand("test")
	root.SetArgs([]string{"--config", cfgPath, "snap", "cam", "--out", output})
	if err := root.Execute(); err != nil {
		t.Fatalf("snap with camera defaults: %v", err)
	}
	args := readRecordedArgs(t, argsPath)
	assertArgsContainSequence(t, args, "-rtsp_transport", "udp")
	assertArgsContainSequence(t, args, "-i", "rtsp://127.0.0.1:554/stream2")

	root = NewRootCommand("test")
	root.SetArgs([]string{
		"--config", cfgPath, "snap", "cam", "--out", output,
		"--rtsp-transport", "tcp", "--stream", "stream1",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("snap with explicit flags: %v", err)
	}
	args = readRecordedArgs(t, argsPath)
	assertArgsContainSequence(t, args, "-rtsp_transport", "tcp")
	assertArgsContainSequence(t, args, "-i", "rtsp://127.0.0.1:554/stream1")
}

func TestRootVersionHelp(t *testing.T) {
	root := NewRootCommand("test-version")
	root.SetArgs([]string{"--version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("--version: %v", err)
	}

	root = NewRootCommand("test-version")
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("--help: %v", err)
	}
}

func TestDoctorNoCameras(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := NewRootCommand("test")
	root.SetArgs([]string{"doctor"})
	if err := root.Execute(); err != nil {
		t.Fatalf("doctor: %v", err)
	}
}

func TestDiscoverNoDevices(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := NewRootCommand("test")
	root.SetArgs([]string{"discover", "--timeout", "10ms"})
	if err := root.Execute(); err != nil && !errors.Is(err, syscall.ENETUNREACH) && !errors.Is(err, syscall.EHOSTUNREACH) {
		t.Fatalf("discover: %v", err)
	}
}

func TestWatchMissingAction(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfgPath := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "camsnap", "config.yaml")
	cfg := config.Config{
		Cameras: []config.Camera{{
			Name:     "cam",
			Host:     "127.0.0.1",
			Username: "u",
			Password: "p",
		}},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	root := NewRootCommand("test")
	root.SetArgs([]string{"--config", cfgPath, "watch", "cam"})
	if err := root.Execute(); err == nil {
		t.Fatalf("expected error for missing action")
	}
}

func TestClipTempOutput(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfgPath := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "camsnap", "config.yaml")
	cfg := config.Config{
		Cameras: []config.Camera{{
			Name:          "cam",
			Host:          "127.0.0.1",
			Port:          554,
			Protocol:      "rtsp",
			Username:      "u",
			Password:      "p",
			RTSPTransport: "tcp",
		}},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	ffmpegPath := makeStubFFmpeg(t)
	t.Setenv("PATH", ffmpegPath)

	root := NewRootCommand("test")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"--config", cfgPath, "clip", "cam", "--dur", "1s", "--timeout", "2s"})

	if err := root.Execute(); err != nil {
		t.Fatalf("clip: %v", err)
	}
	path := extractTempPath(t, buf.String())
	if info, err := os.Stat(path); err != nil {
		t.Fatalf("expected temp clip file to exist: %v", err)
	} else if info.Size() != 0 {
		// stub ffmpeg writes empty file; size zero is expected. Any size is fine but should exist.
		_ = info
	}
}

func TestClipResolvesAudioDefaultsAndExplicitFlags(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfgPath := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "camsnap", "config.yaml")
	cfg := config.Config{Cameras: []config.Camera{{
		Name: "cam", Host: "127.0.0.1", NoAudio: true, AudioCodec: "pcm_alaw",
	}}}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	ffmpegPath, argsPath := makeRecordingStubFFmpeg(t)
	t.Setenv("PATH", ffmpegPath)
	output := filepath.Join(t.TempDir(), "clip.mp4")

	root := NewRootCommand("test")
	root.SetArgs([]string{"--config", cfgPath, "clip", "cam", "--dur", "1s", "--out", output})
	if err := root.Execute(); err != nil {
		t.Fatalf("clip with camera defaults: %v", err)
	}
	args := readRecordedArgs(t, argsPath)
	assertArgsContainSequence(t, args, "-an")

	root = NewRootCommand("test")
	root.SetArgs([]string{
		"--config", cfgPath, "clip", "cam", "--dur", "1s", "--out", output,
		"--no-audio=false", "--audio-codec", "aac",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("clip with explicit flags: %v", err)
	}
	args = readRecordedArgs(t, argsPath)
	assertArgsContainSequence(t, args, "-c:a", "aac")
	for _, arg := range args {
		if arg == "-an" {
			t.Fatalf("explicit --no-audio=false did not override camera default: %v", args)
		}
	}
}

func makeStubFFmpeg(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "ffmpeg")
	content := []byte("#!/bin/sh\nout=\"\"\nfor last in \"$@\"; do\n\tout=\"$last\"\ndone\n: >\"$out\"\nexit 0\n")
	if err := os.WriteFile(script, content, 0o755); err != nil {
		t.Fatalf("write stub ffmpeg: %v", err)
	}
	return dir
}

func makeRecordingStubFFmpeg(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "ffmpeg.args")
	script := filepath.Join(dir, "ffmpeg")
	content := []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" >\"" + argsPath + "\"\nout=\"\"\nfor last in \"$@\"; do out=\"$last\"; done\n: >\"$out\"\nexit 0\n")
	if err := os.WriteFile(script, content, 0o755); err != nil {
		t.Fatalf("write stub ffmpeg: %v", err)
	}
	return dir, argsPath
}

func readRecordedArgs(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ffmpeg args: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func assertArgsContainSequence(t *testing.T, args []string, sequence ...string) {
	t.Helper()
	for index := 0; index+len(sequence) <= len(args); index++ {
		matches := true
		for offset := range sequence {
			if args[index+offset] != sequence[offset] {
				matches = false
				break
			}
		}
		if matches {
			return
		}
	}
	t.Fatalf("ffmpeg args %v do not contain sequence %v", args, sequence)
}

func extractTempPath(t *testing.T, output string) string {
	t.Helper()
	lines := bytes.Split([]byte(output), []byte("\n"))
	for _, l := range lines {
		if bytes.Contains(l, []byte("writing")) {
			parts := bytes.Fields(l)
			if len(parts) > 0 {
				// path is last token
				return string(parts[len(parts)-1])
			}
		}
	}
	t.Fatalf("could not extract temp path from output: %s", output)
	return ""
}
