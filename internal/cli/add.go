// Package cli wires cobra commands for camsnap.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/steipete/camsnap/internal/config"
)

func newAddCmd() *cobra.Command {
	var cam config.Camera

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add or update a camera",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cam.Name == "" || cam.Host == "" {
				return fmt.Errorf("name and host are required")
			}
			if cam.Port == 0 {
				cam.Port = 554
			}
			if cam.Protocol == "" {
				cam.Protocol = "rtsp"
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
	cmd.Flags().StringVar(&cam.Protocol, "protocol", "rtsp", "Protocol (rtsp or rtsps)")
	cmd.Flags().StringVar(&cam.Username, "user", "", "Camera username")
	cmd.Flags().StringVar(&cam.Password, "pass", "", "Camera password")
	cmd.Flags().StringVar(&cam.Path, "path", "", "Explicit RTSP path (e.g., /Bfy... token from UniFi Protect)")
	cmd.Flags().StringVar(&cam.RTSPTransport, "rtsp-transport", "", "Preferred RTSP transport for this camera (tcp|udp)")
	cmd.Flags().StringVar(&cam.Stream, "stream", "", "Default RTSP stream path (stream1 or stream2)")
	cmd.Flags().StringVar(&cam.RTSPClient, "rtsp-client", "", "Default RTSP client (ffmpeg|gortsplib)")
	cmd.Flags().BoolVar(&cam.NoAudio, "no-audio", false, "Default: drop audio for this camera")
	cmd.Flags().StringVar(&cam.AudioCodec, "audio-codec", "", "Default audio codec when recording (e.g., aac)")

	return cmd
}
