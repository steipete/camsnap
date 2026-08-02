package cli

import (
	"fmt"
	"time"

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
	transport    string
	stream       string
	client       string
	path         string
	rtspAuth     string
	noAudio      bool
	audioCodec   string
	device       string
	framerate    int
	videoSize    string
	warmup       time.Duration
	localBackend string
}

func (values captureFlagValues) overrides(cmd *cobra.Command) capture.Overrides {
	return capture.Overrides{
		Transport:    changedString(cmd, "rtsp-transport", values.transport),
		Stream:       changedString(cmd, "stream", values.stream),
		Client:       changedString(cmd, "rtsp-client", values.client),
		Path:         changedString(cmd, "path", values.path),
		RTSPAuth:     changedString(cmd, "rtsp-auth", values.rtspAuth),
		NoAudio:      changedBool(cmd, "no-audio", values.noAudio),
		AudioCodec:   changedString(cmd, "audio-codec", values.audioCodec),
		Device:       changedString(cmd, "device", values.device),
		Framerate:    changedInt(cmd, "framerate", values.framerate),
		VideoSize:    changedString(cmd, "video-size", values.videoSize),
		Warmup:       changedDuration(cmd, "warmup", values.warmup),
		LocalBackend: changedString(cmd, "local-backend", values.localBackend),
	}
}

func resolveCaptureOptions(cam config.Camera, overrides capture.Overrides) (capture.Options, error) {
	options, err := capture.Resolve(cam, overrides)
	if err != nil {
		return capture.Options{}, err
	}
	if options.Kind == capture.KindLocal && options.LocalBackend == "" {
		options.LocalBackend = defaultLocalBackend()
	}
	if options.Kind == capture.KindLocal && options.LocalBackend == capture.LocalBackendNative && !nativeLocalBackendAvailable() {
		return capture.Options{}, fmt.Errorf("native local capture backend is not available in this build; use --local-backend ffmpeg")
	}
	return options, nil
}

func changedInt(cmd *cobra.Command, name string, value int) *int {
	if cmd.Flags().Changed(name) {
		return &value
	}
	return nil
}

func changedDuration(cmd *cobra.Command, name string, value time.Duration) *time.Duration {
	if cmd.Flags().Changed(name) {
		return &value
	}
	return nil
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

func selectCaptureCamera(cmd *cobra.Command, args []string, cameraName, device string) (config.Camera, string, error) {
	if cameraName == "" && len(args) > 0 {
		cameraName = args[0]
	}
	if cameraName != "" && device != "" {
		return config.Camera{}, "", fmt.Errorf("camera name and --device are mutually exclusive")
	}
	if cameraName == "" && device == "" {
		return config.Camera{}, "", fmt.Errorf("--camera or --device is required")
	}
	if device != "" {
		return config.Camera{Name: device, Protocol: "local", Device: device}, device, nil
	}

	cfg, _, err := loadConfigFromFlag(cmd)
	if err != nil {
		return config.Camera{}, "", err
	}
	cam, ok := findCamera(cfg, cameraName)
	if !ok {
		return config.Camera{}, "", fmt.Errorf("camera %q not found", cameraName)
	}
	return cam, cameraName, nil
}
