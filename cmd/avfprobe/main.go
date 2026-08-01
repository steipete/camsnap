//go:build darwin && cgo

// avfprobe exercises the Darwin-only AVFoundation capture spike.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/steipete/camsnap/internal/avf"
)

const usage = `usage: avfprobe <command> [options]

commands:
  devices              list local video capture devices
  status               print the camera authorization status
  request              request camera access and wait for the result
  snap [options]       capture one JPEG frame

snap options:
  --device string      device unique ID (default device when empty)
  --warmup duration    exposure warmup (default 1s)
  --out path           JPEG output path (required)
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "avfprobe: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return flag.ErrHelp
	}

	switch args[0] {
	case "devices":
		if len(args) != 1 {
			return fmt.Errorf("devices takes no arguments")
		}
		return listDevices()
	case "status":
		if len(args) != 1 {
			return fmt.Errorf("status takes no arguments")
		}
		fmt.Println(avf.AuthorizationStatus())
		return nil
	case "request":
		if len(args) != 1 {
			return fmt.Errorf("request takes no arguments")
		}
		return requestAccess()
	case "snap":
		return snap(args[1:])
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func listDevices() error {
	devices, err := avf.ListDevices()
	if err != nil {
		return err
	}
	for _, device := range devices {
		defaultMark := ""
		if device.IsDefault {
			defaultMark = " (default)"
		}
		fmt.Printf("%s\t%s%s\n", device.UniqueID, device.Name, defaultMark)
	}
	return nil
}

func requestAccess() error {
	granted, err := awaitAccess()
	if err != nil {
		return err
	}
	if !granted {
		return fmt.Errorf("camera access was not granted")
	}
	return nil
}

func awaitAccess() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	granted, err := avf.RequestAccess(ctx)
	if err != nil {
		return false, err
	}
	fmt.Printf("granted=%t status=%s\n", granted, avf.AuthorizationStatus())
	return granted, nil
}

func snap(args []string) error {
	flags := flag.NewFlagSet("snap", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	deviceID := flags.String("device", "", "device unique ID")
	warmup := flags.Duration("warmup", avf.DefaultWarmup, "exposure warmup")
	outPath := flags.String("out", "", "JPEG output path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("snap takes only flag arguments")
	}
	if *outPath == "" {
		return fmt.Errorf("--out is required")
	}

	if avf.AuthorizationStatus() == "notDetermined" {
		fmt.Fprintln(os.Stderr, "camera access is notDetermined; requesting access")
		granted, err := awaitAccess()
		if err != nil {
			return err
		}
		if !granted {
			fmt.Fprintln(os.Stderr, "camera access was not granted; attempting capture to record the AVFoundation error")
		}
	}

	if err := avf.CaptureFrame(*deviceID, *warmup, *outPath); err != nil {
		return err
	}
	fmt.Println(*outPath)
	return nil
}
