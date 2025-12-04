package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/camsnap/internal/exec"
	"github.com/steipete/camsnap/internal/rtsp"
)

func newClipCmd() *cobra.Command {
	var cameraName string
	var outPath string
	var duration time.Duration
	var timeout time.Duration
	var authMode string
	var transport string
	var stream string
	var noAudio bool
	var audioCodec string
	var path string

	cmd := &cobra.Command{
		Use:   "clip",
		Short: "Record a short clip to a file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cameraName == "" && len(args) > 0 {
				cameraName = args[0]
			}
			if cameraName == "" {
				return fmt.Errorf("--camera is required")
			}
			if duration <= 0 {
				return fmt.Errorf("--dur must be > 0")
			}
			if !exec.HasBinary("ffmpeg") {
				return fmt.Errorf("ffmpeg not found in PATH")
			}
			if outPath == "" {
				tmp, err := os.CreateTemp("", "camsnap-*.mp4")
				if err != nil {
					return fmt.Errorf("create temp file: %w", err)
				}
				if err := tmp.Close(); err != nil {
					return fmt.Errorf("close temp file: %w", err)
				}
				outPath = tmp.Name()
				cmd.Printf("No --out provided, writing clip to %s\n", outPath)
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

			// per-camera defaults
			if transport == "" && cam.RTSPTransport != "" {
				transport = cam.RTSPTransport
			}
			if stream == "" && cam.Stream != "" && path == "" {
				stream = cam.Stream
			}
			if !noAudio && cam.NoAudio {
				noAudio = true
			}
			if audioCodec == "" && cam.AudioCodec != "" {
				audioCodec = cam.AudioCodec
			}

			if _, ok := parseRTSPAuth(authMode); !ok {
				return fmt.Errorf("invalid --rtsp-auth (use auto|basic|digest)")
			}
			xport, ok := transportFlag(transport)
			if !ok {
				return fmt.Errorf("invalid --rtsp-transport (use tcp|udp)")
			}
			url, err := rtsp.BuildURL(cam)
			if err != nil {
				return err
			}

			ctx, cancel := exec.WithTimeout(context.Background(), timeout)
			defer cancel()

			if path != "" {
				url = appendPath(url, path)
			} else {
				url = appendStream(url, stream)
			}

			ffArgs := []string{
				"-y",
				"-rtsp_transport", xport,
				"-i", url,
				"-t", fmt.Sprintf("%.0f", duration.Seconds()),
			}
			// Video: copy
			ffArgs = append(ffArgs, "-c:v", "copy")
			if noAudio {
				ffArgs = append(ffArgs, "-an")
			} else {
				if audioCodec == "" {
					// safe default for mp4
					ffArgs = append(ffArgs, "-c:a", "aac")
				} else {
					ffArgs = append(ffArgs, "-c:a", audioCodec)
				}
			}
			ffArgs = append(ffArgs, outPath)
			return exec.RunFFmpeg(ctx, ffArgs...)
		},
	}

	cmd.Flags().StringVar(&cameraName, "camera", "", "Camera name to use")
	cmd.Flags().StringVar(&outPath, "out", "", "Output file (e.g., clip.mp4)")
	cmd.Flags().DurationVar(&duration, "dur", 10*time.Second, "Clip duration (e.g., 10s)")
	cmd.Flags().DurationVar(&timeout, "timeout", 20*time.Second, "Timeout for ffmpeg invocation")
	cmd.Flags().StringVar(&authMode, "rtsp-auth", "auto", "RTSP auth mode: auto|basic|digest")
	cmd.Flags().StringVar(&transport, "rtsp-transport", "tcp", "RTSP transport: tcp|udp")
	cmd.Flags().StringVar(&stream, "stream", "", "RTSP path segment (stream1 or stream2); ignored if --path is set")
	cmd.Flags().StringVar(&path, "path", "", "Custom RTSP path (overrides --stream), e.g., /Bfy... from UniFi Protect")
	cmd.Flags().BoolVar(&noAudio, "no-audio", false, "Drop audio track")
	cmd.Flags().StringVar(&audioCodec, "audio-codec", "", "Audio codec (default aac); ignored if --no-audio")

	return cmd
}
