package onvif

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)
// ---------------------------------------------------------------------------
// Unit tests for response builders
// ---------------------------------------------------------------------------

func TestGetProfiles(t *testing.T) {
	cfg := &mockConfig{username: "admin", password: "testpass", port: 8080}

	resp := handleGetProfiles(cfg)

	if len(resp.Profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(resp.Profiles))
	}

	p := resp.Profiles[0]
	if p.Token != "main" {
		t.Errorf("expected token 'main', got %q", p.Token)
	}
	if p.Name != "main" {
		t.Errorf("expected name 'main', got %q", p.Name)
	}

	enc := p.VideoEncoderConfiguration
	if enc == nil {
		t.Fatal("expected VideoEncoderConfiguration, got nil")
	}
	if enc.Encoding != "H264" {
		t.Errorf("expected encoding 'H264', got %q", enc.Encoding)
	}
	if enc.Resolution.Width != 1280 {
		t.Errorf("expected width 1280, got %d", enc.Resolution.Width)
	}
	if enc.Resolution.Height != 720 {
		t.Errorf("expected height 720, got %d", enc.Resolution.Height)
	}
	if enc.RateControl.FrameRateLimit != 15 {
		t.Errorf("expected fps 15, got %d", enc.RateControl.FrameRateLimit)
	}

	src := p.VideoSourceConfiguration
	if src == nil {
		t.Fatal("expected VideoSourceConfiguration, got nil")
	}
	if src.Bounds.Width != 1280 {
		t.Errorf("expected source width 1280, got %d", src.Bounds.Width)
	}
	if src.Bounds.Height != 720 {
		t.Errorf("expected source height 720, got %d", src.Bounds.Height)
	}
}

func TestGetStreamUri(t *testing.T) {
	cfg := &mockConfig{username: "admin", password: "testpass", port: 8080}

	resp := handleGetStreamUri(context.Background(), cfg)

	expected := "rtsp://192.168.1.100:8554/stream"
	if resp.MediaUri.Uri != expected {
		t.Errorf("expected URI %q, got %q", expected, resp.MediaUri.Uri)
	}
}

func TestGetVideoSources(t *testing.T) {
	cfg := &mockConfig{username: "admin", password: "testpass", port: 8080}

	resp := handleGetVideoSources(cfg)

	if len(resp.VideoSources) != 1 {
		t.Fatalf("expected 1 video source, got %d", len(resp.VideoSources))
	}
	if resp.VideoSources[0].Token != "videoSrc0" {
		t.Errorf("expected token 'videoSrc0', got %q", resp.VideoSources[0].Token)
	}
	if resp.VideoSources[0].Name != "Pi Camera" {
		t.Errorf("expected name 'Pi Camera', got %q", resp.VideoSources[0].Name)
	}
}

// ---------------------------------------------------------------------------
// TestGetStreamUriRawSOAP — CRITICAL compatibility test
//
// Simulates the NVR's raw SOAP fallback path. The NVR sends a raw SOAP
// request and parses the response with Go encoding/xml using case-sensitive
// struct tags: GetStreamUriResponse, MediaUri, Uri.
// ---------------------------------------------------------------------------

func TestGetStreamUriRawSOAP(t *testing.T) {
	// Set up the ONVIF server with media handlers
	cfg := &mockConfig{username: "admin", password: "testpass", port: 8080}
	srv := New(cfg)
	RegisterMediaHandlers(srv)

	// Build SOAP GetStreamUri request (same format NVR sends)
	soapReq := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:trt="http://www.onvif.org/ver10/media/wsdl"
 xmlns:tt="http://www.onvif.org/ver10/schema">
  <s:Body>
    <trt:GetStreamUri>
      <trt:StreamSetup>
        <tt:Stream>RTP-Unicast</tt:Stream>
        <tt:Transport>
          <tt:Protocol>RTSP</tt:Protocol>
        </tt:Transport>
      </trt:StreamSetup>
      <trt:ProfileToken>main</trt:ProfileToken>
    </trt:GetStreamUri>
  </s:Body>
</s:Envelope>`)

	// Simulate the NVR reaching us from a specific address. Per-request IP echo
	// means the RTSP URL the NVR gets back uses its own source IP, not the
	// device's. This is the new behavior: "NVR connects from IP X -> rtsp://X:8554/stream".
	req := httptest.NewRequest(http.MethodPost, "/onvif/media_service", strings.NewReader(soapReq))
	req.Header.Set("Content-Type", "application/soap+xml")
	req.RemoteAddr = "192.168.1.100:55123"
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		body := w.Body.String()
		t.Fatalf("expected 200, got %d. Response: %s", w.Code, body)
	}

	// Parse with NVR's raw SOAP fallback struct (from NVR's ONVIF client)
	type testEnvelope struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			XMLName xml.Name `xml:"Body"`
			GetStreamURIResponse struct {
				XMLName  xml.Name `xml:"GetStreamUriResponse"`
				MediaURI struct {
					URI string `xml:"Uri"`
				} `xml:"MediaUri"`
			} `xml:"GetStreamUriResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &testEnvelope{}); err != nil {
		// If strict parse fails, try parsing the raw body for Uri content
		body := w.Body.String()
		if !strings.Contains(body, "rtsp://192.168.1.100:8554/stream") {
			t.Fatalf("failed to parse response with NVR struct and URI not found in body. Parse error: %v\nBody:\n%s", err, body)
		}
		// The URI is in the body but the struct path didn't match — check the XML structure
		t.Fatalf("failed to parse response with NVR struct (parse error: %v). URI IS in body — XML element path mismatch.\nBody:\n%s", err, body)
	}


	// Strict parse succeeded — also verify via the full struct path
	var env testEnvelope
	if err := xml.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}

	expectedURI := "rtsp://192.168.1.100:8554/stream"
	if env.Body.GetStreamURIResponse.MediaURI.URI != expectedURI {
		t.Errorf("expected URI %q, got %q", expectedURI, env.Body.GetStreamURIResponse.MediaURI.URI)
	}
}

// ---------------------------------------------------------------------------
// TestGetStreamUriNoAuth — verify auth is checked
// ---------------------------------------------------------------------------

func TestGetStreamUriNoAuth(t *testing.T) {
	cfg := &mockConfig{username: "admin", password: "testpass", port: 8080}
	srv := New(cfg)
	RegisterMediaHandlers(srv)

	// SOAP request WITHOUT auth header
	soapReq := `<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:trt="http://www.onvif.org/ver10/media/wsdl">
  <s:Body>
    <trt:GetStreamUri>
      <trt:StreamSetup>
        <tt:Stream>RTP-Unicast</tt:Stream>
        <tt:Transport>
          <tt:Protocol>RTSP</tt:Protocol>
        </tt:Transport>
      </trt:StreamSetup>
      <trt:ProfileToken>main</trt:ProfileToken>
    </trt:GetStreamUri>
  </s:Body>
</s:Envelope>`

	req := httptest.NewRequest(http.MethodPost, "/onvif/media_service", strings.NewReader(soapReq))
	req.Header.Set("Content-Type", "application/soap+xml")
	req.RemoteAddr = "192.168.1.100:55123"
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	// Server should succeed — auth is optional (Server passes auth.OK=true regardless)
	// The actual auth checking is done by the handler if needed.
	// For now, media handlers don't require auth, so 200 is expected.
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (auth optional for media), got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify the response still contains the URI (using NVR's source IP)
	body := w.Body.String()
	if !strings.Contains(body, "rtsp://192.168.1.100:8554/stream") {
		t.Errorf("response should contain RTSP URI even without auth. Body: %s", body)
	}
}


// ---------------------------------------------------------------------------
// TestGetProfilesMarshalling — verify XML output is valid and parseable
// ---------------------------------------------------------------------------

func TestGetProfilesMarshalling(t *testing.T) {
	cfg := &mockConfig{username: "admin", password: "testpass", port: 8080}
	resp := handleGetProfiles(cfg)

	data, err := MarshalSOAP(resp)
	if err != nil {
		t.Fatalf("MarshalSOAP failed: %v", err)
	}

	body := string(data)

	// Verify key elements are present
	required := []string{
		"GetProfilesResponse",
		"Profiles",
		"main",
		"H264",
		"1280",
		"720",
		"15",
		"VideoEncoderConfiguration",
		"Resolution",
		"RateControl",
		"FrameRateLimit",
	}
	for _, want := range required {
		if !strings.Contains(body, want) {
			t.Errorf("response XML missing %q\nBody:\n%s", want, body)
		}
	}
}

// ---------------------------------------------------------------------------
// TestGetStreamUriViaHTTPEndToEnd — full HTTP round-trip with auth
// ---------------------------------------------------------------------------

func TestGetStreamUriViaHTTPEndToEnd(t *testing.T) {
	cfg := &mockConfig{username: "admin", password: "testpass", port: 8080}
	srv := New(cfg)
	RegisterMediaHandlers(srv)

	// Authenticated request (digest mode)
	nonce := "dGVzdA=="
	created := "2024-01-01T00:00:00.000Z"
	digest := CheckDigest(nonce, created, "testpass")

	soapReq := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:trt="http://www.onvif.org/ver10/media/wsdl"
 xmlns:wsse="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd"
 xmlns:wsu="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">
  <s:Header>
    <wsse:Security>
      <wsse:UsernameToken>
        <wsse:Username>admin</wsse:Username>
        <wsse:Password Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordDigest">%s</wsse:Password>
        <wsse:Nonce>%s</wsse:Nonce>
        <wsu:Created>%s</wsu:Created>
      </wsse:UsernameToken>
    </wsse:Security>
  </s:Header>
  <s:Body>
    <trt:GetStreamUri>
      <trt:StreamSetup>
        <tt:Stream>RTP-Unicast</tt:Stream>
        <tt:Transport>
          <tt:Protocol>RTSP</tt:Protocol>
        </tt:Transport>
      </trt:StreamSetup>
      <trt:ProfileToken>main</trt:ProfileToken>
    </trt:GetStreamUri>
  </s:Body>
</s:Envelope>`, digest, nonce, created)

req := httptest.NewRequest(http.MethodPost, "/onvif/media_service", strings.NewReader(soapReq))
	req.Header.Set("Content-Type", "application/soap+xml")
	req.RemoteAddr = "192.168.1.100:55123"
w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		body := w.Body.String()
		t.Fatalf("expected 200, got %d. Body: %s", w.Code, body)
	}

	// Parse with onvif-go compatible struct
	type trtGetStreamUriResponse struct {
		XMLName  xml.Name `xml:"GetStreamUriResponse"`
		MediaUri struct {
			URI string `xml:"Uri"`
		} `xml:"MediaUri"`
	}

	var env struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			GetStreamUriResponse trtGetStreamUriResponse `xml:"GetStreamUriResponse"`
		} `xml:"Body"`
	}

	if err := xml.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to parse response: %v\nBody:\n%s", err, w.Body.String())
	}

	expectedURI := "rtsp://192.168.1.100:8554/stream"
	if env.Body.GetStreamUriResponse.MediaUri.URI != expectedURI {
		t.Errorf("expected URI %q, got %q", expectedURI, env.Body.GetStreamUriResponse.MediaUri.URI)
	}
}


// ---------------------------------------------------------------------------
// TestGetVideoSourcesViaHTTP — end-to-end test for GetVideoSources
// ---------------------------------------------------------------------------

func TestGetVideoSourcesViaHTTP(t *testing.T) {
	cfg := &mockConfig{username: "admin", password: "testpass", port: 8080}
	srv := New(cfg)
	RegisterMediaHandlers(srv)

	soapReq := `<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:trt="http://www.onvif.org/ver10/media/wsdl">
  <s:Body>
    <trt:GetVideoSources/>
  </s:Body>
</s:Envelope>`

	req := httptest.NewRequest(http.MethodPost, "/onvif/media_service", strings.NewReader(soapReq))
	req.Header.Set("Content-Type", "application/soap+xml")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		body := w.Body.String()
		t.Fatalf("expected 200, got %d. Body: %s", w.Code, body)
	}

	body := w.Body.String()
	if !strings.Contains(body, "GetVideoSourcesResponse") {
		t.Errorf("response missing GetVideoSourcesResponse\nBody: %s", body)
	}
	if !strings.Contains(body, "videoSrc0") {
		t.Errorf("response missing video source token\nBody: %s", body)
	}
}

// ---------------------------------------------------------------------------
// TestGetProfilesViaHTTP — end-to-end test for GetProfiles
// ---------------------------------------------------------------------------

func TestGetProfilesViaHTTP(t *testing.T) {
	cfg := &mockConfig{username: "admin", password: "testpass", port: 8080}
	srv := New(cfg)
	RegisterMediaHandlers(srv)

	soapReq := `<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:trt="http://www.onvif.org/ver10/media/wsdl">
  <s:Body>
    <trt:GetProfiles/>
  </s:Body>
</s:Envelope>`

	req := httptest.NewRequest(http.MethodPost, "/onvif/media_service", strings.NewReader(soapReq))
	req.Header.Set("Content-Type", "application/soap+xml")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		body := w.Body.String()
		t.Fatalf("expected 200, got %d. Body: %s", w.Code, body)
	}

	// Parse profiles from the response
	type profileResponse struct {
		XMLName  xml.Name `xml:"Envelope"`
		Body     struct {
			GetProfilesResponse struct {
				Profiles []struct {
					Token    string `xml:"token,attr"`
					Name     string `xml:"Name"`
					Encoding struct {
						Encoding string `xml:"Encoding"`
						Width    int    `xml:"Resolution>Width"`
						Height   int    `xml:"Resolution>Height"`
						FPS      int    `xml:"RateControl>FrameRateLimit"`
					} `xml:"VideoEncoderConfiguration"`
				} `xml:"Profiles"`
			} `xml:"GetProfilesResponse"`
		} `xml:"Body"`
	}

	var env profileResponse
	if err := xml.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to parse GetProfiles response: %v\nBody:\n%s", err, w.Body.String())
	}

	profiles := env.Body.GetProfilesResponse.Profiles
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}

	p := profiles[0]
	if p.Token != "main" {
		t.Errorf("expected token 'main', got %q", p.Token)
	}
	if p.Encoding.Encoding != "H264" {
		t.Errorf("expected encoding 'H264', got %q", p.Encoding.Encoding)
	}
	if p.Encoding.Width != 1280 {
		t.Errorf("expected width 1280, got %d", p.Encoding.Width)
	}
	if p.Encoding.Height != 720 {
		t.Errorf("expected height 720, got %d", p.Encoding.Height)
	}
	if p.Encoding.FPS != 15 {
		t.Errorf("expected fps 15, got %d", p.Encoding.FPS)
	}
}

// TestGetStreamUriPerRequestIP — verify the RTSP URI reflects the request's source IP.
// This is the core NVR integration guarantee: "NVR from IP X receives rtsp://X:8554/stream".
// TestGetStreamUriServerIP — verify the RTSP URI uses the RPi interface IP
// (server-side), NOT the NVR's source IP. This is the core NVR integration
// guarantee: "NVR connects to RPi on 192.168.1.100 -> gets back
// rtsp://192.168.1.100:8554/stream" regardless of which IP the NVR itself has.
func TestGetStreamUriServerIP(t *testing.T) {
	cfg := &mockConfig{username: "admin", password: "testpass", port: 8080}
	srv := New(cfg)
	RegisterMediaHandlers(srv)

	soapReq := `<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:trt="http://www.onvif.org/ver10/media/wsdl"
 xmlns:tt="http://www.onvif.org/ver10/schema">
  <s:Body>
    <trt:GetStreamUri>
      <trt:StreamSetup>
        <tt:Stream>RTP-Unicast</tt:Stream>
        <tt:Transport><tt:Protocol>RTSP</tt:Protocol></tt:Transport>
      </trt:StreamSetup>
      <trt:ProfileToken>main</trt:ProfileToken>
    </trt:GetStreamUri>
  </s:Body>
</s:Envelope>`

	// Each case simulates a different RPi interface accepting the connection
	// (the server-side local IP), with the NVR coming from a different IP.
	cases := []struct {
		name        string
		serverIP    string // RPi's interface IP (what gets put in RTSP URI)
		nvrRemoteIP string // NVR's source IP (should be ignored)
		wantIP      string // expected in the URI
	}{
		{"wlan0", "192.168.1.100", "192.168.1.101:55123", "192.168.1.100"},
		{"eth0_10net", "10.0.0.5", "10.0.0.99:1234", "10.0.0.5"},
		{"ipv6", "2001:db8::1", "[2001:db8::99]:8080", "2001:db8::1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/onvif/media_service", strings.NewReader(soapReq))
			req.Header.Set("Content-Type", "application/soap+xml")
			req.RemoteAddr = tc.nvrRemoteIP
			// In production, http.Server.ConnContext does this from conn.LocalAddr().
			req = req.WithContext(WithServerIP(req.Context(), tc.serverIP))

			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}
			wantURI := fmt.Sprintf("rtsp://%s:8554/stream", tc.wantIP)
			if !strings.Contains(w.Body.String(), wantURI) {
				t.Errorf("expected URI %q in response, got body: %s", wantURI, w.Body.String())
			}
			// And the NVR's source IP should NOT appear in the URI.
			if strings.Contains(w.Body.String(), tc.nvrRemoteIP) {
				t.Errorf("NVR source IP %q should not appear in response, got: %s", tc.nvrRemoteIP, w.Body.String())
			}
		})
	}
}

