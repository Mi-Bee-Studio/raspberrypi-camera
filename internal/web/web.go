package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/Mi-Bee-Studio/mibee-eye-raspi/internal/camera"
	"github.com/Mi-Bee-Studio/mibee-eye-raspi/internal/hls"
	"github.com/Mi-Bee-Studio/mibee-eye-raspi/internal/onvif"
	"github.com/Mi-Bee-Studio/mibee-eye-raspi/internal/ptz"
)

// OnvifConfigProvider provides read-only access to ONVIF and RTSP configuration.
type OnvifConfigProvider interface {
	ONVIFPort() int
	ONVIFUsername() string
	ONVIFPassword() string
	RTSPPort() int
	DeviceIP() string
}

// Config holds the web server configuration.
type Config struct {
	Port        int                 // listen port (default 8088)
	Username    string              // basic-auth user (default = onvif user)
	Password    string              // basic-auth pass (default = onvif pass)
	ConfigPath  string              // path to config.yaml (used by /api/config/onvif)
	OnvifConfig OnvifConfigProvider // read-only onvif/rtsp config
	Params      *camera.ParamManager
	PTZ         *ptz.State
	Snapshot    *onvif.SnapshotBuffer
	HLS         *hls.Server // optional HLS bridge; nil disables /api/hls/*
	Logger      *log.Logger // nil -> log.Default()
}

// Server is the web UI HTTP server.
type Server struct {
	cfg    Config
	logger *log.Logger
	mux    *http.ServeMux
	hub    *wsHub
	server *http.Server

	username string
	password string
	sessions *SessionStore
}

// New creates a new web server.
func New(cfg Config) *Server {
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	username := cfg.Username
	password := cfg.Password
	if username == "" && cfg.OnvifConfig != nil {
		username = cfg.OnvifConfig.ONVIFUsername()
	}
	if password == "" && cfg.OnvifConfig != nil {
		password = cfg.OnvifConfig.ONVIFPassword()
	}

	return &Server{
		cfg:      cfg,
		logger:   logger,
		username: username,
		password: password,
		sessions: NewSessionStore(username, password),
	}
}

// Start starts the web UI HTTP server on the configured port.
func (s *Server) Start(ctx context.Context) error {
	port := s.cfg.Port
	if port == 0 {
		port = 8088
	}

	s.mux = http.NewServeMux()
	s.hub = newWSHub(s.logger)

	// Wire up hooks from ParamManager and PTZ to WebSocket hub.
	if s.cfg.Params != nil {
		s.cfg.Params.SetOnChange(func(name string, value interface{}) {
			s.hub.sendEvent(wsEvent{
				Type:  "param-changed",
				Name:  name,
				Value: value,
			})
		})
	}
	if s.cfg.PTZ != nil {
		s.cfg.PTZ.SetOnPositionChange(func(pos ptz.Position) {
			s.hub.sendEvent(wsEvent{
				Type: "ptz-position",
			})
		})
		s.cfg.PTZ.SetOnPresetListChange(func() {
			s.hub.sendEvent(wsEvent{
				Type: "preset-list-changed",
			})
		})
	}

	s.registerRoutes()

	addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(port))
	s.server = &http.Server{
		Addr:    addr,
		Handler: s.mux,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Printf("web: server starting on %s", addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	go s.hub.run(ctx)

	select {
	case <-ctx.Done():
		return s.Stop()
	case err := <-errCh:
		return err
	}
}

// Stop stops the web server.
func (s *Server) Stop() error {
	if s.server == nil {
		return nil
	}
	s.hub.close()
	return s.server.Close()
}

// registerRoutes registers all HTTP routes on the server's ServeMux.
func (s *Server) registerRoutes() {
	m := s.mux

	// Static assets — no auth required
	m.HandleFunc("GET /{$}", s.handleIndex)
	m.HandleFunc("GET /static/style.css", s.handleStaticFile("static/style.css", "text/css"))
	m.HandleFunc("GET /static/app.js", s.handleStaticFile("static/app.js", "application/javascript"))
	m.HandleFunc("GET /static/hls.min.js", s.handleStaticFile("static/hls.min.js", "application/javascript"))

	// Auth endpoints — login is public, logout requires auth
	m.HandleFunc("POST /api/login", s.handleLogin)
	m.HandleFunc("POST /api/logout", s.authRequired(s.handleLogout))

	// API routes — auth required (bearer token in Authorization header)
	m.HandleFunc("GET /api/config", s.authRequired(s.handleGetConfig))
	m.HandleFunc("POST /api/config/onvif", s.authRequired(s.handlePostConfigOnvif))
	m.HandleFunc("GET /api/camera/params", s.authRequired(s.handleGetCameraParams))
	m.HandleFunc("POST /api/camera/param", s.authRequired(s.handlePostCameraParam))
	m.HandleFunc("GET /api/camera/options", s.authRequired(s.handleGetCameraOptions))
	m.HandleFunc("GET /api/ptz/status", s.authRequired(s.handleGetPTZStatus))
	m.HandleFunc("POST /api/ptz/move", s.authRequired(s.handlePostPTZMove))
	m.HandleFunc("POST /api/ptz/absolute", s.authRequired(s.handlePostPTZAbsolute))
	m.HandleFunc("POST /api/ptz/relative", s.authRequired(s.handlePostPTZRelative))
	m.HandleFunc("POST /api/ptz/stop", s.authRequired(s.handlePostPTZStop))
	m.HandleFunc("GET /api/ptz/presets", s.authRequired(s.handleGetPTZPresets))
	m.HandleFunc("POST /api/ptz/preset", s.authRequired(s.handlePostPTZPreset))
	m.HandleFunc("POST /api/ptz/preset/goto", s.authRequired(s.handlePostPTZPresetGoto))
	m.HandleFunc("DELETE /api/ptz/preset/{token}", s.authRequired(s.handleDeletePTZPreset))
	m.HandleFunc("GET /api/snapshot", s.authRequired(s.handleGetSnapshot))
	// HLS live stream — auth required (bearer token)
	m.HandleFunc("GET /api/hls/{name}", s.authRequired(s.handleHLS))

	// WebSocket — auth via ?token= query string
	m.HandleFunc("GET /ws", s.authRequired(s.handleWS))
}

// wsEvent represents a WebSocket event to broadcast.
type wsEvent struct {
	Type  string      `json:"type"`
	Name  string      `json:"name,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

// jsonBufPool reduces allocations for JSON marshalling.
var jsonBufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 0, 256)
	},
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	buf := jsonBufPool.Get().([]byte)
	buf = buf[:0]
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		jsonBufPool.Put(buf)
		return
	}
	jsonBufPool.Put(buf)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(data)
	w.Write([]byte("\n"))
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// handleIndex serves the embedded index.html.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// handleStaticFile serves an embedded static file.
func (s *Server) handleStaticFile(path, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := staticFS.ReadFile(path)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Write(data)
	}
}

// handleHLS serves HLS playlist (.m3u8) and segment (.ts) files from
// the HLS server's output directory. Path traversal is prevented by
// validation + filepath.Join cleaning.
func (s *Server) handleHLS(w http.ResponseWriter, r *http.Request) {
	if s.cfg.HLS == nil {
		http.Error(w, "hls not enabled", http.StatusNotFound)
		return
	}
	name := r.PathValue("name")
	if name == "" || strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}
	fullPath := filepath.Join(s.cfg.HLS.OutputDir(), name)
	// Set explicit content-type for HLS mime types — Go's mime map doesn't
	// include .ts and would sniff the file as text otherwise.
	switch filepath.Ext(name) {
	case ".m3u8":
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	case ".ts":
		w.Header().Set("Content-Type", "video/mp2t")
	}
w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
http.ServeFile(w, r, fullPath)
}

// extractPresetToken extracts the token from the URL path /api/ptz/preset/{token}.
func extractPresetToken(r *http.Request) string {
	// Go 1.22+ ServeMux with {token} pattern puts it in r.PathValue
	return r.PathValue("token")
}

// maskPassword returns "***" if the password is non-empty, "" otherwise.
func maskPassword(pw string) string {
	if pw == "" {
		return ""
	}
	return "***"
}

// coerceFloat64 converts JSON-decoded float64 to appropriate Go type for ParamManager.
// JSON numbers always decode as float64; this converts to int for integer params.
func coerceFloat64(v interface{}) interface{} {
	f, ok := v.(float64)
	if !ok {
		return v
	}
	if f == float64(int(f)) && !strings.Contains(fmt.Sprintf("%g", f), ".") {
		return int(f)
	}
	return f
}

// _ = fmt.Sprint to avoid import in some Go versions
var _ = strings.TrimSpace
