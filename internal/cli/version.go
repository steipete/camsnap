package cli

import "github.com/spf13/cobra"

func newVersionCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show camsnap version",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println(version)
		},
	}
	return cmd
}
