package cli

import "testing"

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
