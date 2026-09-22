package rtspclient

import (
	"bytes"
	"context"
	"reflect"
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

	err := GrabFrameViaGort(ctx, "rtsp://127.0.0.1:0/stream1", "udp", t.TempDir()+"/out.jpg", 500*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error on invalid url/connection")
	}
}

func TestGrabFrameViaGortInvalidTransport(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := GrabFrameViaGort(ctx, "rtsp://127.0.0.1:0/stream1", "invalid", t.TempDir()+"/out.jpg", 500*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error on invalid transport")
	}
}
