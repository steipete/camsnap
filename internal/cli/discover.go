package cli

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/camsnap/internal/config"
	"github.com/steipete/camsnap/internal/discovery"
)

func newDiscoverCmd() *cobra.Command {
	var timeout time.Duration
	var includeInfo bool
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover cameras on the local network via ONVIF WS-Discovery",
		RunE: func(cmd *cobra.Command, args []string) error {
			sty := newStyler(cmd.OutOrStdout())
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			cfg, _, _ := loadConfigFromFlag(cmd)

			devs, err := discovery.Discover(ctx, timeout)
			if err != nil {
				return err
			}
			if len(devs) == 0 {
				cmd.Println(sty.Warn("No devices found. Ensure cameras and this host are on the same LAN."))
				return nil
			}
			for _, d := range devs {
				infoStr := ""
				if includeInfo {
					info := fetchInfo(ctx, cfg, d)
					if info != "" {
						infoStr = " [" + info + "]"
					}
				}
				cmd.Printf("%s\t(add: camsnap add --name cam-%s --host %s --user <user> --pass <pass>)%s\n",
					sty.OK(d.Host), safeName(d.Host), d.Host, infoStr)
			}
			return nil
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", 3*time.Second, "Discovery timeout")
	cmd.Flags().BoolVar(&includeInfo, "info", false, "Attempt ONVIF GetDeviceInformation (may require credentials)")
	return cmd
}

func safeName(host string) string {
	// use host part without port for a short name
	for i, r := range host {
		if r == ':' {
			return host[:i]
		}
	}
	return host
}

func fetchInfo(ctx context.Context, cfg config.Config, d discovery.Device) string {
	// If we already have creds for this host, try them first.
	user, pass := findCreds(cfg, d.Host)
	info, err := discovery.FetchDeviceInfo(ctx, d.Address, user, pass)
	if err != nil {
		return ""
	}
	parts := []string{}
	if info.Model != "" {
		parts = append(parts, info.Model)
	}
	if info.Firmware != "" {
		parts = append(parts, "fw "+info.Firmware)
	}
	if info.Manufacturer != "" {
		parts = append(parts, info.Manufacturer)
	}
	return strings.Join(parts, ", ")
}

func findCreds(cfg config.Config, host string) (string, string) {
	for _, cam := range cfg.Cameras {
		if cam.Host == host || strings.HasPrefix(host, cam.Host+":") {
			return cam.Username, cam.Password
		}
	}
	return "", ""
}
