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
	var device string
	var framerate int
	var videoSize string
	var localBackend string

	cmd := &cobra.Command{
		Use:   "clip",
		Short: "Record a short clip to a file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if duration <= 0 {
				return fmt.Errorf("--dur must be > 0")
			}
			cam, selectedName, err := selectCaptureCamera(cmd, args, cameraName, device)
			if err != nil {
				return err
			}
			cameraName = selectedName
			options, err := resolveCaptureOptions(cam, (captureFlagValues{
				transport:    transport,
				stream:       stream,
				path:         path,
				rtspAuth:     authMode,
				noAudio:      noAudio,
				audioCodec:   audioCodec,
				device:       device,
				framerate:    framerate,
				videoSize:    videoSize,
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
			if !mediaexec.HasBinary("ffmpeg") {
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

			ctx, cancel := mediaexec.WithTimeout(context.Background(), timeout)
			defer cancel()
			if options.Kind == capture.KindLocal {
				return runLocalCapture(ctx, localCaptureRequest{operation: localClip, options: options, output: outPath, duration: duration, notice: cmd.PrintErrln})
			}

			ffArgs, err := capture.ClipArgs(options, duration, outPath, runtime.GOOS)
			if err != nil {
				return err
			}
			return mediaexec.RunFFmpeg(ctx, ffArgs...)
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
	cmd.Flags().StringVar(&device, "device", "", "Local video device index, name, or /dev/videoN path")
	cmd.Flags().IntVar(&framerate, "framerate", 30, "Local capture framerate")
	cmd.Flags().StringVar(&videoSize, "video-size", "", "Local capture size (e.g., 1280x720)")
	cmd.Flags().StringVar(&localBackend, "local-backend", "", "Local snapshot backend (native|ffmpeg; clips always use ffmpeg)")

	return cmd
}
