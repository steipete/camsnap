package rtspclient

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
)

func TestFindH264(t *testing.T) {
	medias := []*description.Media{
		{
			Type:    description.MediaTypeVideo,
			Formats: []format.Format{&format.H264{}},
		},
	}
	medi, fmt := findH264(medias)
	if medi == nil || fmt == nil {
		t.Fatalf("expected h264 media/format")
	}
}

func TestH264SampleIncludesSessionParameters(t *testing.T) {
	sps, pps, idr := []byte{0x67, 0x64}, []byte{0x68, 0x23}, []byte{0x65, 0x42}
	sample := h264Sample{sps: sps, pps: pps}
	for range 100 {
		if got := sample.frame([][]byte{{0x61, 0x01}}); got != nil {
			t.Fatal("non-IDR frame should be discarded")
		}
	}
	var got h264.AnnexB
	if err := got.Unmarshal(sample.frame([][]byte{idr})); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual([][]byte(got), [][]byte{sps, pps, idr}) {
		t.Fatalf("sample = %x, want session parameters followed by IDR", got)
	}
}

func TestH264SampleRetainsUpdatedParameters(t *testing.T) {
	sample := h264Sample{sps: []byte{0x67, 0x01}, pps: []byte{0x68, 0x02}}
	sps, pps := []byte{0x67, 0x03}, []byte{0x68, 0x04}
	wantSPS, wantPPS := bytes.Clone(sps), bytes.Clone(pps)
	if got := sample.frame([][]byte{sps, pps}); got != nil {
		t.Fatal("parameter update is not a keyframe")
	}
	sps[1], pps[1] = 0xff, 0xff
	idr := []byte{0x65, 0x42}
	var got h264.AnnexB
	if err := got.Unmarshal(sample.frame([][]byte{nil, idr})); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual([][]byte(got), [][]byte{wantSPS, wantPPS, idr}) {
		t.Fatalf("sample = %x, want copied in-band parameters followed by IDR", got)
	}
}

func TestFindH264None(t *testing.T) {
	medias := []*description.Media{
		{
			Type:    description.MediaTypeAudio,
			Formats: []format.Format{},
		},
	}
	medi, fmt := findH264(medias)
	if medi != nil || fmt != nil {
		t.Fatalf("expected nil for missing h264")
	}
}

func TestGrabFrameViaGortBadURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := GrabFrameViaGort(ctx, "rtsp://127.0.0.1:0/stream1", "udp", "", t.TempDir()+"/out.jpg", 500*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error on invalid url/connection")
	}
}

func TestGrabFrameViaGortInvalidTransport(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := GrabFrameViaGort(ctx, "rtsp://127.0.0.1:0/stream1", "invalid", "", t.TempDir()+"/out.jpg", 500*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error on invalid transport")
	}
}

func TestGrabFrameViaGortRejectsAuthMode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := GrabFrameViaGort(ctx, "rtsp://127.0.0.1:9/stream1", "tcp", "ntlm", t.TempDir()+"/out.jpg", time.Second)
	if err == nil || err.Error() != `invalid RTSP auth mode "ntlm"` {
		t.Fatalf("error = %v", err)
	}
}

func TestGrabFrameViaGortAuthMode(t *testing.T) {
	tests := []struct {
		name   string
		mode   string
		prefix string
	}{
		{name: "basic", mode: "basic", prefix: "Basic "},
		{name: "digest", mode: "digest", prefix: "Digest "},
		{name: "auto", mode: "", prefix: "Digest "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authCh := make(chan string, 1)
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer ln.Close()
			go serveRTSPAuth(ln, authCh)

			port := ln.Addr().(*net.TCPAddr).Port
			url := fmt.Sprintf("rtsp://user:pass@127.0.0.1:%d/stream1", port)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			err = GrabFrameViaGort(ctx, url, "tcp", tt.mode, t.TempDir()+"/out.jpg", time.Second)
			if err == nil {
				t.Fatal("expected describe to stop before a frame")
			}
			select {
			case got := <-authCh:
				if !strings.HasPrefix(got, tt.prefix) {
					t.Fatalf("Authorization = %q, want prefix %q (client error: %v)", got, tt.prefix, err)
				}
			case <-time.After(3 * time.Second):
				t.Fatalf("no Authorization header for %s: %v", tt.name, err)
			}
		})
	}
}

func serveRTSPAuth(ln net.Listener, authCh chan<- string) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go handleRTSPAuth(conn, authCh)
	}
}

func handleRTSPAuth(conn net.Conn, authCh chan<- string) {
	defer conn.Close()
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 2048)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, err := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		for {
			idx := bytes.Index(buf, []byte("\r\n\r\n"))
			if idx < 0 {
				break
			}
			raw := string(buf[:idx])
			buf = buf[idx+4:]
			replyRTSPAuth(conn, raw, authCh)
		}
		if err != nil {
			return
		}
	}
}

func replyRTSPAuth(conn net.Conn, raw string, authCh chan<- string) {
	lines := strings.Split(raw, "\r\n")
	if len(lines) == 0 || lines[0] == "" {
		return
	}
	method := strings.SplitN(lines[0], " ", 2)[0]
	cseq := "1"
	auth := ""
	for _, line := range lines[1:] {
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "cseq":
			cseq = strings.TrimSpace(value)
		case "authorization":
			auth = strings.TrimSpace(value)
		}
	}
	var body string
	switch {
	case method == "OPTIONS":
		body = "RTSP/1.0 200 OK\r\nCSeq: " + cseq + "\r\nPublic: OPTIONS, DESCRIBE, SETUP, PLAY, TEARDOWN\r\n\r\n"
	case auth == "":
		body = "RTSP/1.0 401 Unauthorized\r\nCSeq: " + cseq + "\r\n" +
			"WWW-Authenticate: Digest realm=\"cam\", nonce=\"nonce1\", stale=\"FALSE\"\r\n" +
			"WWW-Authenticate: Basic realm=\"cam\"\r\n\r\n"
	default:
		select {
		case authCh <- auth:
		default:
		}
		body = "RTSP/1.0 404 Not Found\r\nCSeq: " + cseq + "\r\n\r\n"
	}
	_, _ = conn.Write([]byte(body))
}
