package cli

import (
	"fmt"

	"github.com/spf13/cobra"
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

// appendStream swaps the path to the desired stream if provided.
func appendStream(baseURL, stream string) string {
	if stream == "" {
		return baseURL
	}
	if stream[0] != '/' {
		stream = "/" + stream
	}
	// replace last path segment
	for i := len(baseURL) - 1; i >= 0; i-- {
		if baseURL[i] == '/' {
			return baseURL[:i] + stream
		}
	}
	return baseURL + stream
}

// appendPath replaces the trailing path of an RTSP URL with a custom path.
func appendPath(baseURL, path string) string {
	if path == "" {
		return baseURL
	}
	if path[0] != '/' {
		path = "/" + path
	}
	for i := len(baseURL) - 1; i >= 0; i-- {
		if baseURL[i] == '/' {
			return baseURL[:i] + path
		}
	}
	return baseURL + path
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
