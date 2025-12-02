package exec

import (
	"context"
	"testing"
)

func TestClassifyError(t *testing.T) {
	cases := []struct {
		err  string
		want string
	}{
		{"401 Unauthorized", "auth"},
		{"Server returned 401 unauthorized", "auth"},
		{"Connection refused", "network-refused"},
		{"timed out", "network-timeout"},
		{"not found", "not-found"},
		{"weird message", "unknown"},
	}
	for _, c := range cases {
		if got := ClassifyError(c.err); got != c.want {
			t.Fatalf("ClassifyError(%q) got %s want %s", c.err, got, c.want)
		}
	}
}

func TestWithTimeoutZero(t *testing.T) {
	ctx, cancel := WithTimeout(context.Background(), 0)
	defer cancel()
	select {
	case <-ctx.Done():
		t.Fatalf("zero timeout context should not be canceled immediately")
	default:
	}
}
