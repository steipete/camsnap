package discovery

import "testing"

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
