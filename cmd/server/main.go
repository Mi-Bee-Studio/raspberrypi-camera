package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/Mi-Bee-Studio/raspberrypi-camera/internal/camera"
	"github.com/Mi-Bee-Studio/raspberrypi-camera/internal/config"
	"github.com/Mi-Bee-Studio/raspberrypi-camera/internal/h264"
	"github.com/Mi-Bee-Studio/raspberrypi-camera/internal/onvif"
	"github.com/Mi-Bee-Studio/raspberrypi-camera/internal/ptz"
	"github.com/Mi-Bee-Studio/raspberrypi-camera/internal/rtmp"
	"github.com/Mi-Bee-Studio/raspberrypi-camera/internal/rtsp"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// configAdapter wraps config.Config to implement onvif.ConfigProvider.
type configAdapter struct {
	cfg      *config.Config
	deviceIP string
}

func (a *configAdapter) ONVIFUsername() string { return a.cfg.ONVIF.Username }
func (a *configAdapter) ONVIFPassword() string { return a.cfg.ONVIF.Password }
func (a *configAdapter) ONVIFPort() int        { return a.cfg.ONVIF.Port }
func (a *configAdapter) RTSPPort() int         { return a.cfg.RTSP.Port }
func (a *configAdapter) DeviceIP() string      { return a.deviceIP }
func (a *configAdapter) CameraWidth() int      { return a.cfg.Camera.Width }
func (a *configAdapter) CameraHeight() int     { return a.cfg.Camera.Height }
func (a *configAdapter) CameraFPS() int        { return a.cfg.Camera.FPS }
func (a *configAdapter) CameraBitrate() int    { return a.cfg.Camera.Bitrate }

// detectLocalIP finds the first non-loopback IPv4 address.
func detectLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return "127.0.0.1"
}

// noOpCamera is a stub camera.Camera used in tests.
// Imaging parameter changes are validated but not applied to hardware.
type noOpCamera struct{}

func (c *noOpCamera) Start(ctx context.Context) error          { return nil }
func (c *noOpCamera) Stop() error                        { return nil }
func (c *noOpCamera) Frames() <-chan camera.Frame        { return nil }
func (c *noOpCamera) SetParam(name string, value interface{}) error { return nil }
func (c *noOpCamera) GetParam(name string) (interface{}, error) {
	switch name {
	case "brightness":
		return 0.0, nil
	case "contrast":
		return 1.0, nil
	case "saturation":
		return 1.0, nil
	case "sharpness":
		return 1.0, nil
	case "exposure":
		return 0, nil
	case "gain":
		return 1.0, nil
	case "width":
		return 1280, nil
	case "height":
		return 720, nil
	case "fps":
		return 15, nil
	default:
		return nil, nil
	}
}
func (c *noOpCamera) Info() camera.CameraInfo {
	return camera.CameraInfo{}
}

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	localIP := detectLocalIP()
	adapter := &configAdapter{cfg: cfg, deviceIP: localIP}

	// --- Step 1: Camera ---
	cameraParams := camera.Params{
		Width:    uint32(cfg.Camera.Width),
		Height:   uint32(cfg.Camera.Height),
		FPS:      float32(cfg.Camera.FPS),
		Bitrate:  uint32(cfg.Camera.Bitrate),
		Codec:    "hardwareH264",
	}
	cameraInfo := camera.CameraInfo{
		Name:         cfg.Device.Name,
		Manufacturer: cfg.Device.Manufacturer,
		Model:        cfg.Device.Model,
		SerialNumber: cfg.Device.SerialNumber,
		Width:        uint32(cfg.Camera.Width),
		Height:       uint32(cfg.Camera.Height),
		FPS:          float32(cfg.Camera.FPS),
		Codec:        "H264",
	}

	cam := camera.NewRPiCamera(
		camera.WithBinPath("deploy/bin/mtxrpicam"),
		camera.WithParams(cameraParams),
		camera.WithInfo(cameraInfo),
	)

	if err := cam.Start(ctx); err != nil {
		log.Fatalf("camera start: %v", err)
	}
	defer cam.Stop()

	// --- Step 2: H264 Parser + AUHub ---
	parser := h264.NewParser()
	auHub := h264.NewAUHub()

	go func() {
		for frame := range cam.Frames() {
			nalus := parser.Parse(frame.Data)
			if len(nalus) == 0 {
				continue
			}
			au := h264.AccessUnit{
				NALUs:     nalus,
				Timestamp: frame.Timestamp,
				KeyFrame:  false,
			}
			for _, n := range nalus {
				if n.IsIDR {
					au.KeyFrame = true
					break
				}
			}
			auHub.Write(au)
		}
	}()

	// --- Step 3: RTSP Server ---
	rtspSub := auHub.Subscribe(ctx)
	rtspServer := rtsp.New(rtsp.Config{
		Port:     cfg.RTSP.Port,
		Username: cfg.RTSP.Username,
		Password: cfg.RTSP.Password,
		Address:  localIP,
	})
	rtspServer.SetFrameSource(rtspSub.Channel)

	if err := rtspServer.Start(ctx); err != nil {
		log.Fatalf("rtsp server start: %v", err)
	}
	defer rtspServer.Stop()

	// --- Step 4: ParamManager + PTZ ---
	paramManager := camera.NewParamManager(cam)
	ptzState := ptz.NewState()

	// --- Step 5: ONVIF Server ---
	onvifServer := onvif.New(adapter)

	deviceHost := fmt.Sprintf("%s:%d", localIP, cfg.ONVIF.Port)
	onvif.RegisterDeviceHandlers(onvifServer, deviceHost, onvif.DeviceInfo{
		Name:         cfg.Device.Name,
		Manufacturer: cfg.Device.Manufacturer,
		Model:        cfg.Device.Model,
		Firmware:     cfg.Device.Firmware,
		HardwareID:   cfg.Device.HardwareID,
		SerialNumber: cfg.Device.SerialNumber,
	})
	onvif.RegisterMediaHandlers(onvifServer)
	onvif.RegisterImagingHandlers(onvifServer, paramManager)
	onvif.RegisterPTZHandlers(onvifServer, ptzState)

	// Snapshot: second AUHub subscriber
	snapshotSub := auHub.Subscribe(ctx)
	onvif.RegisterSnapshotHandlers(onvifServer, snapshotSub.Channel)

	// --- Step 6: WS-Discovery ---
	discovery := onvif.NewDiscovery(cfg, localIP)
	if err := discovery.StartUDP(ctx); err != nil {
		log.Printf("warning: failed to start WS-Discovery: %v", err)
	}
	defer discovery.StopUDP()
	onvifServer.SetDiscoveryHandler(http.HandlerFunc(discovery.HandleHTTPProbe))

	// Start ONVIF HTTP server in goroutine
	go func() {
		if err := onvifServer.Start(ctx); err != nil {
			log.Printf("onvif server exited: %v", err)
		}
	}()

	// --- Step 7: RTMP Push (optional) ---
	if cfg.RTMP.Enabled {
		rtmpPush := rtmp.New(rtmp.Config{
			Enabled: true,
			URL:     cfg.RTMP.URL,
			RTSPURL: fmt.Sprintf("rtsp://localhost:%d/stream", cfg.RTSP.Port),
		})
		rtmpPush.Start(ctx)
		defer rtmpPush.Stop()
	}

	<-ctx.Done()
	log.Printf("rpi-cam %s shutting down...", version)
}
