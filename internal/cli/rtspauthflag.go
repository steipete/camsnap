package cli

import "strings"

// parseRTSPAuth maps user-friendly flag to ffmpeg-compatible value.
func parseRTSPAuth(mode string) (string, bool) {
	m := strings.ToLower(mode)
	switch m {
	case "", "auto":
		return "", true
	case "basic":
		return "basic", true
	case "digest":
		return "digest", true
	default:
		return "", false
	}
}
