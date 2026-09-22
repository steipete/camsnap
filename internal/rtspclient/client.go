// Package rtspclient provides RTSP helpers using gortsplib.
package rtspclient

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
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
		p := gortsplib.ProtocolUDP
		cl.Protocol = &p
	case "tcp":
		p := gortsplib.ProtocolTCP
		cl.Protocol = &p
	default:
		return fmt.Errorf("invalid transport %q", transport)
	}

	if err := cl.Start(); err != nil {
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

	sample := h264Sample{sps: bytes.Clone(fmtH264.SPS), pps: bytes.Clone(fmtH264.PPS)}
	var frame []byte
	var sampleMu sync.Mutex
	done := make(chan struct{})
	errCh := make(chan error, 1)

	cl.OnPacketRTP(medi, fmtH264, func(pkt *rtp.Packet) {
		select {
		case <-done:
			return
		default:
		}
		nalus, err := dec.Decode(pkt)
		if err != nil {
			return
		}
		if len(nalus) == 0 {
			return
		}
		sampleMu.Lock()
		defer sampleMu.Unlock()
		select {
		case <-done:
			return
		default:
		}
		if frame = sample.frame(nalus); frame != nil {
			close(done)
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

	var sampleData []byte
	select {
	case <-done:
		sampleMu.Lock()
		sampleData = frame
		sampleMu.Unlock()
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
	cmd.Stdin = bytes.NewReader(sampleData)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg write frame: %w\n%s", err, string(out))
	}
	return nil
}

type h264Sample struct {
	sps, pps []byte
}

func (s *h264Sample) frame(nalus [][]byte) []byte {
	keyframe := false
	for _, nalu := range nalus {
		if len(nalu) == 0 {
			continue
		}
		switch h264.NALUType(nalu[0] & 0x1f) {
		case h264.NALUTypeSPS:
			s.sps = bytes.Clone(nalu)
		case h264.NALUTypePPS:
			s.pps = bytes.Clone(nalu)
		case h264.NALUTypeIDR:
			keyframe = true
		}
	}
	if !keyframe {
		return nil
	}

	// Cameras may send codec parameters only in SDP, or update them in-band.
	// Feed ffmpeg those parameters and the IDR, without undecodable earlier frames.
	var sample bytes.Buffer
	for _, nalu := range append([][]byte{s.sps, s.pps}, nalus...) {
		if len(nalu) != 0 {
			sample.Write([]byte{0, 0, 0, 1})
			sample.Write(nalu)
		}
	}
	return sample.Bytes()
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
