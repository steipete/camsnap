//go:build !windows

package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/steipete/camsnap/internal/config"
)

func TestActionInheritsEnvironmentAndReapsProcess(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "event")
	script := "#!/bin/sh\nprintf '%s\\n' \"$CAMSNAP_TEST_PARENT\" \"$CAMSNAP_CAMERA\" \"$CAMSNAP_SCORE\" \"$CAMSNAP_TIME\" \"$$\" > \"$CAMSNAP_TEST_OUTPUT\"\n"
	if err := os.WriteFile(filepath.Join(dir, "camsnap-test-action"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CAMSNAP_TEST_PARENT", "inherited")
	t.Setenv("CAMSNAP_TEST_OUTPUT", output)
	t.Setenv("CAMSNAP_CAMERA", "stale")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	now := time.Now()
	if err := runAction(ctx, "exec camsnap-test-action", 0.321, now, "front door"); err != nil {
		t.Fatal(err)
	}
	for {
		data, err := os.ReadFile(output)
		if err == nil && strings.Count(string(data), "\n") == 5 {
			fields := strings.Split(strings.TrimSpace(string(data)), "\n")
			want := "inherited\nfront door\n0.321\n" + now.Format(time.RFC3339Nano) + "\n"
			if !strings.HasPrefix(string(data), want) {
				t.Fatalf("action environment = %q, want prefix %q", data, want)
			}
			pid, err := strconv.Atoi(fields[4])
			if err != nil {
				t.Fatal(err)
			}
			if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal("action did not inherit its environment or was not reaped")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestWatchHonorsCanceledCommandContext(t *testing.T) {
	dir := t.TempDir()
	started := filepath.Join(dir, "started")
	t.Setenv("CAMSNAP_TEST_STARTED", started)
	if err := os.WriteFile(filepath.Join(dir, "ffmpeg"), []byte("#!/bin/sh\n: > \"$CAMSNAP_TEST_STARTED\"\nexec sleep 5\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	cfg := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfg, config.Config{Cameras: []config.Camera{{Name: "cam", Host: "127.0.0.1"}}}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	root := NewRootCommand("test")
	root.SetArgs([]string{"--config", cfg, "watch", "cam", "--action", ":", "--duration", "100ms"})
	_ = root.ExecuteContext(ctx)
	if _, err := os.Stat(started); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled watch started ffmpeg: %v", err)
	}
}

func TestActionReportsStartFailure(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := runAction(context.Background(), ":", 0.321, time.Now(), "cam"); err == nil {
		t.Fatal("missing shell should report a start error")
	}
}
