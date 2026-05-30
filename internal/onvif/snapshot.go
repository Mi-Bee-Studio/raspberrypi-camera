package onvif

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"os/exec"
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

// handleSnapshotHTTP serves a JPEG snapshot by converting the latest H.264 IDR
// frame to JPEG via an on-demand FFmpeg subprocess.
func handleSnapshotHTTP(w http.ResponseWriter, r *http.Request, buf *SnapshotBuffer) {
	handleSnapshotHTTPWithBin(w, r, buf, "ffmpeg")
}

// snapshotTimeout is the maximum time allowed for FFmpeg conversion.
const snapshotTimeout = 5 * time.Second

// handleSnapshotHTTPWithBin is like handleSnapshotHTTP but accepts a custom
// ffmpeg binary path, used in tests to inject a mock converter.
func handleSnapshotHTTPWithBin(w http.ResponseWriter, r *http.Request, buf *SnapshotBuffer, ffmpegBin string) {
	au := buf.Latest()
	if au == nil {
		http.Error(w, "snapshot not available: no frame received yet", http.StatusServiceUnavailable)
		return
	}

	annexB := buildAnnexB(au.NALUs)
	if len(annexB) == 0 {
		http.Error(w, "snapshot not available: empty frame data", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), snapshotTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		ffmpegBin,
		"-f", "h264",
		"-i", "pipe:0",
		"-frames:v", "1",
		"-f", "image2pipe",
		"pipe:1",
	)
	cmd.Stdin = bytes.NewReader(annexB)

	jpegData, err := cmd.Output()
	if ctx.Err() != nil {
		http.Error(w, "snapshot conversion timed out", http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		http.Error(w, "snapshot conversion failed", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(jpegData)))
	w.Header().Set("X-Frame-Timestamp", au.Timestamp.UTC().Format(time.RFC3339Nano))
	w.Write(jpegData)
}

// startCode is the 4-byte H.264 Annex-B start code.
var startCode = []byte{0x00, 0x00, 0x00, 0x01}

// buildAnnexB reconstructs an H.264 Annex-B bytestream from NALUs.
// SPS and PPS NALUs are placed before the IDR for decodability.
func buildAnnexB(nalus []h264.NALU) []byte {
	var sps, pps, idr, others []h264.NALU
	for _, n := range nalus {
		switch {
		case n.IsSPS:
			sps = append(sps, n)
		case n.IsPPS:
			pps = append(pps, n)
		case n.IsIDR:
			idr = append(idr, n)
		default:
			others = append(others, n)
		}
	}

	var buf []byte
	for _, n := range sps {
		buf = append(buf, startCode...)
		buf = append(buf, n.Data...)
	}
	for _, n := range pps {
		buf = append(buf, startCode...)
		buf = append(buf, n.Data...)
	}
	for _, n := range others {
		buf = append(buf, startCode...)
		buf = append(buf, n.Data...)
	}
	for _, n := range idr {
		buf = append(buf, startCode...)
		buf = append(buf, n.Data...)
	}
	return buf
}
