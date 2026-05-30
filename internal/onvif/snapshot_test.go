package onvif

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Mi-Bee-Studio/raspberrypi-camera/internal/h264"
)

// ---------------------------------------------------------------------------
// Unit tests — response builders
// ---------------------------------------------------------------------------

func TestGetSnapshotUri(t *testing.T) {
	cfg := &mockConfig{username: "admin", password: "testpass", port: 8080}

	resp := handleGetSnapshotUri(cfg)

	if !strings.Contains(resp.MediaUri.Uri, "/snapshot") {
		t.Errorf("expected URI to contain '/snapshot', got %q", resp.MediaUri.Uri)
	}
	expected := "http://192.168.1.100:8080/snapshot"
	if resp.MediaUri.Uri != expected {
		t.Errorf("expected URI %q, got %q", expected, resp.MediaUri.Uri)
	}
	if resp.MediaUri.Timeout != "PT10S" {
		t.Errorf("expected timeout PT10S, got %q", resp.MediaUri.Timeout)
	}
}

// ---------------------------------------------------------------------------
// SnapshotBuffer tests
// ---------------------------------------------------------------------------

func TestSnapshotBufferRejectsNonKeyFrames(t *testing.T) {
	buf := NewSnapshotBuffer()

	buf.Feed(h264.AccessUnit{
		NALUs:    []h264.NALU{{Type: 1, Data: []byte("non-idr")}},
		Timestamp: time.Now(),
		KeyFrame:  false,
	})

	if buf.Available() {
		t.Error("buffer should not be available after non-key frame")
	}
	if buf.Latest() != nil {
		t.Error("latest should be nil after non-key frame")
	}
}

func TestSnapshotBufferStoresKeyFrame(t *testing.T) {
	buf := NewSnapshotBuffer()

	ts := time.Now().UTC()
	buf.Feed(h264.AccessUnit{
		NALUs:    []h264.NALU{{Type: 5, Data: []byte("idr-frame-data"), IsIDR: true}},
		Timestamp: ts,
		KeyFrame:  true,
	})

	if !buf.Available() {
		t.Fatal("buffer should be available after key frame")
	}

	au := buf.Latest()
	if au == nil {
		t.Fatal("latest should not be nil")
	}
	if len(au.NALUs) != 1 {
		t.Fatalf("expected 1 NALU, got %d", len(au.NALUs))
	}
	if string(au.NALUs[0].Data) != "idr-frame-data" {
		t.Errorf("expected NALU data 'idr-frame-data', got %q", string(au.NALUs[0].Data))
	}
}

func TestSnapshotBufferOverwrites(t *testing.T) {
	buf := NewSnapshotBuffer()

	buf.Feed(h264.AccessUnit{
		NALUs:     []h264.NALU{{Type: 5, Data: []byte("first-idr"), IsIDR: true}},
		Timestamp: time.Now(),
		KeyFrame:  true,
	})
	buf.Feed(h264.AccessUnit{
		NALUs:     []h264.NALU{{Type: 5, Data: []byte("second-idr"), IsIDR: true}},
		Timestamp: time.Now(),
		KeyFrame:  true,
	})

	au := buf.Latest()
	if string(au.NALUs[0].Data) != "second-idr" {
		t.Errorf("expected latest to be 'second-idr', got %q", string(au.NALUs[0].Data))
	}
}

// ---------------------------------------------------------------------------
// HTTP endpoint tests
// ---------------------------------------------------------------------------

func TestSnapshotNotAvailable(t *testing.T) {
	buf := NewSnapshotBuffer()

	req := httptest.NewRequest(http.MethodGet, "/snapshot", nil)
	w := httptest.NewRecorder()

	handleSnapshotHTTP(w, req, buf)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "no frame received") {
		t.Errorf("expected error message about no frame, got %q", w.Body.String())
	}
}

func TestSnapshotHTTP(t *testing.T) {
	buf := NewSnapshotBuffer()

	buf.Feed(h264.AccessUnit{
		NALUs:     []h264.NALU{{Type: 5, Data: []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}, IsIDR: true}},
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		KeyFrame:  true,
	})

	req := httptest.NewRequest(http.MethodGet, "/snapshot", nil)
	w := httptest.NewRecorder()

	handleSnapshotHTTP(w, req, buf)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if ct != "image/jpeg" {
		t.Errorf("expected Content-Type image/jpeg, got %q", ct)
	}
	if w.Body.Len() == 0 {
		t.Error("expected non-empty body")
	}
	tsHeader := w.Header().Get("X-Frame-Timestamp")
	if !strings.Contains(tsHeader, "2024-01-15") {
		t.Errorf("expected timestamp header to contain date, got %q", tsHeader)
	}
}

// ---------------------------------------------------------------------------
// SOAP integration test — GetSnapshotUri via full HTTP round-trip
// ---------------------------------------------------------------------------

func TestGetSnapshotUriViaHTTPIntegration(t *testing.T) {
	cfg := &mockConfig{username: "admin", password: "testpass", port: 8080}
	srv := New(cfg)
	RegisterSnapshotHandlers(srv, make(chan h264.AccessUnit))

	soapReq := `<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:trt="http://www.onvif.org/ver10/media/wsdl">
  <s:Body>
    <trt:GetSnapshotUri>
      <trt:ProfileToken>main</trt:ProfileToken>
    </trt:GetSnapshotUri>
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

	// Parse with NVR-compatible struct
	type testEnvelope struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			GetSnapshotUriResponse struct {
				MediaUri struct {
					URI string `xml:"Uri"`
				} `xml:"MediaUri"`
			} `xml:"GetSnapshotUriResponse"`
		} `xml:"Body"`
	}

	var env testEnvelope
	if err := xml.Unmarshal(w.Body.Bytes(), &env); err != nil {
		// If strict parse fails, check body for the URI
		body := w.Body.String()
		if !strings.Contains(body, "/snapshot") {
			t.Fatalf("failed to parse and URI not in body. Parse error: %v\nBody:\n%s", err, body)
		}
		t.Fatalf("failed to parse with NVR struct (parse error: %v). URI IS in body — XML path mismatch.\nBody:\n%s", err, body)
	}

	if !strings.Contains(env.Body.GetSnapshotUriResponse.MediaUri.URI, "/snapshot") {
		t.Errorf("expected URI to contain '/snapshot', got %q", env.Body.GetSnapshotUriResponse.MediaUri.URI)
	}
}

// ---------------------------------------------------------------------------
// Integration test — /snapshot routed through Server.ServeHTTP
// ---------------------------------------------------------------------------

func TestSnapshotViaServerServeHTTP(t *testing.T) {
	cfg := &mockConfig{username: "admin", password: "testpass", port: 8080}
	srv := New(cfg)

	// Manually set up snapshot handler with a pre-fed buffer.
	buf := NewSnapshotBuffer()
	buf.Feed(h264.AccessUnit{
		NALUs:     []h264.NALU{{Type: 5, Data: []byte("test-frame-bytes"), IsIDR: true}},
		Timestamp: time.Now(),
		KeyFrame:  true,
	})
	srv.snapshotHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleSnapshotHTTP(w, r, buf)
	})

	// GET /snapshot should be routed to the snapshot handler, not SOAP.
	req := httptest.NewRequest(http.MethodGet, "/snapshot", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if ct != "image/jpeg" {
		t.Errorf("expected Content-Type image/jpeg, got %q", ct)
	}
}

// ---------------------------------------------------------------------------
// Integration test — /snapshot unavailable through Server.ServeHTTP
// ---------------------------------------------------------------------------

func TestSnapshotUnavailableViaServerServeHTTP(t *testing.T) {
	cfg := &mockConfig{username: "admin", password: "testpass", port: 8080}
	srv := New(cfg)

	buf := NewSnapshotBuffer()
	srv.snapshotHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleSnapshotHTTP(w, r, buf)
	})

	req := httptest.NewRequest(http.MethodGet, "/snapshot", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Test GetSnapshotUri marshalling
// ---------------------------------------------------------------------------

func TestGetSnapshotUriMarshalling(t *testing.T) {
	cfg := &mockConfig{username: "admin", password: "testpass", port: 8080}
	resp := handleGetSnapshotUri(cfg)

	data, err := MarshalSOAP(resp)
	if err != nil {
		t.Fatalf("MarshalSOAP failed: %v", err)
	}

	body := string(data)
	required := []string{
		"GetSnapshotUriResponse",
		"MediaUri",
		"/snapshot",
		"PT10S",
	}
	for _, want := range required {
		if !strings.Contains(body, want) {
			t.Errorf("response XML missing %q\nBody:\n%s", want, body)
		}
	}
}

// ---------------------------------------------------------------------------
// Test SnapshotBuffer with RegisterSnapshotHandlers and live channel
// ---------------------------------------------------------------------------

func TestSnapshotHandlerFedFromChannel(t *testing.T) {
	cfg := &mockConfig{username: "admin", password: "testpass", port: 8080}
	frameCh := make(chan h264.AccessUnit, 16)

	srv := New(cfg)
	RegisterSnapshotHandlers(srv, frameCh)

	// Send a key frame through the channel.
	frameCh <- h264.AccessUnit{
		NALUs:     []h264.NALU{{Type: 5, Data: []byte("live-frame"), IsIDR: true}},
		Timestamp: time.Now(),
		KeyFrame:  true,
	}

	// Wait a bit for the goroutine to consume.
	<-time.After(50 * time.Millisecond)

	// GET /snapshot through server.
	req := httptest.NewRequest(http.MethodGet, "/snapshot", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if ct != "image/jpeg" {
		t.Errorf("expected Content-Type image/jpeg, got %q", ct)
	}
}

// ---------------------------------------------------------------------------
// Verify GetSnapshotUri response also parses with NVR-style raw SOAP
// ---------------------------------------------------------------------------

func TestGetSnapshotUriRawSOAP(t *testing.T) {
	cfg := &mockConfig{username: "admin", password: "testpass", port: 8080}
	srv := New(cfg)
	RegisterSnapshotHandlers(srv, make(chan h264.AccessUnit))

	soapReq := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:trt="http://www.onvif.org/ver10/media/wsdl">
  <s:Body>
    <trt:GetSnapshotUri>
      <trt:ProfileToken>main</trt:ProfileToken>
    </trt:GetSnapshotUri>
  </s:Body>
</s:Envelope>`)

	req := httptest.NewRequest(http.MethodPost, "/onvif/media_service", strings.NewReader(soapReq))
	req.Header.Set("Content-Type", "application/soap+xml")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "http://192.168.1.100:8080/snapshot") {
		t.Errorf("response body missing snapshot URI.\nBody:\n%s", body)
	}

	// Verify with the same struct the NVR uses
	type trtGetSnapshotUriResponse struct {
		XMLName  xml.Name `xml:"GetSnapshotUriResponse"`
		MediaUri struct {
			URI string `xml:"Uri"`
		} `xml:"MediaUri"`
	}

	var env struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			GetSnapshotUriResponse trtGetSnapshotUriResponse `xml:"GetSnapshotUriResponse"`
		} `xml:"Body"`
	}

	if err := xml.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to parse response with NVR-style struct: %v\nBody:\n%s", err, body)
	}

	expectedURI := "http://192.168.1.100:8080/snapshot"
	if env.Body.GetSnapshotUriResponse.MediaUri.URI != expectedURI {
		t.Errorf("expected URI %q, got %q", expectedURI, env.Body.GetSnapshotUriResponse.MediaUri.URI)
	}
}
