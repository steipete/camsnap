package discovery

import (
	"context"
	"encoding/xml"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"strings"
	"time"
)

const (
	wsdAddr       = "239.255.255.250:3702"
	probeTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<e:Envelope xmlns:e="http://www.w3.org/2003/05/soap-envelope"
            xmlns:w="http://schemas.xmlsoap.org/ws/2004/08/addressing"
            xmlns:d="http://schemas.xmlsoap.org/ws/2005/04/discovery">
  <e:Header>
    <w:MessageID>uuid:%s</w:MessageID>
    <w:To>urn:schemas-xmlsoap-org:ws:2005:04:discovery</w:To>
    <w:Action>http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe</w:Action>
  </e:Header>
  <e:Body>
    <d:Probe>
      <d:Types>dn:NetworkVideoTransmitter</d:Types>
    </d:Probe>
  </e:Body>
</e:Envelope>`
)

// Device represents a discovered device.
type Device struct {
	Address string // full XAddr
	Host    string // host:port extracted from XAddr
	Model   string
	FW      string
}

// Discover performs a WS-Discovery probe for ONVIF devices.
func Discover(ctx context.Context, timeout time.Duration) ([]Device, error) {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	deadline := time.Now().Add(timeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}

	localAddr, err := net.ResolveUDPAddr("udp4", ":0")
	if err != nil {
		return nil, fmt.Errorf("resolve udp: %w", err)
	}
	remoteAddr, err := net.ResolveUDPAddr("udp4", wsdAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve multicast: %w", err)
	}

	conn, err := net.ListenUDP("udp4", localAddr)
	if err != nil {
		return nil, fmt.Errorf("listen udp: %w", err)
	}
	defer conn.Close()

	msgID := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		rand.Uint32(),
		rand.Uint32()&0xffff,
		rand.Uint32()&0xffff,
		rand.Uint32()&0xffff,
		rand.Uint64()&0xffffffffffff,
	)
	probe := fmt.Sprintf(probeTemplate, msgID)

	if _, err := conn.WriteToUDP([]byte(probe), remoteAddr); err != nil {
		return nil, fmt.Errorf("send probe: %w", err)
	}

	if err := conn.SetReadDeadline(deadline); err != nil {
		return nil, fmt.Errorf("set deadline: %w", err)
	}

	var devices []Device
	buf := make([]byte, 8192)
	for {
		if ctx.Err() != nil {
			break
		}
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				break
			}
			return devices, fmt.Errorf("read udp: %w", err)
		}
		matches, parseErr := parseProbeMatch(buf[:n])
		if parseErr != nil {
			// ignore malformed responses
			continue
		}
		for _, addr := range matches {
			devices = append(devices, Device{
				Address: addr,
				Host:    hostPort(addr),
			})
		}
	}

	return uniqueDevices(devices), nil
}

type probeMatches struct {
	XMLName xml.Name       `xml:"Envelope"`
	Body    probeMatchBody `xml:"Body"`
}

type probeMatchBody struct {
	Matches []singleMatch `xml:"ProbeMatches>ProbeMatch"`
}

type singleMatch struct {
	XAddrs string `xml:"XAddrs"`
}

func parseProbeMatch(data []byte) ([]string, error) {
	var env probeMatches
	if err := xml.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	var addrs []string
	for _, m := range env.Body.Matches {
		for _, addr := range strings.Fields(m.XAddrs) {
			addrs = append(addrs, addr)
		}
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no addresses")
	}
	return addrs, nil
}

func uniqueDevices(devs []Device) []Device {
	seen := make(map[string]struct{})
	out := make([]Device, 0, len(devs))
	for _, d := range devs {
		key := d.Host
		if key == "" {
			key = d.Address
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, d)
	}
	return out
}

func hostPort(addr string) string {
	u, err := url.Parse(addr)
	if err != nil {
		return ""
	}
	if u.Host != "" {
		return u.Host
	}
	return ""
}
