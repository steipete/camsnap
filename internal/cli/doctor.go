package cli

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/camsnap/internal/capture"
	mediaexec "github.com/steipete/camsnap/internal/exec"
)

func newDoctorCmd() *cobra.Command {
	var timeout time.Duration
	var probe bool
	var authMode string
	var transport string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check ffmpeg and configured RTSP/local cameras",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sty := newStyler(cmd.OutOrStdout())

			cfgFlag, err := configPathFlag(cmd)
			if err != nil {
				return err
			}
			cfg, path, err := loadConfig(cfgFlag)
			if err != nil {
				return err
			}

			ffmpegFound := mediaexec.HasBinary("ffmpeg")
			if ffmpegFound {
				cmd.Println(sty.OK("✔ ffmpeg found in PATH"))
			} else {
				cmd.Println(sty.Err("✖ ffmpeg missing (install ffmpeg and retry)"))
			}

			cmd.Printf("Config file: %s\n", path)
			if len(cfg.Cameras) == 0 {
				cmd.Println(sty.Warn("No cameras saved. Add one with camsnap add ..."))
				return nil
			}

			var (
				localDevices    []localDevice
				localDevicesErr error
			)
			for _, cam := range cfg.Cameras {
				if strings.EqualFold(cam.Protocol, "local") {
					if ffmpegFound {
						localDevices, localDevicesErr = enumerateLocalDevices(runtime.GOOS)
					} else {
						localDevicesErr = fmt.Errorf("ffmpeg missing")
					}
					break
				}
			}

			for _, cam := range cfg.Cameras {
				if strings.EqualFold(cam.Protocol, "local") {
					options, err := capture.Resolve(cam, (captureFlagValues{
						transport: transport,
						rtspAuth:  authMode,
					}).overrides(cmd))
					if err != nil {
						cmd.Printf("%s %s local camera invalid: %v\n", sty.Err("✖"), cam.Name, err)
						continue
					}
					if localDevicesErr != nil {
						cmd.Printf("%s %s device enumeration failed: %v\n", sty.Err("✖"), cam.Name, localDevicesErr)
						continue
					}
					if !localDeviceVisible(localDevices, cam.Device) {
						cmd.Printf("%s %s device %q not found in local enumeration\n", sty.Err("✖"), cam.Name, cam.Device)
						continue
					}
					if probe {
						ctx, cancel := mediaexec.WithTimeout(context.Background(), timeout+2*time.Second)
						err = runLocalCapture(ctx, localCaptureRequest{operation: localProbe, options: options})
						cancel()
						if err != nil {
							cmd.Printf("%s %s local capture probe failed: %v\n", sty.Err("✖"), cam.Name, err)
							continue
						}
					}
					cmd.Printf("%s %s device %q visible\n", sty.OK("✔"), cam.Name, cam.Device)
					continue
				}
				host := cam.Host
				port := cam.Port
				if port == 0 {
					port = 554
				}
				addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))

				if err := dialOnce(addr, timeout); err != nil {
					cmd.Printf("%s %s dial %s failed: %v\n", sty.Err("✖"), cam.Name, addr, err)
					continue
				}
				options, err := capture.Resolve(cam, (captureFlagValues{
					transport: transport,
					rtspAuth:  authMode,
				}).overrides(cmd))
				if err != nil {
					cmd.Printf("%s %s RTSP URL invalid: %v\n", sty.Err("✖"), cam.Name, err)
					continue
				}
				if probe {
					probeArgs, err := capture.ProbeArgs(options, runtime.GOOS)
					if err != nil {
						cmd.Printf("%s %s ffmpeg probe invalid: %v\n", sty.Err("✖"), cam.Name, err)
						continue
					}
					if err := probeRTSP(cmd, probeArgs, timeout+2*time.Second, authMode); err != nil {
						cmd.Printf("%s %s ffmpeg probe failed: %v\n", sty.Err("✖"), cam.Name, err)
						continue
					}
				}
				cmd.Printf("%s %s reachable at %s\n", sty.OK("✔"), cam.Name, addr)
			}
			return nil
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", 2*time.Second, "Dial timeout per camera")
	cmd.Flags().BoolVar(&probe, "probe", false, "Use ffmpeg to probe each camera briefly")
	cmd.Flags().StringVar(&authMode, "rtsp-auth", "auto", "RTSP auth mode: auto|basic|digest")
	cmd.Flags().StringVar(&transport, "rtsp-transport", "tcp", "RTSP transport: tcp|udp (probe)")
	return cmd
}

func dialOnce(addr string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return err
	}
	return conn.Close()
}

func probeRTSP(_ *cobra.Command, args []string, timeout time.Duration, authMode string) error {
	// retry a couple times to avoid transient RTSP setup errors
	var lastErr error
	var lastOut string
	if _, ok := parseRTSPAuth(authMode); !ok {
		return fmt.Errorf("invalid --rtsp-auth (use auto|basic|digest)")
	}

	for attempt := 0; attempt < 3; attempt++ {
		ctx, cancel := mediaexec.WithTimeout(context.Background(), timeout)
		lastOut, lastErr = mediaexec.RunFFmpegWithOutput(ctx, args...)
		cancel()
		if lastErr == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	class := mediaexec.ClassifyError(lastOut)
	return fmt.Errorf("%s (%s)", lastErr, class)
}
