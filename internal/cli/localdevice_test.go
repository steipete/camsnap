package cli

import (
	"strings"
	"testing"
)

func TestResolveNativeDevice(t *testing.T) {
	standardDevices := []localDevice{
		{ID: "first-id", Index: "0", Name: "First Camera"},
		{ID: "second-id", Index: "1", Name: "Second Camera"},
	}
	tests := []struct {
		name     string
		devices  []localDevice
		selector string
		wantID   string
		wantErr  []string
	}{
		{name: "exact name", devices: standardDevices, selector: "Second Camera", wantID: "second-id"},
		{name: "case insensitive name", devices: standardDevices, selector: "second camera", wantID: "second-id"},
		{name: "unique ID", devices: standardDevices, selector: "first-id", wantID: "first-id"},
		{name: "numeric index", devices: standardDevices, selector: "1", wantID: "second-id"},
		{name: "out of range index", devices: standardDevices, selector: "9", wantErr: []string{"index 9", "out of range", "found 2 devices"}},
		{
			name: "ambiguous case insensitive name",
			devices: []localDevice{
				{ID: "upper-id", Name: "Cam"},
				{ID: "lower-id", Name: "cam"},
			},
			selector: "CAM",
			wantErr:  []string{"ambiguous", "Cam (upper-id)", "cam (lower-id)"},
		},
		{
			name: "exact name wins over case insensitive",
			devices: []localDevice{
				{ID: "lower-id", Name: "cam"},
				{ID: "exact-id", Name: "Cam"},
			},
			selector: "Cam",
			wantID:   "exact-id",
		},
		{
			name: "duplicate exact names use first",
			devices: []localDevice{
				{ID: "first-id", Name: "Camera"},
				{ID: "second-id", Name: "Camera"},
			},
			selector: "Camera",
			wantID:   "first-id",
		},
		{
			name:     "not found lists available devices",
			devices:  standardDevices,
			selector: "Missing Camera",
			wantErr:  []string{`native camera "Missing Camera" not found`, "First Camera (first-id)", "Second Camera (second-id)"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveNativeDevice(test.devices, test.selector)
			if len(test.wantErr) > 0 {
				if err == nil {
					t.Fatalf("resolveNativeDevice(%q) returned nil error", test.selector)
				}
				for _, want := range test.wantErr {
					if !strings.Contains(err.Error(), want) {
						t.Fatalf("resolveNativeDevice(%q) error %q missing %q", test.selector, err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveNativeDevice(%q): %v", test.selector, err)
			}
			if got.ID != test.wantID {
				t.Fatalf("resolveNativeDevice(%q) ID = %q, want %q", test.selector, got.ID, test.wantID)
			}
		})
	}
}

func TestDefaultNativeDevice(t *testing.T) {
	devices := []localDevice{
		{ID: "first-id", Name: "First Camera"},
		{ID: "default-id", Name: "Default Camera", IsDefault: true},
	}
	got, ok := defaultNativeDevice(devices)
	if !ok || got.ID != "default-id" {
		t.Fatalf("defaultNativeDevice() = %#v, %t", got, ok)
	}
	if _, ok := defaultNativeDevice(devices[:1]); ok {
		t.Fatal("defaultNativeDevice() found a device without a default marker")
	}
}
