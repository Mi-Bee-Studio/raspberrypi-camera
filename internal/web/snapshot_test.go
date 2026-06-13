package web

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mi-Bee-Studio/mibee-eye-raspi/internal/h264"
	"github.com/Mi-Bee-Studio/mibee-eye-raspi/internal/onvif"
)

func TestHandleGetSnapshot_NoBuffer(t *testing.T) {
	s := &Server{
		cfg:    Config{Snapshot: nil},
		logger: log.New(io.Discard, "", 0),
	}
	req := httptest.NewRequest("GET", "/api/snapshot", nil)
	w := httptest.NewRecorder()
	s.handleGetSnapshot(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "not configured") {
		t.Errorf("expected 'not configured' message, got %q", w.Body.String())
	}
}

func TestHandleGetSnapshot_NoFrame(t *testing.T) {
	buf := onvif.NewSnapshotBuffer()
	s := &Server{
		cfg:    Config{Snapshot: buf},
		logger: log.New(io.Discard, "", 0),
	}
	req := httptest.NewRequest("GET", "/api/snapshot", nil)
	w := httptest.NewRecorder()
	s.handleGetSnapshot(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "no frame received") {
		t.Errorf("expected 'no frame received' message, got %q", w.Body.String())
	}
}

func TestHandleGetSnapshot_Success(t *testing.T) {
	// Create a SnapshotBuffer with a pre-fed key frame.
	buf := onvif.NewSnapshotBuffer()
	buf.Feed(h264.AccessUnit{
		NALUs: []h264.NALU{
			{Type: 7, Data: []byte("mock-sps"), IsSPS: true},
			{Type: 8, Data: []byte("mock-pps"), IsPPS: true},
			{Type: 5, Data: []byte("mock-idr-data"), IsIDR: true},
		},
		Timestamp: time.Now(),
		KeyFrame:  true,
	})

	// Write a mock ffmpeg script that outputs a minimal JPEG.
	mockDir := t.TempDir()
	mockFFmpeg := filepath.Join(mockDir, "ffmpeg")

	// Minimal JPEG: SOI + JFIF header + EOI.
	jpegData := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46,
		0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01,
		0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9,
	}
	jpegPath := filepath.Join(mockDir, "test.jpg")
	if err := os.WriteFile(jpegPath, jpegData, 0644); err != nil {
		t.Fatal(err)
	}

	// Shell script: cat stdin to /dev/null, then cat the JPEG to stdout.
	script := "#!/bin/sh\ncat > /dev/null\ncat " + jpegPath + "\n"
	if err := os.WriteFile(mockFFmpeg, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	// Prepend mock dir to PATH so "ffmpeg" resolves to our mock.
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", mockDir+string(filepath.ListSeparator)+origPath)

	s := &Server{
		cfg:    Config{Snapshot: buf},
		logger: log.New(io.Discard, "", 0),
	}
	req := httptest.NewRequest("GET", "/api/snapshot", nil)
	w := httptest.NewRecorder()
	s.handleGetSnapshot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if ct != "image/jpeg" {
		t.Errorf("expected Content-Type image/jpeg, got %q", ct)
	}

	body := w.Body.Bytes()
	if len(body) < 2 || body[0] != 0xFF || body[1] != 0xD8 {
		t.Error("expected JPEG SOI marker in response body")
	}

	cacheControl := w.Header().Get("Cache-Control")
	if !strings.Contains(cacheControl, "no-cache") {
		t.Errorf("expected Cache-Control with no-cache, got %q", cacheControl)
	}
}
