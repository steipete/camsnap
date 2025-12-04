package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/camsnap/internal/exec"
	"github.com/steipete/camsnap/internal/rtsp"
	"github.com/steipete/camsnap/internal/rtspclient"
)

func newSnapCmd() *cobra.Command {
	var cameraName string
	var outPath string
	var timeout time.Duration
	var authMode string
	var transport string
	var stream string
	var client string
	var path string

	cmd := &cobra.Command{
		Use:   "snap",
		Short: "Capture a single frame to a file",
		RunE: func(cmd *cobra.Command, args []string) error {
			// allow positional camera name if --camera not set
			if cameraName == "" && len(args) > 0 {
				cameraName = args[0]
			}
			if cameraName == "" {
				return fmt.Errorf("--camera is required")
			}
			if outPath == "" {
				tmp, err := os.CreateTemp("", "camsnap-*.jpg")
				if err != nil {
					return fmt.Errorf("create temp file: %w", err)
				}
				if err := tmp.Close(); err != nil {
					return fmt.Errorf("close temp file: %w", err)
				}
				outPath = tmp.Name()
				cmd.Printf("No --out provided, writing snapshot to %s\n", outPath)
			}
			if !exec.HasBinary("ffmpeg") {
				return fmt.Errorf("ffmpeg not found in PATH")
			}

			if stream != "" && path != "" {
				return fmt.Errorf("use --path for custom RTSP token URLs; omit --stream")
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

			// fall back to per-camera defaults
			if transport == "" && cam.RTSPTransport != "" {
				transport = cam.RTSPTransport
			}
			if stream == "" && cam.Stream != "" && path == "" {
				stream = cam.Stream
			}
			if client == "" && cam.RTSPClient != "" {
				client = cam.RTSPClient
			}

			if _, ok := parseRTSPAuth(authMode); !ok {
				return fmt.Errorf("invalid --rtsp-auth (use auto|basic|digest)")
			}
			xport, ok := transportFlag(transport)
			if !ok {
				return fmt.Errorf("invalid --rtsp-transport (use tcp|udp)")
			}

			ctx, cancel := exec.WithTimeout(context.Background(), timeout)
			defer cancel()

			if path != "" {
				url = appendPath(url, path)
			} else {
				url = appendStream(url, stream)
			}

			if client == "gortsplib" {
				return rtspclient.GrabFrameViaGort(ctx, url, xport, outPath, timeout)
			}

			ffArgs := []string{
				"-y",
				"-rtsp_transport", xport,
				"-i", url,
				"-frames:v", "1",
				"-q:v", "2",
				outPath,
			}
			return exec.RunFFmpeg(ctx, ffArgs...)
		},
	}

	cmd.Flags().StringVar(&cameraName, "camera", "", "Camera name to use")
	cmd.Flags().StringVar(&outPath, "out", "", "Output file (e.g., snap.jpg)")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Second, "Timeout for ffmpeg invocation")
	cmd.Flags().StringVar(&authMode, "rtsp-auth", "auto", "RTSP auth mode: auto|basic|digest")
	cmd.Flags().StringVar(&transport, "rtsp-transport", "tcp", "RTSP transport: tcp|udp")
	cmd.Flags().StringVar(&stream, "stream", "", "RTSP path segment (stream1 or stream2); ignored if --path is set")
	cmd.Flags().StringVar(&path, "path", "", "Custom RTSP path (overrides --stream), e.g., /Bfy... from UniFi Protect")
	cmd.Flags().StringVar(&client, "rtsp-client", "ffmpeg", "RTSP client: ffmpeg|gortsplib")

	return cmd
}
