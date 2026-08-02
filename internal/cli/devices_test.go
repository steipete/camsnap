package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/steipete/camsnap/internal/capture"
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

func TestParseAVFoundationDevicesSortsIndicesNumerically(t *testing.T) {
	output := `[AVFoundation indev @ 0x1] AVFoundation video devices:
[AVFoundation indev @ 0x1] [10] Camera 10
[AVFoundation indev @ 0x1] [2] Camera 2
[AVFoundation indev @ 0x1] [0] Camera 0
[AVFoundation indev @ 0x1] [9] Camera 9
[AVFoundation indev @ 0x1] [1] Camera 1
[AVFoundation indev @ 0x1] [8] Camera 8
[AVFoundation indev @ 0x1] [3] Camera 3
[AVFoundation indev @ 0x1] [7] Camera 7
[AVFoundation indev @ 0x1] [4] Camera 4
[AVFoundation indev @ 0x1] [6] Camera 6
[AVFoundation indev @ 0x1] [5] Camera 5
[AVFoundation indev @ 0x1] AVFoundation audio devices:`
	devices := parseAVFoundationDevices(output)
	want := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}
	if got := deviceIndices(devices); !reflect.DeepEqual(got, want) {
		t.Fatalf("device indices = %v, want %v", got, want)
	}
}

func TestWriteLocalDevicesTableNativeIncludesIndex(t *testing.T) {
	var output bytes.Buffer
	devices := []localDevice{{Index: "3", ID: "camera-id", Name: "Camera", IsDefault: true}}
	if err := writeLocalDevicesTable(&output, capture.LocalBackendNative, devices); err != nil {
		t.Fatal(err)
	}
	want := "INDEX  ID         NAME    DEFAULT\n3      camera-id  Camera  true\n"
	if got := output.String(); got != want {
		t.Fatalf("native devices table:\n%q\nwant:\n%q", got, want)
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

func deviceIndices(devices []localDevice) []string {
	indices := make([]string, len(devices))
	for i, device := range devices {
		indices[i] = device.Index
	}
	return indices
}
