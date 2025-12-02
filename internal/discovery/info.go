package discovery

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DeviceInfo holds basic ONVIF device metadata.
type DeviceInfo struct {
	Manufacturer string
	Model        string
	Firmware     string
	Serial       string
	HardwareID   string
}

// FetchDeviceInfo tries WS-Security UsernameToken first, then falls back to HTTP Basic.
func FetchDeviceInfo(ctx context.Context, xaddr, user, pass string) (DeviceInfo, error) {
	if xaddr == "" {
		return DeviceInfo{}, fmt.Errorf("xaddr required")
	}
	soapBody := `<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
  <s:Header>%s</s:Header>
  <s:Body>
    <tds:GetDeviceInformation xmlns:tds="http://www.onvif.org/ver10/device/wsdl"/>
  </s:Body>
</s:Envelope>`

	client := &http.Client{Timeout: 5 * time.Second}

	// WS-Security attempt when creds present.
	if user != "" {
		header := wsSecurityHeader(user, pass)
		body := fmt.Sprintf(soapBody, header)
		info, err := doInfoRequest(ctx, client, xaddr, body, "")
		if err == nil {
			return info, nil
		}
		if !isAuthError(err) {
			return info, err
		}
	}

	// Basic or anonymous
	body := fmt.Sprintf(soapBody, "")
	return doInfoRequest(ctx, client, xaddr, body, basicAuth(user, pass))
}

type infoEnvelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    infoBody `xml:"Body"`
}

type infoBody struct {
	DeviceInformation deviceInformation `xml:"GetDeviceInformationResponse"`
}

type deviceInformation struct {
	Manufacturer    string `xml:"Manufacturer"`
	Model           string `xml:"Model"`
	FirmwareVersion string `xml:"FirmwareVersion"`
	SerialNumber    string `xml:"SerialNumber"`
	HardwareID      string `xml:"HardwareId"`
}

func doInfoRequest(ctx context.Context, client *http.Client, url, body, authHeader string) (DeviceInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(body))
	if err != nil {
		return DeviceInfo{}, err
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := client.Do(req)
	if err != nil {
		return DeviceInfo{}, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return DeviceInfo{}, fmt.Errorf("auth failed (status %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return DeviceInfo{}, fmt.Errorf("device info status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var envelope infoEnvelope
	if err := xml.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return DeviceInfo{}, fmt.Errorf("decode device info: %w", err)
	}

	return DeviceInfo{
		Manufacturer: strings.TrimSpace(envelope.Body.DeviceInformation.Manufacturer),
		Model:        strings.TrimSpace(envelope.Body.DeviceInformation.Model),
		Firmware:     strings.TrimSpace(envelope.Body.DeviceInformation.FirmwareVersion),
		Serial:       strings.TrimSpace(envelope.Body.DeviceInformation.SerialNumber),
		HardwareID:   strings.TrimSpace(envelope.Body.DeviceInformation.HardwareID),
	}, nil
}

func wsSecurityHeader(user, pass string) string {
	nonce := make([]byte, 16)
	_, _ = rand.Read(nonce)
	nonceB64 := base64.StdEncoding.EncodeToString(nonce)
	created := time.Now().UTC().Format(time.RFC3339Nano)

	h := sha1.New() //nolint:gosec
	h.Write(nonce)
	h.Write([]byte(created))
	h.Write([]byte(pass))
	passwordDigest := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return fmt.Sprintf(`
<wsse:Security xmlns:wsse="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd"
               xmlns:wsu="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">
  <wsse:UsernameToken>
    <wsse:Username>%s</wsse:Username>
    <wsse:Password Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordDigest">%s</wsse:Password>
    <wsse:Nonce>%s</wsse:Nonce>
    <wsu:Created>%s</wsu:Created>
  </wsse:UsernameToken>
</wsse:Security>`, escapeXML(user), passwordDigest, nonceB64, created)
}

func basicAuth(user, pass string) string {
	if user == "" {
		return ""
	}
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

func escapeXML(s string) string {
	replacer := strings.NewReplacer(
		`&`, "&amp;",
		`<`, "&lt;",
		`>`, "&gt;",
		`"`, "&quot;",
		`'`, "&apos;",
	)
	return replacer.Replace(s)
}

func isAuthError(err error) bool {
	return strings.Contains(err.Error(), "auth failed")
}
