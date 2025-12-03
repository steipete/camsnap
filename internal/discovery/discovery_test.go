package discovery

import (
	"context"
	"strings"
	"testing"
	"time"
)

const sampleProbeMatch = `<?xml version="1.0" encoding="UTF-8"?>
<e:Envelope xmlns:e="http://www.w3.org/2003/05/soap-envelope"
            xmlns:d="http://schemas.xmlsoap.org/ws/2005/04/discovery">
  <e:Body>
    <d:ProbeMatches>
      <d:ProbeMatch>
        <d:XAddrs>http://192.168.1.50:2020/onvif/device_service</d:XAddrs>
      </d:ProbeMatch>
      <d:ProbeMatch>
        <d:XAddrs>http://192.168.1.51:2020/onvif/device_service</d:XAddrs>
      </d:ProbeMatch>
    </d:ProbeMatches>
  </e:Body>
</e:Envelope>`

func TestParseProbeMatch(t *testing.T) {
	addrs, err := parseProbeMatch([]byte(sampleProbeMatch))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(addrs) != 2 {
		t.Fatalf("expected 2 addrs, got %d", len(addrs))
	}
}

func TestUniqueDevices(t *testing.T) {
	devs := []Device{
		{Address: "http://a", Host: "a"},
		{Address: "http://b", Host: "b"},
		{Address: "http://a", Host: "a"},
	}
	out := uniqueDevices(devs)
	if len(out) != 2 {
		t.Fatalf("expected 2 unique devices, got %d", len(out))
	}
}

func TestHostPort(t *testing.T) {
	if hp := hostPort("http://10.0.0.1:2020/onvif"); hp != "10.0.0.1:2020" {
		t.Fatalf("unexpected hostPort %s", hp)
	}
	if hp := hostPort("nonsense"); hp != "" {
		t.Fatalf("expected empty for bad url, got %s", hp)
	}
}

func TestDiscoverTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := Discover(ctx, 10*time.Millisecond)
	// Should return either nil or timeout error; we only care that it doesn't hang.
	if err != nil && !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseProbeMatchError(t *testing.T) {
	_, err := parseProbeMatch([]byte("garbage"))
	if err == nil {
		t.Fatalf("expected error for bad xml")
	}
}
