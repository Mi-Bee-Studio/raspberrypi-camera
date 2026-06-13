package rtmp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewPush(t *testing.T) {
	cfg := Config{
		Enabled: true,
		URL:     "rtmp://push-server/app/stream",
		RTSPURL: "rtsp://localhost:8554/stream",
	}

	p := New(cfg)

	if p == nil {
		t.Fatal("expected non-nil Push")
	}
	if !p.Enabled() {
		t.Error("expected enabled=true")
	}
	if p.URL() != "rtmp://push-server/app/stream" {
		t.Errorf("expected URL=rtmp://push-server/app/stream, got %s", p.URL())
	}
	if p.Status() != StatusDisconnected {
		t.Errorf("expected initial status=disconnected, got %s", p.Status())
	}
	if p.RestartCount() != 0 {
		t.Errorf("expected initial restarts=0, got %d", p.RestartCount())
	}
}

func TestNewPushDefaults(t *testing.T) {
	cfg := Config{}
	p := New(cfg)

	if p.Enabled() {
		t.Error("expected enabled=false by default")
	}
	if p.URL() != "" {
		t.Errorf("expected empty URL, got %s", p.URL())
	}
}

func TestBuildCommand(t *testing.T) {
	cfg := Config{
		Enabled: true,
		URL:     "rtmp://push.example.com/live/key",
		RTSPURL: "rtsp://localhost:8554/stream",
	}
	p := New(cfg)

	cmd := p.buildCommand()
	if filepath.Base(cmd.Path) != "ffmpeg" {
		t.Errorf("expected basename=ffmpeg, got %s", cmd.Path)
	}

	args := cmd.Args
	if len(args) < 2 || args[0] != "ffmpeg" {
		t.Fatalf("unexpected Args: %v", args)
	}

	// Verify all expected flags are present in order
	expected := []string{
		"ffmpeg",
		"-rtsp_transport", "tcp",
		"-i", "rtsp://localhost:8554/stream",
		"-c", "copy",
		"-f", "flv",
		"rtmp://push.example.com/live/key",
	}

	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}

	for i, exp := range expected {
		if args[i] != exp {
			t.Errorf("arg[%d]: expected %q, got %q", i, exp, args[i])
		}
	}
}

func TestBuildCommandCopyFlag(t *testing.T) {
	cfg := Config{
		Enabled: true,
		URL:     "rtmp://x/a/b",
		RTSPURL: "rtsp://localhost:8554/stream",
	}
	p := New(cfg)
	cmd := p.buildCommand()

	found := false
	for _, arg := range cmd.Args {
		if arg == "-c" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected -c flag in FFmpeg command")
	}

	// Verify "copy" follows "-c"
	for i, arg := range cmd.Args {
		if arg == "-c" && i+1 < len(cmd.Args) && cmd.Args[i+1] == "copy" {
			return // success
		}
	}
	t.Error("expected '-c copy' in FFmpeg command")
}

func TestBuildCommandWithBin(t *testing.T) {
	cfg := Config{
		Enabled: true,
		URL:     "rtmp://x/a/b",
		RTSPURL: "rtsp://localhost:8554/stream",
	}
	p := New(cfg)

	cmd := p.buildCommandWithBin("/usr/local/bin/ffmpeg")
	if cmd.Path != "/usr/local/bin/ffmpeg" {
		t.Errorf("expected Path=/usr/local/bin/ffmpeg, got %s", cmd.Path)
	}

	args := cmd.Args
	if args[0] != "/usr/local/bin/ffmpeg" {
		t.Errorf("expected first arg=/usr/local/bin/ffmpeg, got %s", args[0])
	}
}

func TestSetURL(t *testing.T) {
	cfg := Config{
		Enabled: true,
		URL:     "rtmp://old-server/app/stream",
		RTSPURL: "rtsp://localhost:8554/stream",
	}
	p := New(cfg)

	p.SetURL("rtmp://new-server/live/key")
	if p.URL() != "rtmp://new-server/live/key" {
		t.Errorf("expected updated URL, got %s", p.URL())
	}
}

func TestDisabled(t *testing.T) {
	cfg := Config{
		Enabled: false,
		URL:     "rtmp://push-server/app/stream",
		RTSPURL: "rtsp://localhost:8554/stream",
	}
	p := New(cfg)

	// Start should return nil immediately for disabled push
	err := p.Start(context.Background())
	if err != nil {
		t.Errorf("expected nil error for disabled push, got %v", err)
	}

	// Status should remain disconnected
	if p.Status() != StatusDisconnected {
		t.Errorf("expected status=disconnected, got %s", p.Status())
	}
}

func TestStartStop(t *testing.T) {
	// Create a mock ffmpeg binary that sleeps for a long time
	tmpDir := t.TempDir()
	mockBin := filepath.Join(tmpDir, "ffmpeg")
	script := `#!/bin/sh
# Mock ffmpeg that sleeps
sleep 60
`
	if err := os.WriteFile(mockBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		Enabled: true,
		URL:     "rtmp://push-server/app/stream",
		RTSPURL: "rtsp://localhost:8554/stream",
	}
	p := New(cfg)

	// Override the ffmpeg binary by directly setting the command
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Manually set up the mock command
	cmd := p.buildCommandWithBin(mockBin)
	p.SetCmd(cmd)
	p.SetCancel(cancel)

	// Start the mock
	err := cmd.Start()
	if err != nil {
		t.Fatalf("failed to start mock ffmpeg: %v", err)
	}
	p.SetStatus(StatusConnected)

	// Verify it's running
	if cmd.Process == nil {
		t.Fatal("expected process to be running")
	}

	// Stop it
	err = p.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify status is disconnected
	if p.Status() != StatusDisconnected {
		t.Errorf("expected status=disconnected after stop, got %s", p.Status())
	}

	// Verify process was killed
	err = cmd.Wait()
	if err == nil {
		t.Error("expected error from killed process")
	}
}

func TestStatusTransitions(t *testing.T) {
	cfg := Config{
		Enabled: true,
		URL:     "rtmp://push-server/app/stream",
		RTSPURL: "rtsp://localhost:8554/stream",
	}
	p := New(cfg)

	// Initial status
	if p.Status() != StatusDisconnected {
		t.Errorf("expected disconnected, got %s", p.Status())
	}

	// Transition to connecting
	p.SetStatus(StatusConnecting)
	if p.Status() != StatusConnecting {
		t.Errorf("expected connecting, got %s", p.Status())
	}

	// Transition to connected
	p.SetStatus(StatusConnected)
	if p.Status() != StatusConnected {
		t.Errorf("expected connected, got %s", p.Status())
	}

	// Transition to error
	p.SetStatus(StatusError)
	if p.Status() != StatusError {
		t.Errorf("expected error, got %s", p.Status())
	}
}

func TestBuildCommandNoTranscoding(t *testing.T) {
	cfg := Config{
		Enabled: true,
		URL:     "rtmp://x/a/b",
		RTSPURL: "rtsp://localhost:8554/stream",
	}
	p := New(cfg)
	args := p.buildCommand().Args

	// Must NOT contain any transcoding flags
	forbidden := []string{"-c:v", "-c:a", "-vcodec", "-acodec", "-preset", "-crf"}
	for _, f := range forbidden {
		for _, arg := range args {
			if arg == f {
				t.Errorf("command must not contain transcoding flag %q (no transcoding allowed)", f)
			}
		}
	}
}

func TestRestartCountIncrement(t *testing.T) {
	cfg := Config{
		Enabled: true,
		URL:     "rtmp://push-server/app/stream",
		RTSPURL: "rtsp://localhost:8554/stream",
	}
	p := New(cfg)

	if p.RestartCount() != 0 {
		t.Fatalf("expected 0 restarts, got %d", p.RestartCount())
	}

	// Simulate restart increments via internal method
	p.mu.Lock()
	p.restartCount = 1
	p.mu.Unlock()

	if p.RestartCount() != 1 {
		t.Errorf("expected 1 restart, got %d", p.RestartCount())
	}
}

func TestAutoRestartWithMock(t *testing.T) {
	// Create a mock ffmpeg that exits immediately (simulates crash)
	tmpDir := t.TempDir()
	mockBin := filepath.Join(tmpDir, "ffmpeg")
	// Write a mock that exits with error immediately
	script := `#!/bin/sh
exit 1
`
	if err := os.WriteFile(mockBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	// Temporarily short-circuit: test the run loop directly with a short timeout
	// We create a push, start it, and check that it handles errors
	cfg := Config{
		Enabled: true,
		URL:     "rtmp://push-server/app/stream",
		RTSPURL: "rtsp://localhost:8554/stream",
	}
	p := New(cfg)

	// Set mock ffmpeg binary
	p.ffmpegBin = mockBin

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go p.run(ctx)

	// Wait for context to expire (should trigger stop)
	<-ctx.Done()

	// Should have attempted at least 1 restart
	if p.RestartCount() == 0 {
		t.Error("expected at least 1 restart attempt")
	}
}

func TestMaxRestarts(t *testing.T) {
	tmpDir := t.TempDir()
	mockBin := filepath.Join(tmpDir, "ffmpeg")
	script := `#!/bin/sh
exit 1
`
	if err := os.WriteFile(mockBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		Enabled: true,
		URL:     "rtmp://push-server/app/stream",
		RTSPURL: "rtsp://localhost:8554/stream",
	}
	p := New(cfg)
	p.ffmpegBin = mockBin

	// Pre-set restart count to just below the limit so one more attempt triggers cooldown
	p.mu.Lock()
	p.restartCount = maxRestarts - 1
	p.restartWindowStart = time.Now()
	p.mu.Unlock()

	// Use enough time for one restart cycle: mock exits instantly,
	// then restartDelay (5s) before the next loop iteration checks maxRestarts.
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	go p.run(ctx)

	<-ctx.Done()

	p.mu.RLock()
	count := p.restartCount
	cooldownSet := !p.cooldownUntil.IsZero()
	p.mu.RUnlock()

	t.Logf("Restart count: %d, cooldown set: %v", count, cooldownSet)

	if count < maxRestarts {
		t.Errorf("expected restart count >= %d, got %d", maxRestarts, count)
	}
	if !cooldownSet {
		t.Error("expected cooldown to be set after reaching max restarts")
	}
}

func TestStopIdempotent(t *testing.T) {
	cfg := Config{
		Enabled: false,
		RTSPURL: "rtsp://localhost:8554/stream",
	}
	p := New(cfg)

	// Stop on a never-started push should not panic
	if err := p.Stop(); err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	// Double stop should also not panic
	if err := p.Stop(); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}
