package cli

import (
	"os"
	"reflect"
	"testing"
)

func TestParseAVFoundationDevicesFixture(t *testing.T) {
	fixture, err := os.ReadFile("testdata/avfoundation_devices.txt")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	want := []localDevice{
		{Index: "0", Name: "Capture screen 0"},
	}
	if got := parseAVFoundationDevices(string(fixture)); !reflect.DeepEqual(got, want) {
		t.Fatalf("parseAVFoundationDevices = %#v, want %#v", got, want)
	}
}
