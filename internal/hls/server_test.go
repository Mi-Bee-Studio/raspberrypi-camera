package hls

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestStop(t *testing.T) {
	// Skip if ffmpeg not available.
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available on this system")
	}

	dir := t.TempDir()
	s := New(Config{
		OutputDir:      dir,
		RTSPURL:        "rtsp://127.0.0.1:1/nonexistent",
		RestartOnExit:  false,
		SegmentTime:    1,
		ListSize:       3,
	})

	// Use a short timeout so the test doesn't block for 15s.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start will likely fail (bad RTSP URL + short timeout), but we just
	// need to verify Stop cleans up properly.
	_ = s.Start(ctx)

	// Stop should not panic and should clean up the subprocess.
	if err := s.Stop(); err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}

	// Double-Stop must be safe (stopOnce ensures idempotency).
	if err := s.Stop(); err != nil {
		t.Errorf("second Stop() returned error: %v", err)
	}
}

func TestStopWithoutStart(t *testing.T) {
	// Stop on a never-started Server must not panic.
	s := New(Config{
		RTSPURL: "rtsp://127.0.0.1:1/nonexistent",
	})
	if err := s.Stop(); err != nil {
		t.Errorf("Stop() on unstarted server returned error: %v", err)
	}
}

func TestKillProcessGroupBogusPID(t *testing.T) {
	// killProcessGroup with a non-existent PID must not panic.
	// Use a PID that almost certainly does not exist as a process group.
	// NOTE: PID 0 means "current process group" and signals would be
	// sent to the test process itself. PID -1 would target init.
	killProcessGroup(999999999)
}
