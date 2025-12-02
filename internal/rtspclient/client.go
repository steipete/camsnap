// Package rtspclient provides RTSP helpers using gortsplib.
package rtspclient

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtph264"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/pion/rtp"
)

// GrabFrameViaGort connects with gortsplib, reads until a random-access (IDR) frame, then pipes it to ffmpeg to save a JPEG.
func GrabFrameViaGort(ctx context.Context, url, transport, outPath string, timeout time.Duration) error {
	if transport == "" {
		transport = "udp"
	}

	u, err := base.ParseURL(url)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}

	cl := &gortsplib.Client{
		Scheme:       u.Scheme,
		Host:         u.Host,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	switch transport {
	case "udp":
		t := gortsplib.TransportUDP
		cl.Transport = &t
	case "tcp":
		t := gortsplib.TransportTCP
		cl.Transport = &t
	default:
		return fmt.Errorf("invalid transport %q", transport)
	}

	if err := cl.Start2(); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	defer cl.Close()

	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	desc, _, err := cl.Describe(u)
	if err != nil {
		return fmt.Errorf("describe: %w", err)
	}

	medi, fmtH264 := findH264(desc.Medias)
	if medi == nil || fmtH264 == nil {
		return fmt.Errorf("no H264 track found")
	}

	// ensure auth propagated to setup/play
	if u.User != nil && desc.BaseURL != nil {
		desc.BaseURL.User = u.User
	}

	if _, err := cl.Setup(desc.BaseURL, medi, 0, 0); err != nil {
		return fmt.Errorf("setup video: %w", err)
	}

	dec, err := fmtH264.CreateDecoder()
	if err != nil {
		return fmt.Errorf("decoder: %w", err)
	}

	var sample bytes.Buffer
	done := make(chan struct{})
	errCh := make(chan error, 1)

	cl.OnPacketRTP(medi, fmtH264, func(pkt *rtp.Packet) {
		nalus, err := dec.Decode(pkt)
		if err != nil {
			if err != rtph264.ErrMorePacketsNeeded {
				// ignore non-fatal decode errors
				return
			}
			return
		}
		if len(nalus) == 0 {
			return
		}
		for _, n := range nalus {
			sample.Write([]byte{0x00, 0x00, 0x00, 0x01})
			sample.Write(n)
		}
		if h264.IsRandomAccess(nalus) {
			select {
			case <-done:
			default:
				close(done)
			}
		}
	})

	if _, err := cl.Play(nil); err != nil {
		return fmt.Errorf("play: %w", err)
	}

	go func() {
		if err := cl.Wait(); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-done:
	case err := <-errCh:
		return fmt.Errorf("rtsp client: %w", err)
	case <-ctxTimeout.Done():
		return fmt.Errorf("timeout waiting for frame")
	}

	// feed collected H264 to ffmpeg to produce jpeg
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y",
		"-f", "h264",
		"-i", "pipe:0",
		"-frames:v", "1",
		outPath,
	)
	cmd.Stdin = bytes.NewReader(sample.Bytes())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg write frame: %w\n%s", err, string(out))
	}
	return nil
}

func findH264(medias []*description.Media) (*description.Media, *format.H264) {
	for _, m := range medias {
		for _, f := range m.Formats {
			if h, ok := f.(*format.H264); ok {
				return m, h
			}
		}
	}
	return nil, nil
}
