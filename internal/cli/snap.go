package cli

import (
	"context"
	"fmt"
	"os"
	"runtime"
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
	var device string
	var framerate int
	var videoSize string
	var warmup time.Duration
	var localBackend string

	cmd := &cobra.Command{
		Use:   "snap",
		Short: "Capture a single frame to a file",
		Long:  "Capture a single frame to a file. In native macOS builds, omitting both the camera name and --device uses the default local camera.",
		RunE: func(cmd *cobra.Command, args []string) error {
			useNativeDefault := localBackend != capture.LocalBackendFFmpeg
			cam, selectedName, err := selectCaptureCameraWithDefault(cmd, args, cameraName, device, useNativeDefault)
			if err != nil {
				return err
			}
			cameraName = selectedName
			options, err := resolveCaptureOptions(cam, (captureFlagValues{
				transport:    transport,
				stream:       stream,
				client:       client,
				path:         path,
				rtspAuth:     authMode,
				device:       device,
				framerate:    framerate,
				videoSize:    videoSize,
				warmup:       warmup,
				localBackend: localBackend,
			}).overrides(cmd))
			if err != nil {
				return err
			}
			if options.Kind == capture.KindRTSP {
				if _, ok := parseRTSPAuth(authMode); !ok {
					return fmt.Errorf("invalid --rtsp-auth (use auto|basic|digest)")
				}
			}
			if (options.Kind == capture.KindRTSP || options.LocalBackend == capture.LocalBackendFFmpeg) && !mediaexec.HasBinary("ffmpeg") {
				return fmt.Errorf("ffmpeg not found in PATH")
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

			ctx, cancel := mediaexec.WithTimeout(context.Background(), timeout)
			defer cancel()
			if options.Kind == capture.KindLocal {
				return runLocalCapture(ctx, localCaptureRequest{operation: localSnap, options: options, output: outPath, notice: cmd.PrintErrln})
			}

			if options.Client == "gortsplib" {
				return rtspclient.GrabFrameViaGort(ctx, options.URL, options.Transport, outPath, timeout)
			}

			ffArgs, err := capture.SnapArgs(options, outPath, runtime.GOOS)
			if err != nil {
				return err
			}
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
	cmd.Flags().StringVar(&device, "device", "", "Local video device index, name, or /dev/videoN path")
	cmd.Flags().IntVar(&framerate, "framerate", 30, "Local capture framerate")
	cmd.Flags().StringVar(&videoSize, "video-size", "", "Local capture size (e.g., 1280x720)")
	cmd.Flags().DurationVar(&warmup, "warmup", time.Second, "Local camera auto-exposure warmup")
	cmd.Flags().StringVar(&localBackend, "local-backend", "", "Local snapshot backend (native|ffmpeg)")

	return cmd
}
