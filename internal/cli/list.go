package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List saved cameras",
		RunE: func(cmd *cobra.Command, args []string) error {
			sty := newStyler(cmd.OutOrStdout())
			cfgFlag, err := configPathFlag(cmd)
			if err != nil {
				return err
			}
			cfg, _, err := loadConfig(cfgFlag)
			if err != nil {
				return err
			}
			if len(cfg.Cameras) == 0 {
				cmd.Println(sty.Warn("No cameras saved. Add one with: camsnap add --name cam1 --host 192.168.1.50 --user tapo --pass secret"))
				return nil
			}
			// deterministic order
			sort.Slice(cfg.Cameras, func(i, j int) bool { return cfg.Cameras[i].Name < cfg.Cameras[j].Name })

			for _, cam := range cfg.Cameras {
				// avoid printing password
				auth := cam.Username
				if auth != "" && cam.Password != "" {
					auth += ":***"
				}
				cmd.Printf("%-12s host=%s port=%d proto=%s auth=%s\n",
					cam.Name, cam.Host, cam.Port, strings.ToLower(cam.Protocol), auth)
			}
			return nil
		},
	}
	return cmd
}
