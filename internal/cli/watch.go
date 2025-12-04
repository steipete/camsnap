package cli

import (
	"bufio"
	"context"
	"fmt"
	osexec "os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	iexec "github.com/steipete/camsnap/internal/exec"
	"github.com/steipete/camsnap/internal/rtsp"
)

func newWatchCmd() *cobra.Command {
	var cameraName string
	var action string
	var threshold float64
	var cooldown time.Duration
	var runtime time.Duration
	var jsonOutput bool
	var tmpl string
	var authMode string
	var transport string
	var stream string
	var path string

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Run motion detection and execute an action",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cameraName == "" && len(args) > 0 {
				cameraName = args[0]
			}
			if cameraName == "" {
				return fmt.Errorf("--camera is required")
			}
			if action == "" {
				return fmt.Errorf("--action is required (e.g., \"say motion\" or \"touch /tmp/motion\")")
			}
			if threshold <= 0 || threshold >= 1 {
				return fmt.Errorf("--threshold must be between 0 and 1 (e.g., 0.2)")
			}
			if !iexec.HasBinary("ffmpeg") {
				return fmt.Errorf("ffmpeg not found in PATH")
			}
			if _, ok := parseRTSPAuth(authMode); !ok {
				return fmt.Errorf("invalid --rtsp-auth (use auto|basic|digest)")
			}
			xport, ok := transportFlag(transport)
			if !ok {
				return fmt.Errorf("invalid --rtsp-transport (use tcp|udp)")
			}

			cfgFlag, err := configPathFlag(cmd)
			if err != nil {
				return err
			}
			cfg, _, err := loadConfig(cfgFlag)
			if err != nil {
				return err
			}
			cam, ok := findCamera(cfg, cameraName)
			if !ok {
				return fmt.Errorf("camera %q not found", cameraName)
			}
			if stream != "" && path != "" {
				return fmt.Errorf("use --path for custom RTSP token URLs; omit --stream")
			}
			if path == "" && cam.Path != "" {
				path = cam.Path
			}
			if path != "" {
				cam.Path = path
				cam.Stream = ""
			}
			url, err := rtsp.BuildURL(cam)
			if err != nil {
				return err
			}

			if tmpl != "" {
				action, err = applyTemplate(tmpl, cam.Name, 0, time.Now())
				if err != nil {
					return err
				}
			}

			ctx := context.Background()
			if runtime > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, runtime)
				defer cancel()
			}

			return watchMotion(ctx, cameraName, url, threshold, cooldown, action, tmpl, jsonOutput, xport, stream, path, cmd)
		},
	}

	cmd.Flags().StringVar(&cameraName, "camera", "", "Camera name to monitor")
	cmd.Flags().StringVar(&action, "action", "", "Command to execute when motion detected")
	cmd.Flags().Float64Var(&threshold, "threshold", 0.2, "Scene change threshold (0-1, higher = less sensitive)")
	cmd.Flags().DurationVar(&cooldown, "cooldown", 5*time.Second, "Cooldown between triggering actions")
	cmd.Flags().DurationVar(&runtime, "duration", 0, "Optional max runtime (0 = until interrupted)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Log motion events as JSON lines")
	cmd.Flags().StringVar(&tmpl, "action-template", "", "Optional template to build action command (placeholders: {camera},{score},{time})")
	cmd.Flags().StringVar(&authMode, "rtsp-auth", "auto", "RTSP auth mode: auto|basic|digest")
	cmd.Flags().StringVar(&transport, "rtsp-transport", "tcp", "RTSP transport: tcp|udp")
	cmd.Flags().StringVar(&stream, "stream", "", "RTSP path segment (stream1 or stream2); ignored if --path is set")
	cmd.Flags().StringVar(&path, "path", "", "Custom RTSP path (overrides --stream), e.g., /Bfy... from UniFi Protect")

	return cmd
}

func watchMotion(ctx context.Context, cameraName, url string, threshold float64, cooldown time.Duration, action string, tmpl string, jsonOutput bool, transport string, stream string, path string, cmd *cobra.Command) error {
	ffArgs := []string{
		"-hide_banner",
		"-loglevel", "info",
		"-rtsp_transport", transport,
	}
	if path != "" {
		url = appendPath(url, path)
	} else {
		url = appendStream(url, stream)
	}
	ffArgs = append(ffArgs,
		"-i", url,
		"-an",
		"-sn",
		"-dn",
		"-vf", fmt.Sprintf("select='gt(scene\\,%0.3f)',metadata=print", threshold),
		"-f", "null",
		"-",
	)

	ff := osexec.CommandContext(ctx, "ffmpeg", ffArgs...)
	stderr, err := ff.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := ff.Start(); err != nil {
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	lastTrigger := time.Time{}
	var logBuf []string
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		// keep last ~20 lines for error classification
		logBuf = append(logBuf, line)
		if len(logBuf) > 20 {
			logBuf = logBuf[1:]
		}
		if score, ok := parseSceneScore(line); ok {
			now := time.Now()
			if lastTrigger.IsZero() || now.Sub(lastTrigger) >= cooldown {
				lastTrigger = now
				if jsonOutput {
					cmd.Printf(`{"event":"motion","camera":"%s","score":%.3f,"time":"%s"}\n`, cameraName, score, now.Format(time.RFC3339Nano))
				} else {
					cmd.Printf("event=motion camera=%s score=%.3f action=%q time=%s\n", cameraName, score, action, now.Format(time.RFC3339Nano))
				}
				act := action
				if tmpl != "" {
					if rendered, err := applyTemplate(tmpl, cameraName, score, now); err == nil {
						act = rendered
					}
				}
				runAction(ctx, act, score, now, cameraName)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read ffmpeg logs: %w", err)
	}

	if err := ff.Wait(); err != nil && ctx.Err() == nil {
		class := iexec.ClassifyError(strings.Join(logBuf, "\n"))
		return fmt.Errorf("ffmpeg exited: %w (%s)", err, class)
	}
	return nil
}

func parseSceneScore(line string) (float64, bool) {
	// looks like: "[Parsed_metadata_1 ...] scene_score=0.123"
	if !strings.Contains(line, "scene_score=") {
		return 0, false
	}
	idx := strings.Index(line, "scene_score=")
	if idx < 0 || idx+12 >= len(line) {
		return 0, false
	}
	part := line[idx+12:]
	// trim trailing text
	for i, r := range part {
		if !(r == '.' || r == '-' || (r >= '0' && r <= '9')) {
			part = part[:i]
			break
		}
	}
	val, err := strconv.ParseFloat(part, 64)
	if err != nil {
		return 0, false
	}
	return val, true
}

func runAction(ctx context.Context, action string, score float64, t time.Time, camera string) {
	// best-effort: fire and forget, with context env
	cmd := osexec.CommandContext(ctx, "sh", "-c", action)
	cmd.Env = append(cmd.Env,
		"CAMSNAP_SCORE="+fmt.Sprintf("%.3f", score),
		"CAMSNAP_TIME="+t.Format(time.RFC3339Nano),
		"CAMSNAP_CAMERA="+camera,
	)
	_ = cmd.Start()
}

func applyTemplate(tmpl, camera string, score float64, t time.Time) (string, error) {
	repl := map[string]string{
		"{camera}": camera,
		"{score}":  fmt.Sprintf("%.3f", score),
		"{time}":   t.Format(time.RFC3339Nano),
	}
	out := tmpl
	for k, v := range repl {
		out = strings.ReplaceAll(out, k, v)
	}
	return out, nil
}
