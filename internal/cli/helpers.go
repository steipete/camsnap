package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/steipete/camsnap/internal/capture"
	"github.com/steipete/camsnap/internal/config"
)

func loadConfig(pathFlag string) (config.Config, string, error) {
	var path string
	var err error
	if pathFlag != "" {
		path = pathFlag
	} else {
		path, err = config.DefaultConfigPath()
		if err != nil {
			return config.Config{}, "", err
		}
	}
	cfg, err := config.Load(path)
	if err != nil {
		return config.Config{}, "", err
	}
	return cfg, path, nil
}

func saveConfig(path string, cfg config.Config) error {
	if path == "" {
		return fmt.Errorf("config path missing")
	}
	return config.Save(path, cfg)
}

func findCamera(cfg config.Config, name string) (config.Camera, bool) {
	return config.FindCamera(cfg, name)
}

type captureFlagValues struct {
	transport  string
	stream     string
	client     string
	path       string
	noAudio    bool
	audioCodec string
}

func (values captureFlagValues) overrides(cmd *cobra.Command) capture.Overrides {
	return capture.Overrides{
		Transport:  changedString(cmd, "rtsp-transport", values.transport),
		Stream:     changedString(cmd, "stream", values.stream),
		Client:     changedString(cmd, "rtsp-client", values.client),
		Path:       changedString(cmd, "path", values.path),
		NoAudio:    changedBool(cmd, "no-audio", values.noAudio),
		AudioCodec: changedString(cmd, "audio-codec", values.audioCodec),
	}
}

func changedString(cmd *cobra.Command, name, value string) *string {
	if cmd.Flags().Changed(name) {
		return &value
	}
	return nil
}

func changedBool(cmd *cobra.Command, name string, value bool) *bool {
	if cmd.Flags().Changed(name) {
		return &value
	}
	return nil
}

// loadConfigFromFlag reads the persistent config flag off a command and loads the config.
func loadConfigFromFlag(cmd *cobra.Command) (config.Config, string, error) {
	cfgFlag, err := configPathFlag(cmd)
	if err != nil {
		return config.Config{}, "", err
	}
	cfg, path, err := loadConfig(cfgFlag)
	if err != nil {
		return config.Config{}, "", err
	}
	return cfg, path, nil
}
