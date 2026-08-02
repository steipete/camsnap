// Package cli wires cobra commands for camsnap.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steipete/camsnap/internal/capture"
	"github.com/steipete/camsnap/internal/config"
)

func newAddCmd() *cobra.Command {
	var cam config.Camera

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add or update a camera",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cam.Protocol = strings.ToLower(cam.Protocol)
			cam.LocalBackend = strings.ToLower(cam.LocalBackend)
			if cam.Name == "" {
				return fmt.Errorf("name is required")
			}
			switch cam.Protocol {
			case "local":
				if cam.Device == "" {
					return fmt.Errorf("--device is required for protocol local")
				}
				for _, flag := range []string{"host", "port", "user", "pass", "path", "rtsp-transport", "stream", "rtsp-client", "no-audio", "audio-codec"} {
					if cmd.Flags().Changed(flag) {
						return fmt.Errorf("--%s is not valid for protocol local", flag)
					}
				}
				cam.Port = 0
				if cam.LocalBackend != "" && cam.LocalBackend != capture.LocalBackendNative && cam.LocalBackend != capture.LocalBackendFFmpeg {
					return fmt.Errorf("invalid --local-backend (use native|ffmpeg)")
				}
			case "rtsp", "rtsps":
				if cam.Host == "" {
					return fmt.Errorf("name and host are required")
				}
				if cmd.Flags().Changed("device") {
					return fmt.Errorf("--device is only valid for protocol local")
				}
				if cmd.Flags().Changed("local-backend") {
					return fmt.Errorf("--local-backend is only valid for protocol local")
				}
				if cam.Port == 0 {
					cam.Port = 554
				}
			default:
				return fmt.Errorf("invalid --protocol (use rtsp|rtsps|local)")
			}

			cfgFlag, err := configPathFlag(cmd)
			if err != nil {
				return err
			}
			cfg, path, err := loadConfig(cfgFlag)
			if err != nil {
				return err
			}
			cfg, created := config.UpsertCamera(cfg, cam)
			if err := saveConfig(path, cfg); err != nil {
				return err
			}
			if created {
				cmd.Printf("Added camera %q\n", cam.Name)
			} else {
				cmd.Printf("Updated camera %q\n", cam.Name)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&cam.Name, "name", "", "Camera name (unique)")
	cmd.Flags().StringVar(&cam.Host, "host", "", "Camera host or IP")
	cmd.Flags().IntVar(&cam.Port, "port", 554, "Camera port (default 554)")
	cmd.Flags().StringVar(&cam.Protocol, "protocol", "rtsp", "Protocol (rtsp, rtsps, or local)")
	cmd.Flags().StringVar(&cam.Username, "user", "", "Camera username")
	cmd.Flags().StringVar(&cam.Password, "pass", "", "Camera password")
	cmd.Flags().StringVar(&cam.Device, "device", "", "Local video device index, name, or /dev/videoN path")
	cmd.Flags().StringVar(&cam.LocalBackend, "local-backend", "", "Local snapshot backend (native|ffmpeg)")
	cmd.Flags().StringVar(&cam.Path, "path", "", "Explicit RTSP path (e.g., /Bfy... token from UniFi Protect)")
	cmd.Flags().StringVar(&cam.RTSPTransport, "rtsp-transport", "", "Preferred RTSP transport for this camera (tcp|udp)")
	cmd.Flags().StringVar(&cam.Stream, "stream", "", "Default RTSP stream path (stream1 or stream2)")
	cmd.Flags().StringVar(&cam.RTSPClient, "rtsp-client", "", "Default RTSP client (ffmpeg|gortsplib)")
	cmd.Flags().BoolVar(&cam.NoAudio, "no-audio", false, "Default: drop audio for this camera")
	cmd.Flags().StringVar(&cam.AudioCodec, "audio-codec", "", "Default audio codec when recording (e.g., aac)")

	return cmd
}
