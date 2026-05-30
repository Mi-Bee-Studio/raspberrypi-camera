package onvif

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Mi-Bee-Studio/raspberrypi-camera/internal/h264"
)

// GetSnapshotUriResponse is the ONVIF GetSnapshotUri SOAP response.
type GetSnapshotUriResponse struct {
	XMLName  xml.Name `xml:"trt:GetSnapshotUriResponse"`
	MediaUri MediaUri `xml:"tt:MediaUri"`
}

// SnapshotBuffer stores the latest H.264 key frame data for snapshot requests.
// It subscribes to the AUHub and keeps the most recent IDR frame in memory.
type SnapshotBuffer struct {
	mu         sync.RWMutex
	latestAU   *h264.AccessUnit
	available  bool
}

// NewSnapshotBuffer creates a new frame buffer.
func NewSnapshotBuffer() *SnapshotBuffer {
	return &SnapshotBuffer{}
}

// Feed updates the buffer with a new access unit.
// Only IDR frames are stored to ensure we always have a decodable reference.
func (sb *SnapshotBuffer) Feed(au h264.AccessUnit) {
	if !au.KeyFrame {
		return
	}
	sb.mu.Lock()
	defer sb.mu.Unlock()

	copied := h264.AccessUnit{
		NALUs:     make([]h264.NALU, len(au.NALUs)),
		Timestamp: au.Timestamp,
		KeyFrame:  au.KeyFrame,
	}
	for i, nalu := range au.NALUs {
		copied.NALUs[i] = h264.NALU{
			Type:  nalu.Type,
			Data:  append([]byte(nil), nalu.Data...),
			IsIDR: nalu.IsIDR,
			IsSPS: nalu.IsSPS,
			IsPPS: nalu.IsPPS,
		}
	}
	sb.latestAU = &copied
	sb.available = true
}

// Latest returns the most recent IDR access unit. Returns nil if none available.
func (sb *SnapshotBuffer) Latest() *h264.AccessUnit {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	if !sb.available || sb.latestAU == nil {
		return nil
	}
	au := sb.latestAU
	return au
}

// Available reports whether a frame has been stored.
func (sb *SnapshotBuffer) Available() bool {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.available
}

// RegisterSnapshotHandlers registers the GetSnapshotUri SOAP action and the
// /snapshot HTTP endpoint. The frameSource channel feeds the snapshot buffer
// with incoming H.264 access units for the /snapshot endpoint.
func RegisterSnapshotHandlers(s *Server, frameSource <-chan h264.AccessUnit) {
	buf := NewSnapshotBuffer()

	// Feed buffer from frame source in background goroutine.
	go func() {
		for au := range frameSource {
			buf.Feed(au)
		}
	}()

	s.RegisterAction("GetSnapshotUri", func(ctx context.Context, body []byte, auth *AuthResult) (interface{}, error) {
		return handleGetSnapshotUri(s.config), nil
	})

	s.snapshotHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleSnapshotHTTP(w, r, buf)
	})
}

// handleGetSnapshotUri returns the snapshot URL for the device.
func handleGetSnapshotUri(cfg ConfigProvider) *GetSnapshotUriResponse {
	uri := fmt.Sprintf("http://%s:%d/snapshot", cfg.DeviceIP(), cfg.ONVIFPort())

	return &GetSnapshotUriResponse{
		MediaUri: MediaUri{
			Uri:                uri,
			InvalidAfterConnect: "false",
			InvalidAfterReboot:  "false",
			Timeout:             "PT10S",
		},
	}
}

// handleSnapshotHTTP serves a snapshot from the frame buffer.
// If no frame is available yet, it returns a 503 Service Unavailable.
// The response contains raw H.264 NALU data with Content-Type: image/jpeg
// as a placeholder. Post-MVP, this will be replaced with actual JPEG conversion.
func handleSnapshotHTTP(w http.ResponseWriter, r *http.Request, buf *SnapshotBuffer) {
	au := buf.Latest()
	if au == nil {
		http.Error(w, "snapshot not available: no frame received yet", http.StatusServiceUnavailable)
		return
	}

	// Build raw H.264 frame data from NALUs.
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("X-Frame-Timestamp", au.Timestamp.UTC().Format(time.RFC3339Nano))

	var totalLen int
	for _, nalu := range au.NALUs {
		totalLen += len(nalu.Data)
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", totalLen))
	for _, nalu := range au.NALUs {
		w.Write(nalu.Data)
	}
}
