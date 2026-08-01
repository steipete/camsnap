package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type localDevice struct {
	Index string `json:"index"`
	Name  string `json:"name"`
}

var avfoundationDevicePattern = regexp.MustCompile(`\[(\d+)\]\s+(.+)$`)

func newDevicesCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "devices",
		Short: "List local video capture devices",
		RunE: func(cmd *cobra.Command, _ []string) error {
			devices, err := enumerateLocalDevices(runtime.GOOS)
			if err != nil {
				return err
			}
			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(devices)
			}
			if len(devices) == 0 {
				if runtime.GOOS == "darwin" {
					cmd.Println("No local video devices found. Continuity Camera appears only while the iPhone is nearby and unlocked.")
				} else {
					cmd.Println("No local video devices found.")
				}
				return nil
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(writer, "INDEX\tNAME")
			for _, device := range devices {
				_, _ = fmt.Fprintf(writer, "%s\t%s\n", device.Index, device.Name)
			}
			return writer.Flush()
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print devices as JSON")
	return cmd
}

func enumerateLocalDevices(goos string) ([]localDevice, error) {
	switch goos {
	case "darwin":
		output, err := exec.Command("ffmpeg", "-f", "avfoundation", "-list_devices", "true", "-i", "").CombinedOutput()
		if err != nil {
			var exitError *exec.ExitError
			if !errors.As(err, &exitError) {
				return nil, fmt.Errorf("list avfoundation devices: %w", err)
			}
		}
		return parseAVFoundationDevices(string(output)), nil
	case "linux":
		paths, err := filepath.Glob("/dev/video*")
		if err != nil {
			return nil, fmt.Errorf("list v4l2 devices: %w", err)
		}
		devices := make([]localDevice, 0, len(paths))
		for _, path := range paths {
			devices = append(devices, localDevice{Index: strings.TrimPrefix(path, "/dev/video"), Name: path})
		}
		return devices, nil
	default:
		return nil, fmt.Errorf("local webcams are unsupported on %s", goos)
	}
}

func parseAVFoundationDevices(stderr string) []localDevice {
	devices := make([]localDevice, 0)
	inVideoSection := false
	for _, line := range strings.Split(stderr, "\n") {
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "avfoundation video devices:"):
			inVideoSection = true
			continue
		case strings.Contains(lower, "avfoundation audio devices:"):
			inVideoSection = false
			continue
		}
		if !inVideoSection {
			continue
		}
		matches := avfoundationDevicePattern.FindStringSubmatch(line)
		if len(matches) == 3 {
			devices = append(devices, localDevice{Index: matches[1], Name: strings.TrimSpace(matches[2])})
		}
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].Index < devices[j].Index })
	return devices
}

func localDeviceVisible(devices []localDevice, configured string) bool {
	for _, device := range devices {
		if device.Index == configured || device.Name == configured {
			return true
		}
	}
	return false
}
