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

	"gopkg.in/yaml.v3"
)

// mockOnvifConfig implements OnvifConfigProvider for testing.
type mockOnvifConfig struct {
	port     int
	username string
	password string
}

func (m *mockOnvifConfig) ONVIFPort() int           { return m.port }
func (m *mockOnvifConfig) ONVIFUsername() string    { return m.username }
func (m *mockOnvifConfig) ONVIFPassword() string    { return m.password }
func (m *mockOnvifConfig) RTSPPort() int            { return 8554 }
func (m *mockOnvifConfig) DeviceIP() string         { return "192.168.1.1" }

func TestSaveConfigPreservesAllSections(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write a multi-section config file.
	initialYAML := `camera:
  device: /dev/video0
  width: 1280
  height: 720
  fps: 15
  codec: h264
  bitrate: 2000000
rtsp:
  port: 8554
  username: ""
  password: ""
onvif:
  port: 8080
  username: old-admin
  password: old-secret
rtmp:
  enabled: true
  url: rtmp://example.com/live
device:
  name: Test Camera
  manufacturer: Test Manufacturer
  model: Test Model
  firmware: 1.0.0
  hardware_id: TEST-001
  serial_number: SN12345
logging:
  level: debug
web:
  enabled: true
  port: 8088
  username: web-user
  password: web-pass
`
	if err := os.WriteFile(configPath, []byte(initialYAML), 0600); err != nil {
		t.Fatal(err)
	}

	s := &Server{
		cfg: Config{
			ConfigPath: configPath,
			OnvifConfig: &mockOnvifConfig{
				port:     8080,
				username: "old-admin",
				password: "old-secret",
			},
		},
		logger: log.New(io.Discard, "", 0),
	}

	// POST new ONVIF credentials.
	body := `{"username":"new-admin","password":"new-secret"}`
	req := httptest.NewRequest("POST", "/api/config/onvif", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.handlePostConfigOnvif(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Read back the config file.
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse saved config: %v", err)
	}

	// Verify all sections are present.
	sections := []string{"camera", "rtsp", "onvif", "rtmp", "device", "logging", "web"}
	for _, sec := range sections {
		if _, ok := result[sec]; !ok {
			t.Errorf("section %q is missing after save", sec)
		}
	}

	// Verify onvif section was updated.
	onvif, ok := result["onvif"].(map[string]interface{})
	if !ok {
		t.Fatal("onvif section is not a map")
	}
	if onvif["username"] != "new-admin" {
		t.Errorf("expected username 'new-admin', got %v", onvif["username"])
	}
	if onvif["password"] != "new-secret" {
		t.Errorf("expected password 'new-secret', got %v", onvif["password"])
	}
	if onvif["port"] != 8080 {
		t.Errorf("expected port 8080, got %v", onvif["port"])
	}

	// Verify other sections are intact.
	rtmp, ok := result["rtmp"].(map[string]interface{})
	if !ok {
		t.Fatal("rtmp section is not a map")
	}
	if rtmp["enabled"] != true {
		t.Errorf("expected rtmp.enabled=true, got %v", rtmp["enabled"])
	}
	if rtmp["url"] != "rtmp://example.com/live" {
		t.Errorf("expected rtmp.url unchanged, got %v", rtmp["url"])
	}

	web, ok := result["web"].(map[string]interface{})
	if !ok {
		t.Fatal("web section is not a map")
	}
	if web["port"] != 8088 {
		t.Errorf("expected web.port 8088, got %v", web["port"])
	}
}

func TestSaveConfigAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	initialYAML := `camera:
  device: /dev/video0
onvif:
  port: 8080
  username: admin
  password: secret
`
	if err := os.WriteFile(configPath, []byte(initialYAML), 0600); err != nil {
		t.Fatal(err)
	}

	s := &Server{
		cfg: Config{
			ConfigPath: configPath,
			OnvifConfig: &mockOnvifConfig{
				port:     8080,
				username: "admin",
				password: "secret",
			},
		},
		logger: log.New(io.Discard, "", 0),
	}

	body := `{"username":"new","password":"newpass"}`
	req := httptest.NewRequest("POST", "/api/config/onvif", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.handlePostConfigOnvif(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// List directory — should only contain config.yaml (no leftover temp files).
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range entries {
		if e.Name() != "config.yaml" {
			t.Errorf("unexpected file left in config directory: %s", e.Name())
		}
	}
	if len(entries) != 1 {
		t.Errorf("expected exactly 1 file (config.yaml), got %d", len(entries))
	}

	// Verify the config file is valid YAML and contains updated credentials.
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		t.Fatalf("saved config is not valid YAML: %v", err)
	}
	onvif, ok := result["onvif"].(map[string]interface{})
	if !ok {
		t.Fatal("onvif section missing")
	}
	if onvif["username"] != "new" {
		t.Errorf("expected username 'new', got %v", onvif["username"])
	}
}

func TestSaveConfigHandlesMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yaml")

	s := &Server{
		cfg: Config{
			ConfigPath: configPath,
			OnvifConfig: &mockOnvifConfig{
				port:     8080,
				username: "admin",
				password: "secret",
			},
		},
		logger: log.New(io.Discard, "", 0),
	}

	body := `{"username":"new","password":"newpass"}`
	req := httptest.NewRequest("POST", "/api/config/onvif", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.handlePostConfigOnvif(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for missing file, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSaveConfigHandlesInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(configPath, []byte("not: valid: yaml: [[["), 0600); err != nil {
		t.Fatal(err)
	}

	s := &Server{
		cfg: Config{
			ConfigPath: configPath,
			OnvifConfig: &mockOnvifConfig{
				port:     8080,
				username: "admin",
				password: "secret",
			},
		},
		logger: log.New(io.Discard, "", 0),
	}

	body := `{"username":"new","password":"newpass"}`
	req := httptest.NewRequest("POST", "/api/config/onvif", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.handlePostConfigOnvif(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for invalid YAML, got %d: %s", rr.Code, rr.Body.String())
	}
}
