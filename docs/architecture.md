[中文](zh/architecture.md)

# Architecture - rpi-cam ONVIF Camera Server

## System Overview

rpi-cam is a lightweight Go application providing ONVIF-compliant camera services for Raspberry Pi. It replaces MediaMTX with a custom implementation to add missing ONVIF server capabilities while maintaining low resource usage (~20MB RAM, measured on device: rpi-cam 9MB + mtxrpicam 10MB). The system supports ONVIF Device/Media/PTZ/Imaging services, RTSP streaming, RTMP push, and WS-Discovery for NVR integration.

## Component Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Main Server                             │
│                   (cmd/server/main.go)                     │
└───────────────────────┬───────────────────────────────────────┘
                       │
┌───────────────────────▼───────────────────────────────────────┐
│                ONVIF Server                                │
│              (internal/onvif/server.go)                     │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐│
│  │ Device Service  │  │ Media Service  │  │ PTZ Service    ││
│  │                 │  │                 │  │                ││
│  │ - Device Info   │  │ - Get Profiles  │  │ - Continuous   ││
│  │ - Capabilities  │  │ - Get StreamUri│  │   Movement     ││
│  │ - WS-Discovery  │  │ - Get Snapshot │  │ - Absolute Move││
│  │                 │  │                 │  │ - Presets      ││
│  └─────────────────┘  └─────────────────┘  └─────────────────┘│
│  ┌─────────────────┐  ┌─────────────────┐                     │
│  │ Imaging Service │  │ WS-Discovery   │                     │
│  │                 │  │                │                     │
│  │ - Brightness    │  │ - UDP Probe    │                     │
│  │ - Contrast      │  │ - HTTP Probe   │                     │
│  │ - Saturation    │  │                │                     │
│  │ - Exposure      │  │                │                     │
│  └─────────────────┘  └─────────────────┘                     │
└───────────────────────┬───────────────────────────────────────┘
                       │
┌───────────────────────▼───────────────────────────────────────┐
│                 AUHub (internal/h264/hub.go)               │
│                        Frame Fan-out                        │
└───────────────────────┬───────────────────────────────────────┘
                       │
           ┌────────────┼────────────┐
           │            │            │
    ┌──────▼────┐  ┌────▼────┐  ┌────▼────┐
    │ RTSP Server│  │Snapshot │  │RTMP Push│
    │            │  │Handler  │  │         │
    └────────────┘  └─────────┘  └─────────┘
```

## Key Components

### ONVIF Server (`internal/onvif/server.go`)

The ONVIF server implements a single-endpoint SOAP framework handling multiple ONVIF services:

- **Service Routing**: All SOAP actions dispatched to `/onvif/device_service`
- **Authentication**: WS-Security UsernameToken digest authentication
- **WS-Discovery**: Supports both UDP multicast and HTTP probe requests
- **SOAP Processing**: XML envelope parsing, action routing, fault handling
- **Configuration**: Interface-based config provider for auth and media parameters

Services implemented:
- **Device**: Device information, capabilities, WS-Discovery
- **Media**: Profiles, stream URI, snapshot access
- **PTZ**: Virtual pan/tilt/zoom control with presets
- **Imaging**: Camera parameter control (brightness, contrast, etc.)

### Camera Subsystem (`internal/camera/camera.go`)

Camera capture uses MediaMTX's proven `mtxrpicam` C binary (from
[mediamtx-rpicamera](https://github.com/bluenviron/mediamtx-rpicamera)) via subprocess. It bundles its own `libcamera.so.9.9` to avoid version conflicts with system libcamera.

- **Pipe Protocol**: `PIPE_CONF_FD` for config, `PIPE_VIDEO_FD` for H.264 NALU frames
- **Subprocess Isolation**: Spawned with `Setpgid=true` for signal isolation
- **Parameter Control**: Real-time camera parameter updates via config pipe
- **Error Handling**: Process monitoring and graceful shutdown

Required files in `deploy/bin/`:
`mtxrpicam`, `libcamera.so.9.9`, `libcamera-base.so.9.9`,
`ipa_module/ipa_rpi_vc4.so`, `ipa_module/ipa_rpi_vc4.so.sign`,
`libpisp/backend_default_config.json`, `ipa_conf/`

`LD_LIBRARY_PATH` must include `deploy/bin/` so mtxrpicam finds bundled libcamera. The interface supports start/stop, parameter updates, and buffered frame delivery (2s at 15fps).

### H.264 AUHub (`internal/h264/hub.go`)

AUHub provides frame distribution to multiple consumers with fan-out pattern:

- **Thread Safety**: Internal mutex for concurrent subscriber management
- **Non-blocking Delivery**: Drops frames to prevent writer blocking
- **Subscriber Management**: Automatic cleanup on context cancellation
- **Access Unit Format**: H.264 access units with timestamp and keyframe detection

Consumers include:
- RTSP server for video streaming
- Snapshot handler for JPEG capture
- RTMP push for cloud services

### RTSP Server (`internal/rtsp/server.go`)

RTSP server built on `gortsplib v5` for H.264 streaming:

- **Protocol Support**: RTSP 1.0 with DESCRIBE, SETUP, PLAY commands
- **Authentication**: Optional digest authentication for stream access
- **On-demand Streaming**: Starts frame consumption only when clients connect
- **Media Description**: Dynamic H.264 format with SPS/PPS updates
- **Timestamp Synchronization**: NTP-adjusted timestamps for accurate playback

Key features:
- Single-port RTSP serving
- Client connection management
- RTP packet encoding and transmission
- Stream resource cleanup

### Digital PTZ (`internal/ptz/state.go`)

Virtual PTZ implementation with software-based positioning:

- **Position System**: Pan [-1,1], Tilt [-1,1], Zoom [0,1] coordinate ranges
- **Movement Modes**:
  - Continuous: Velocity-based movement with 50ms updates
  - Absolute: Exponential easing to target position
  - Relative: Immediate delta positioning
- **Preset Management**: Named position storage and recall
- **State Management**: Thread-safe position tracking and status reporting

PTZ operations map to camera cropping parameters for digital zoom without hardware changes.

## Data Flow Pipeline

OV5647 Camera → mtxrpicam → H.264 NALUs → Parser → AUHub → Subscribers
                                       ↓
                             ┌─────────────┼─────────────┐
                             │             │             │
                       RTSP Server     Snapshot Handler   RTMP Push
                       (gortsplib v5)  (FFmpeg → JPEG)    (loopback)
```

1. **Capture**: mtxrpicam subprocess captures frames from OV5647 CSI camera
2. **Transport**: H.264 data transferred via binary pipe to Go process
3. **Processing**: Parser extracts NALUs and timestamps, detects keyframes
4. **Distribution**: AUHub fans out access units to multiple consumers
5. **Streaming**: RTSP server serves video via gortsplib to NVR clients
6. **Snapshot**: FFmpeg subprocess converts H.264 keyframes to JPEG on demand
7. **Control**: ONVIF services provide camera control and discovery

## Resource Usage

Measured on Raspberry Pi 3B at 720p@15fps:

| Process | RSS Memory | Purpose |
|---------|------------|---------|
| rpi-cam | ~9MB | Go main process (ONVIF + RTSP + pipeline) |
| mtxrpicam | ~10MB | Camera capture subprocess |
| **Total** | **~20MB** | |

- **CPU**: ~2% for rpi-cam, ~12% for mtxrpicam at 720p@15fps
- **Network**: ~2Mbps for 720p@15fps H.264 stream

## Dependencies

- **gortsplib v5**: RTSP server functionality (same library as MediaMTX)
- **pion/rtp**: RTP packet handling for H.264 streaming
- **yaml.v3**: Configuration file parsing
- **onvif-go**: ONVIF server implementation (indirect dependency via research)
- **mtxrpicam**: Camera capture subprocess with bundled libcamera (from bluenviron/mediamtx-rpicamera v2.6.0)
- **FFmpeg**: On-demand JPEG conversion for snapshot endpoint (must be installed on device)

## Deployment Architecture

The system runs as a single systemd service with:

- **Process Isolation**: Camera capture in subprocess, main service in Go process
- **Resource Usage**: ~20MB RAM measured on RPi 3B
- **Cross-compilation**: Build from x86 workstation to aarch64 RPi
- **Configuration**: YAML-based config with environment variable overrides
- **Monitoring**: Prometheus metrics for operational visibility

### Camera Capture Dependencies

| Component | Type | Size | Purpose |
|-----------|------|------|---------|
| mtxrpicam | C binary (arm64) | 1.7MB | Camera capture + H.264 encoding |
| libcamera.so.9.9 | Shared library (bundled) | 5.7MB | Camera framework (from mediamtx-rpicamera) |
| libcamera-base.so.9.9 | Shared library (bundled) | 140KB | libcamera base support |
| ipa_module/ipa_rpi_vc4.so | IPA module | 690KB | RPi VC4 image processing |
| libpisp/backend_default_config.json | Config | 11KB | PiSP backend configuration |

These dependencies are bundled from mediamtx-rpicamera releases and do NOT depend on the system-installed libcamera. This avoids version conflicts between Debian's libcamera (0.7.0) and the version mtxrpicam was compiled against.

This architecture replaces MediaMTX entirely to provide ONVIF compliance while maintaining the proven camera capture and RTSP streaming components.