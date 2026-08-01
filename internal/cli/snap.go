package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/camsnap/internal/capture"
	mediaexec "github.com/steipete/camsnap/internal/exec"
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
			if !mediaexec.HasBinary("ffmpeg") {
				return fmt.Errorf("ffmpeg not found in PATH")
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

			if _, ok := parseRTSPAuth(authMode); !ok {
				return fmt.Errorf("invalid --rtsp-auth (use auto|basic|digest)")
			}
			options, err := capture.Resolve(cam, (captureFlagValues{
				transport: transport,
				stream:    stream,
				client:    client,
				path:      path,
			}).overrides(cmd))
			if err != nil {
				return err
			}

			ctx, cancel := mediaexec.WithTimeout(context.Background(), timeout)
			defer cancel()

			if options.Client == "gortsplib" {
				return rtspclient.GrabFrameViaGort(ctx, options.URL, options.Transport, outPath, timeout)
			}

			ffArgs := capture.SnapArgs(options, outPath)
			return mediaexec.RunFFmpeg(ctx, ffArgs...)
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
