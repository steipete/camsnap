//go:build darwin && cgo

package cli

import "testing"

func TestResolveNativeDevice(t *testing.T) {
	devices := []localDevice{
		{ID: "first-id", Index: "0", Name: "First Camera"},
		{ID: "second-id", Index: "1", Name: "Second Camera"},
	}
	for _, test := range []struct {
		selector string
		wantID   string
	}{
		{selector: "1", wantID: "second-id"},
		{selector: "first-id", wantID: "first-id"},
		{selector: "Second Camera", wantID: "second-id"},
	} {
		got, err := resolveNativeDevice(devices, test.selector)
		if err != nil {
			t.Fatalf("resolveNativeDevice(%q): %v", test.selector, err)
		}
		if got.ID != test.wantID {
			t.Fatalf("resolveNativeDevice(%q) ID = %q, want %q", test.selector, got.ID, test.wantID)
		}
	}
}

func TestResolveNativeDeviceRejectsUnknownSelector(t *testing.T) {
	devices := []localDevice{{ID: "camera-id", Index: "0", Name: "Camera"}}
	for _, selector := range []string{"2", "Missing Camera"} {
		if _, err := resolveNativeDevice(devices, selector); err == nil {
			t.Fatalf("resolveNativeDevice(%q) returned nil error", selector)
		}
	}
}
