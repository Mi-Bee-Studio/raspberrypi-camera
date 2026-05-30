// Portions derived from MediaMTX (https://github.com/bluenviron/mediamtx)
// Original code Copyright (c) bluenviron, MIT License
//
// camera.go defines the Camera interface and the RPiCamera implementation
// that communicates with the mtxrpicam subprocess via the binary pipe protocol.

package camera

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// Frame represents a single captured video frame.
type Frame struct {
	Data      []byte    // H.264 Annex-B NALU data (may contain multiple NALUs)
	Timestamp time.Time // Frame capture time (NTP-adjusted)
	PTS       int64     // Presentation timestamp in 90kHz clock
}

// CameraInfo provides metadata about the camera device.
type CameraInfo struct {
	Name         string
	Manufacturer string
	Model        string
	Width        uint32
	Height       uint32
	FPS          float32
	Codec        string
	SerialNumber string
}

// Camera is the interface for camera capture backends.
type Camera interface {
	// Start begins capturing frames from the camera device.
	Start(ctx context.Context) error

	// Stop gracefully stops capture and releases resources.
	Stop() error

	// Frames returns a read-only channel that receives captured frames.
	// The channel is closed when Stop() is called or on error.
	Frames() <-chan Frame

	// SetParam changes a camera parameter at runtime.
	// Supported names: brightness, contrast, saturation, sharpness,
	// width, height, fps, exposure, gain, awbMode, hFlip, vFlip,
	// shutter, denoise, ev, bitrate, idrPeriod.
	SetParam(name string, value interface{}) error

	// GetParam returns the current value of a camera parameter.
	GetParam(name string) (interface{}, error)

	// Info returns camera device information.
	Info() CameraInfo
}

// RPiCamera implements Camera by spawning the mtxrpicam subprocess
// and communicating via the binary pipe protocol.
type RPiCamera struct {
	mu      sync.RWMutex
	params  Params
	info    CameraInfo

	// subprocess management
	cmd       *exec.Cmd
	confPipe  *pipe // config: Go -> mtxrpicam
	videoPipe *pipe // video: mtxrpicam -> Go

	// frame delivery
	framesCh chan Frame
	doneCh   chan struct{}
	stopOnce sync.Once

	// binary path
	binPath string
}

// RPiCameraOption configures RPiCamera behavior.
type RPiCameraOption func(*RPiCamera)

// WithBinPath sets the path to the mtxrpicam binary.
func WithBinPath(path string) RPiCameraOption {
	return func(c *RPiCamera) {
		c.binPath = path
	}
}

// WithParams sets initial camera parameters.
func WithParams(p Params) RPiCameraOption {
	return func(c *RPiCamera) {
		c.params = p
	}
}

// WithInfo sets camera info metadata.
func WithInfo(info CameraInfo) RPiCameraOption {
	return func(c *RPiCamera) {
		c.info = info
	}
}

// NewRPiCamera creates a new RPiCamera with the given options.
func NewRPiCamera(opts ...RPiCameraOption) *RPiCamera {
	c := &RPiCamera{
		params:  DefaultParams(),
		binPath: filepath.Join("deploy", "bin", "mtxrpicam"),
		info: CameraInfo{
			Name:         "RPi Camera",
			Manufacturer: "Raspberry Pi",
			Model:        "OV5647",
			Width:        1280,
			Height:       720,
			FPS:          15,
			Codec:        "H264",
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	c.info.Width = c.params.Width
	c.info.Height = c.params.Height
	c.info.FPS = c.params.FPS

	return c
}

// Start spawns the mtxrpicam subprocess and begins reading frames.
func (c *RPiCamera) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd != nil {
		return fmt.Errorf("camera already started")
	}

	// Validate binary exists
	if _, err := os.Stat(c.binPath); err != nil {
		return fmt.Errorf("mtxrpicam binary not found at %s: %w", c.binPath, err)
	}

	// Create pipes for subprocess communication using syscall.Pipe
	// (os.Pipe sets close-on-exec, which would close FDs on fork).
	// mtxrpicam expects FDs passed via environment variables:
	//   PIPE_CONF_FD  — Go writes config commands here (child reads)
	//   PIPE_VIDEO_FD — mtxrpicam writes video frames here (child writes)
	var confFds, videoFds [2]int
	if err := syscall.Pipe(confFds[:]); err != nil {
		return fmt.Errorf("create conf pipe: %w", err)
	}
	if err := syscall.Pipe(videoFds[:]); err != nil {
		syscall.Close(confFds[0])
		syscall.Close(confFds[1])
		return fmt.Errorf("create video pipe: %w", err)
	}

	binDir := filepath.Dir(c.binPath)
	env := []string{
		"PIPE_CONF_FD=" + strconv.Itoa(confFds[0]),
		"PIPE_VIDEO_FD=" + strconv.Itoa(videoFds[1]),
		"LD_LIBRARY_PATH=" + binDir,
		"HOME=" + os.Getenv("HOME"),
		"PATH=" + os.Getenv("PATH"),
	}

	c.cmd = exec.CommandContext(ctx, c.binPath)
	c.cmd.Env = env
	c.cmd.Dir = binDir
	c.cmd.Stdout = os.Stdout
	c.cmd.Stderr = os.Stderr

	// Prevent subprocess from receiving signals from parent
	c.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := c.cmd.Start(); err != nil {
		syscall.Close(confFds[0])
		syscall.Close(confFds[1])
		syscall.Close(videoFds[0])
		syscall.Close(videoFds[1])
		c.cmd = nil
		return fmt.Errorf("start mtxrpicam: %w", err)
	}

	// Close the ends that belong to the child process
	syscall.Close(confFds[0])    // child reads config
	syscall.Close(videoFds[1])   // child writes video

	// Initialize pipe wrappers using os.NewFile for io.Reader/Writer interface
	confWriteFile := os.NewFile(uintptr(confFds[1]), "conf-write")
	videoReadFile := os.NewFile(uintptr(videoFds[0]), "video-read")
	c.confPipe = newPipe(nil, confWriteFile)
	c.videoPipe = newPipe(videoReadFile, nil)

	// Setup frame channel
	c.framesCh = make(chan Frame, 30) // buffer ~2 seconds at 15fps
	c.doneCh = make(chan struct{})

	// Start frame reader goroutine BEFORE sending config
	// so it's ready to receive the 'r' (ready) signal from mtxrpicam.
	go c.readLoop()

	// Send initial config
	if err := c.confPipe.write(c.params.SerializeCommand()); err != nil {
		c.cleanup()
		return fmt.Errorf("send initial config: %w", err)
	}

	return nil
}

// Stop gracefully stops the camera subprocess.
func (c *RPiCamera) Stop() error {
	var retErr error
	c.stopOnce.Do(func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		if c.cmd == nil {
			return
		}

		// Send quit command
		if c.confPipe != nil {
			_ = c.confPipe.write(SerializeQuit())
		}

		// Close config pipe write end to signal subprocess
		c.cleanup()

		retErr = nil
	})

	return retErr
}

// cleanup closes all pipes and waits for the subprocess.
// Must be called with mu held.
func (c *RPiCamera) cleanup() {
	if c.confPipe != nil {
		// Close the underlying writer (confWrite)
		if closer, ok := c.confPipe.writer.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		c.confPipe = nil
	}

	if c.videoPipe != nil {
		// Close the underlying reader (videoRead)
		if closer, ok := c.videoPipe.reader.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		c.videoPipe = nil
	}

	if c.framesCh != nil {
		close(c.framesCh)
		c.framesCh = nil
	}

	if c.doneCh != nil {
		close(c.doneCh)
		c.doneCh = nil
	}

	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_ = c.cmd.Wait()
		c.cmd = nil
	}
}

// Frames returns the read-only channel of captured frames.
func (c *RPiCamera) Frames() <-chan Frame {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.framesCh
}

// SetParam modifies a camera parameter and sends the update to the subprocess.
func (c *RPiCamera) SetParam(name string, value interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd == nil {
		return fmt.Errorf("camera not started")
	}

	paramName, ok := mapParamName(name)
	if !ok {
		return fmt.Errorf("unknown parameter: %s", name)
	}

	if err := setParamValue(&c.params, paramName, value); err != nil {
		return fmt.Errorf("set %s: %w", name, err)
	}

	// Send updated params to subprocess
	if c.confPipe == nil {
		return fmt.Errorf("config pipe not available")
	}

	if err := c.confPipe.write(c.params.SerializeCommand()); err != nil {
		return fmt.Errorf("send param update: %w", err)
	}

	// Update info if resolution/FPS changed
	c.info.Width = c.params.Width
	c.info.Height = c.params.Height
	c.info.FPS = c.params.FPS

	return nil
}

// GetParam returns the current value of a camera parameter.
func (c *RPiCamera) GetParam(name string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	paramName, ok := mapParamName(name)
	if !ok {
		return nil, fmt.Errorf("unknown parameter: %s", name)
	}

	return getParamValue(c.params, paramName)
}

// Info returns the camera device information.
func (c *RPiCamera) Info() CameraInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.info
}

// readLoop reads frames from the video pipe and sends them to the frames channel.
// This runs in its own goroutine started by Start().
func (c *RPiCamera) readLoop() {
	defer func() {
		// Ensure cleanup on exit
		c.mu.Lock()
		c.cleanup()
		c.mu.Unlock()
		}()


	for {
		buf, err := c.videoPipe.read()
		if err != nil {
			return // pipe closed or error — cleanup will happen
		}

		if len(buf) == 0 {
			continue
		}

		c.mu.RLock()
		framesCh := c.framesCh
		c.mu.RUnlock()

		if framesCh == nil {
			return
		}

		switch buf[0] {
		case 'e':
			// Error from subprocess
			errMsg := string(buf[1:])
			log.Printf("camera: mtxrpicam error: %s", errMsg)
			return

		case 'r':
			// Ready signal — subprocess is ready to capture
			continue

		case 'd':
			// Video frame data
			if len(buf) < 9 {
				continue
			}

			// Parse DTS (8 bytes, little-endian)
			dts := int64(buf[8])<<56 | int64(buf[7])<<48 | int64(buf[6])<<40 |
				int64(buf[5])<<32 | int64(buf[4])<<24 | int64(buf[3])<<16 |
				int64(buf[2])<<8 | int64(buf[1])

			// Convert DTS (microseconds) to PTS (90kHz)
			pts := multiplyAndDivide(dts, 90000, 1e6)

			// Calculate NTP timestamp
			now := time.Now()
			ntp := now

			// Extract NALU data (everything after the 9-byte header)
			naluData := make([]byte, len(buf)-9)
			copy(naluData, buf[9:])

			// Detect keyframe: look for SPS NALU (type 7) in Annex-B data
			isKeyFrame := isIDRFrame(naluData)

			frame := Frame{
				Data:      naluData,
				Timestamp: ntp,
				PTS:       pts,
			}

			// Non-blocking send — drop frame if channel full
			select {
			case framesCh <- frame:
				_ = isKeyFrame // stored in frame struct if needed
			default:
				// Frame dropped — consumer too slow
			}

		default:
			// Unknown message type — ignore
			continue
		}
	}
}

// multiplyAndDivide performs (v * m / d) without overflow.
// Portions derived from MediaMTX.
func multiplyAndDivide(v, m, d int64) int64 {
	secs := v / d
	dec := v % d
	return secs*m + dec*m/d
}

// isIDRFrame checks if the Annex-B NALU data contains an IDR frame.
// IDR NALU types in H.264: 5 (IDR slice) or SPS (type 7) followed by IDR.
func isIDRFrame(data []byte) bool {
	if len(data) < 4 {
		return false
	}

	// Annex-B start code: 00 00 00 01 or 00 00 01
	// After start code, NALU header: first byte, lower 5 bits = NALU type
	// Type 5 = IDR slice
	i := 0
	for i < len(data)-3 {
		if data[i] == 0 && data[i+1] == 0 {
			// 4-byte start code: 00 00 00 01
			if i+3 < len(data) && data[i+2] == 0 && data[i+3] == 1 {
				if i+4 < len(data) {
					naluType := data[i+4] & 0x1F
					if naluType == 5 {
						return true
					}
				}
				i += 5
				continue
			}
			// 3-byte start code: 00 00 01
			if data[i+2] == 1 {
				naluType := data[i+3] & 0x1F
				if naluType == 5 {
					return true
				}
				i += 4
				continue
			}
		}
		i++
	}
	return false
}

// mapParamName maps user-facing parameter names to internal param field names.
func mapParamName(name string) (string, bool) {
	mapping := map[string]string{
		"brightness":  "Brightness",
		"contrast":    "Contrast",
		"saturation":  "Saturation",
		"sharpness":   "Sharpness",
		"width":       "Width",
		"height":      "Height",
		"fps":         "FPS",
		"exposure":    "Exposure",
		"gain":        "Gain",
		"awbMode":     "AWB",
		"hFlip":       "HFlip",
		"vFlip":       "VFlip",
		"shutter":     "Shutter",
		"denoise":     "Denoise",
		"ev":          "EV",
		"bitrate":     "Bitrate",
		"idrPeriod":   "IDRPeriod",
		"metering":    "Metering",
		"mode":        "Mode",
		"hdr":         "HDR",
		"awbGainRed":  "AWBGainRed",
		"awbGainBlue": "AWBGainBlue",
		"codec":       "Codec",
		"cameraID":    "CameraID",
	}

	internal, ok := mapping[name]
	return internal, ok
}
