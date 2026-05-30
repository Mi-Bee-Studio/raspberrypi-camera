// Package rtmp manages FFmpeg subprocess for RTSP→RTMP stream push.
package rtmp

import (
	"context"
	"os/exec"
	"sync"
	"time"
)

// Status represents the RTMP push connection state.
type Status string

const (
	StatusDisconnected Status = "disconnected"
	StatusConnecting   Status = "connecting"
	StatusConnected    Status = "connected"
	StatusError        Status = "error"
)

// maxRestarts is the maximum number of automatic restart attempts.
const maxRestarts = 3

// restartDelay is the delay between restart attempts.
const restartDelay = 5 * time.Second

// Config holds RTMP push configuration.
type Config struct {
	Enabled bool
	URL     string
	RTSPURL string // Local RTSP URL (e.g., rtsp://localhost:8554/stream)
}

// Push manages FFmpeg subprocess for RTSP→RTMP push.
type Push struct {
	mu        sync.RWMutex
	status    Status
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	url       string // RTMP target URL
	rtspURL   string // Local RTSP URL to pull from
	enabled   bool
	ffmpegBin string // Path to ffmpeg binary (default: "ffmpeg")
	restarts  int
}

// New creates a new RTMP push manager.
func New(cfg Config) *Push {
	return &Push{
		status:    StatusDisconnected,
		url:       cfg.URL,
		rtspURL:   cfg.RTSPURL,
		enabled:   cfg.Enabled,
		ffmpegBin: "ffmpeg",
	}
}

// Start begins pushing RTSP to RTMP via FFmpeg subprocess.
// The FFmpeg command: ffmpeg -rtsp_transport tcp -i {rtspURL} -c copy -f flv {rtmpURL}
// On crash, auto-restarts up to maxRestarts times with restartDelay between attempts.
func (p *Push) Start(ctx context.Context) error {
	p.mu.Lock()
	if !p.enabled {
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)

	p.mu.Lock()
	p.cancel = cancel
	p.mu.Unlock()

	go p.run(ctx)
	return nil
}

// Stop stops the FFmpeg subprocess and cancels auto-restart.
func (p *Push) Stop() error {
	p.mu.Lock()
	cancel := p.cancel
	p.cancel = nil
	cmd := p.cmd
	p.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
	}

	p.mu.Lock()
	p.status = StatusDisconnected
	p.restarts = 0
	p.mu.Unlock()

	return nil
}

// Status returns current connection status.
func (p *Push) Status() Status {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

// SetURL changes the RTMP target URL (requires restart to take effect).
func (p *Push) SetURL(url string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.url = url
}

// URL returns the current RTMP target URL.
func (p *Push) URL() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.url
}

// Enabled returns whether RTMP push is enabled.
func (p *Push) Enabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.enabled
}

// buildCommand constructs the FFmpeg command.
func (p *Push) buildCommand() *exec.Cmd {
	return p.buildCommandWithBin(p.ffmpegBin)
}

func (p *Push) run(ctx context.Context) {
	for {
		p.mu.Lock()
		if p.restarts >= maxRestarts {
			p.setStatus(StatusError)
			p.mu.Unlock()
			return
		}
		p.restarts++
		p.mu.Unlock()

		select {
		case <-ctx.Done():
			return
		default:
		}

		p.setStatus(StatusConnecting)

		cmd := p.buildCommand()

		p.mu.Lock()
		p.cmd = cmd
		p.mu.Unlock()

		err := cmd.Start()
		if err != nil {
			p.setStatus(StatusError)
			select {
			case <-ctx.Done():
				return
			case <-time.After(restartDelay):
				continue
			}
		}

		p.setStatus(StatusConnected)

		// Wait for FFmpeg to exit
		err = cmd.Wait()

		p.mu.Lock()
		p.cmd = nil
		p.mu.Unlock()

		if ctx.Err() != nil {
			return
		}

		if err != nil {
			p.setStatus(StatusError)
		} else {
			p.setStatus(StatusDisconnected)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(restartDelay):
			continue
		}
	}
}

func (p *Push) setStatus(s Status) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = s
}

// buildCommandWithBin is like buildCommand but allows overriding the ffmpeg binary path.
// Exported for testing.
func (p *Push) buildCommandWithBin(bin string) *exec.Cmd {
	return exec.Command(
		bin,
		"-rtsp_transport", "tcp",
		"-i", p.rtspURL,
		"-c", "copy",
		"-f", "flv",
		p.url,
	)
}

// RestartCount returns the number of restart attempts.
// Exported for testing.
func (p *Push) RestartCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.restarts
}

// SetStatus directly sets the status. Used only in tests.
func (p *Push) SetStatus(s Status) {
	p.setStatus(s)
}

// SetCancel sets the cancel function. Used only in tests.
func (p *Push) SetCancel(f context.CancelFunc) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cancel = f
}

// SetCmd sets the command. Used only in tests.
func (p *Push) SetCmd(cmd *exec.Cmd) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cmd = cmd
}
