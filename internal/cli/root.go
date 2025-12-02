package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewRootCommand wires the CLI tree.
func NewRootCommand(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "camsnap",
		Short:         "camsnap captures frames and clips from RTSP cameras (Tapo first, Ubiquiti next)",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.PersistentFlags().String("config", "", "Path to config file (default: $XDG_CONFIG_HOME/camsnap/config.yaml)")

	cmd.AddCommand(
		newAddCmd(),
		newListCmd(),
		newSnapCmd(),
		newClipCmd(),
		newDiscoverCmd(),
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
