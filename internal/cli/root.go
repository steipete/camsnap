package cli

import (
	"fmt"
	"strings"

	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
)

// NewRootCommand wires the CLI tree.
func NewRootCommand(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "camsnap",
		Short:         "camsnap captures frames and clips from RTSP cameras and local webcams",
		Long:          colorizeLong(),
		SilenceErrors: true,
		SilenceUsage:  true,
		Example:       exampleText(),
	}

	cmd.Version = version
	cmd.SetVersionTemplate("{{.Version}}\n")

	cmd.PersistentFlags().String("config", "", "Path to config file (default: $XDG_CONFIG_HOME/camsnap/config.yaml)")

	cmd.AddCommand(
		newAddCmd(),
		newListCmd(),
		newSnapCmd(),
		newClipCmd(),
		newDiscoverCmd(),
		newDevicesCmd(),
		newWatchCmd(),
		newDoctorCmd(),
		newVersionCmd(version),
	)

	return cmd
}

func configPathFlag(cmd *cobra.Command) (string, error) {
	path, err := cmd.Flags().GetString("config")
	if err != nil {
		return "", fmt.Errorf("read config flag: %w", err)
	}
	return path, nil
}

func exampleText() string {
	var b strings.Builder
	b.WriteString("  camsnap add --name kitchen --host 192.168.0.175 --user tapo --pass secret --rtsp-transport udp --stream stream2\n")
	b.WriteString("  camsnap snap kitchen --out shot.jpg\n")
	b.WriteString("  camsnap snap --device 0 --out webcam.jpg\n")
	b.WriteString("  camsnap devices\n")
	b.WriteString("  camsnap clip kitchen --dur 5s --no-audio --out clip.mp4\n")
	b.WriteString("  camsnap watch kitchen --threshold 0.2 --cooldown 5s --json --action 'touch /tmp/motion'\n")
	b.WriteString("  camsnap doctor --probe --rtsp-transport udp\n")
	return b.String()
}

func colorizeLong() string {
	p := termenv.ColorProfile()
	g := termenv.String().Foreground(p.Color("#4caf50")).Styled
	b := termenv.String().Foreground(p.Color("#00acc1")).Styled
	r := termenv.String().Foreground(p.Color("#e53935")).Styled

	return fmt.Sprintf("%s %s\n\n%s\n  %s\n  %s\n  %s\n",
		b("camsnap"), "– capture frames/clips and motion from RTSP/ONVIF cameras and local webcams.",
		b("Common commands:"),
		g("snap")+"     grab a frame (positional camera name allowed)",
		g("clip")+"     short clip; drop audio with --no-audio",
		r("doctor")+"   checks ffmpeg, reachability, optional RTSP probe",
	)
}
