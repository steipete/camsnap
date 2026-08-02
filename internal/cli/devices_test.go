package cli

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
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

func TestLocalDevicesJSONIncludesBackendAndDefault(t *testing.T) {
	data, err := json.Marshal(localDevicesOutput{
		Backend:             "native",
		AuthorizationStatus: "authorized",
		Devices:             []localDevice{{ID: "camera-id", Name: "Camera", IsDefault: false}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"backend":"native"`, `"authorization_status":"authorized"`, `"id":"camera-id"`, `"default":false`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("JSON %s missing %s", data, want)
		}
	}
}
