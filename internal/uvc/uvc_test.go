package uvc

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestRange(t *testing.T) {
	t.Parallel()

	r := Range{Min: 100, Max: 500, Res: 20, Def: 100}
	tests := []struct {
		name        string
		value       int32
		wantClamped int32
		wantPercent float64
	}{
		{name: "minimum", value: 0, wantClamped: 100, wantPercent: 0},
		{name: "snaps down", value: 149, wantClamped: 140, wantPercent: 10},
		{name: "snaps up", value: 151, wantClamped: 160, wantPercent: 15},
		{name: "maximum", value: 900, wantClamped: 500, wantPercent: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := r.Clamp(tt.value); got != tt.wantClamped {
				t.Fatalf("Clamp(%d) = %d, want %d", tt.value, got, tt.wantClamped)
			}
			if got := r.PercentOf(tt.value); got != tt.wantPercent {
				t.Fatalf("PercentOf(%d) = %v, want %v", tt.value, got, tt.wantPercent)
			}
		})
	}

	percentTests := []struct {
		percent float64
		want    int32
	}{
		{percent: -10, want: 100},
		{percent: 25, want: 200},
		{percent: 100, want: 500},
		{percent: 125, want: 500},
	}
	for _, tt := range percentTests {
		if got := r.FromPercent(tt.percent); got != tt.want {
			t.Errorf("FromPercent(%v) = %d, want %d", tt.percent, got, tt.want)
		}
	}
}

func TestDegreesArcsecondsRoundTrip(t *testing.T) {
	t.Parallel()

	for _, degrees := range []float64{-42.25, 0, 12.5, 90} {
		arcseconds := DegreesToArcsec(degrees)
		if got := ArcsecToDegrees(arcseconds); got != degrees {
			t.Errorf("ArcsecToDegrees(DegreesToArcsec(%v)) = %v", degrees, got)
		}
	}
	if got := DegreesToArcsec(math.MaxFloat64); got != math.MaxInt32 {
		t.Fatalf("DegreesToArcsec(MaxFloat64) = %d, want %d", got, int32(math.MaxInt32))
	}
}

func TestParseUSBUniqueID(t *testing.T) {
	t.Parallel()

	locationID, vendorID, productID, err := ParseUSBUniqueID("0x21100002e1a4c06")
	if err != nil {
		t.Fatal(err)
	}
	if locationID != 0x02110000 || vendorID != 0x2e1a || productID != 0x4c06 {
		t.Fatalf("ParseUSBUniqueID = %#x, %#x, %#x", locationID, vendorID, productID)
	}

	for _, id := range []string{"", "not-hex", "0x2e1a4c06", "0x1234567890abcdef0"} {
		if _, _, _, err := ParseUSBUniqueID(id); err == nil || !strings.Contains(err.Error(), "USB camera unique ID") && !strings.Contains(err.Error(), "USB location ID") {
			t.Errorf("ParseUSBUniqueID(%q) error = %v", id, err)
		}
	}
}

func TestPanTiltEncoding(t *testing.T) {
	t.Parallel()

	want := []byte{0xc0, 0x1d, 0xfe, 0xff, 0xa0, 0x8c, 0x00, 0x00}
	encoded := encodePanTilt(-123456, 36000)
	if !reflect.DeepEqual(encoded, want) {
		t.Fatalf("encodePanTilt = % x, want % x", encoded, want)
	}
	pan, tilt, err := decodePanTilt(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if pan != -123456 || tilt != 36000 {
		t.Fatalf("decodePanTilt = %d, %d", pan, tilt)
	}
	if _, _, err := decodePanTilt(make([]byte, 7)); err == nil {
		t.Fatal("decodePanTilt accepted a short payload")
	}
}

func TestZoomEncoding(t *testing.T) {
	t.Parallel()

	want := []byte{0x34, 0x12}
	encoded := encodeZoom(0x1234)
	if !reflect.DeepEqual(encoded, want) {
		t.Fatalf("encodeZoom = % x, want % x", encoded, want)
	}
	zoom, err := decodeZoom(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if zoom != 0x1234 {
		t.Fatalf("decodeZoom = %#x", zoom)
	}
	if _, err := decodeZoom(make([]byte, 1)); err == nil {
		t.Fatal("decodeZoom accepted a short payload")
	}
}

func TestParseCameraTerminalDescriptor(t *testing.T) {
	t.Parallel()

	// Synthetic VideoControl descriptor block: header, output terminal, then a
	// camera input terminal with three bmControls bytes.
	descriptors := []byte{
		13, 0x24, 0x01, 0x50, 0x01, 40, 0, 0, 0, 0, 0, 0, 0,
		9, 0x24, 0x03, 2, 0x01, 0x01, 0, 1, 0,
		18, 0x24, 0x02, 5, 0x01, 0x02, 0, 0, 0, 0, 0, 0, 0, 0, 3, 0x00, 0x1a, 0x00,
	}
	unitID, controls, ok := parseCameraTerminalDescriptor(descriptors)
	if !ok {
		t.Fatal("parseCameraTerminalDescriptor did not find the input terminal")
	}
	if unitID != 5 || controls != 0x1a00 {
		t.Fatalf("parseCameraTerminalDescriptor = %d, %#x", unitID, controls)
	}
	capabilities := capabilitiesFromControls(controls)
	want := Capabilities{PanTiltAbsolute: true, PanTiltRelative: true, ZoomAbsolute: true, ZoomRelative: false}
	if capabilities != want {
		t.Fatalf("capabilities = %#v, want %#v", capabilities, want)
	}
}

func TestParseCameraTerminalDescriptorRejectsMalformedData(t *testing.T) {
	t.Parallel()

	tests := [][]byte{
		nil,
		{0, 0x24, 0x01, 0, 0, 7, 0},
		{7, 0x24, 0x01, 0, 0, 8, 0, 0},
		{7, 0x24, 0x01, 0, 0, 20, 0, 18, 0x24, 0x02},
	}
	for _, descriptors := range tests {
		if _, _, ok := parseCameraTerminalDescriptor(descriptors); ok {
			t.Errorf("parseCameraTerminalDescriptor(% x) succeeded", descriptors)
		}
	}
}
