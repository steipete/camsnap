package rtspclient

import (
	"context"
	"testing"
	"time"

	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
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
