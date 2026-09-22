package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestParseSceneScore(t *testing.T) {
	line := "[Parsed_metadata_1] scene_score=0.321 something"
	score, ok := parseSceneScore(line)
	if !ok {
		t.Fatalf("expected score")
	}
	if score < 0.32 || score > 0.322 {
		t.Fatalf("unexpected score %f", score)
	}
	if _, ok := parseSceneScore("no score here"); ok {
		t.Fatalf("expected no match")
	}
}

func TestMotionJSONLines(t *testing.T) {
	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	camera := "front \"door\"\\camera\n\t\x01☃"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	handler := motionLineHandler(ctx, camera, 0, ":", "", true, cmd)
	handler("lavfi.scene_score=0.321")
	handler("lavfi.scene_score=0.456")
	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected two JSON lines, got %q", output.String())
	}
	for i, line := range lines {
		var event struct {
			Event  string    `json:"event"`
			Camera string    `json:"camera"`
			Score  float64   `json:"score"`
			Time   time.Time `json:"time"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("invalid event %q: %v", line, err)
		}
		if event.Event != "motion" || event.Camera != camera || event.Score != []float64{0.321, 0.456}[i] || event.Time.IsZero() {
			t.Fatalf("unexpected event: %+v", event)
		}
	}
}

func TestActionTemplateDoesNotReinterpretCameraName(t *testing.T) {
	now := time.Now()
	got := applyTemplate("{camera} {score} {time}", "camera-{score}-{time}", 0.321, now)
	want := "camera-{score}-{time} 0.321 " + now.Format(time.RFC3339Nano)
	if got != want {
		t.Fatalf("template = %q, want %q", got, want)
	}
}
