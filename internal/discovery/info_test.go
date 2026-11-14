package discovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchDeviceInfoUsesCallerTimeout(t *testing.T) {
	const body = `<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
  <s:Body>
    <tds:GetDeviceInformationResponse>
      <tds:Manufacturer>Acme</tds:Manufacturer>
      <tds:Model>Cam100</tds:Model>
      <tds:FirmwareVersion>1.2.3</tds:FirmwareVersion>
    </tds:GetDeviceInformationResponse>
  </s:Body>
</s:Envelope>`
	const responseDelay = 6 * time.Second
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timer := time.NewTimer(responseDelay)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
			return
		case <-timer.C:
		}
		w.Header().Set("Content-Type", "application/soap+xml")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	info, err := FetchDeviceInfo(ctx, srv.URL, "", "")
	if err != nil {
		t.Fatalf("FetchDeviceInfo() error = %v", err)
	}
	if info.Model != "Cam100" || info.Manufacturer != "Acme" || info.Firmware != "1.2.3" {
		t.Fatalf("info = %+v", info)
	}
}
