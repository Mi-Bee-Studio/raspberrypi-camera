# rpi-cam

[![CI](https://github.com/Mi-Bee-Studio/raspberrypi-camera/actions/workflows/ci.yml/badge.svg)](https://github.com/Mi-Bee-Studio/raspberrypi-camera/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.26-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://github.com/Mi-Bee-Studio/raspberrypi-camera/blob/main/LICENSE)

[中文文档](README_zh.md)

rpi-cam is a lightweight Go ONVIF camera service for Raspberry Pi. It provides ONVIF Device/Media/PTZ/Imaging services, RTSP streaming, RTMP push, and WS-Discovery for NVR/VMS integration.

## Features

- **ONVIF Device/Media/PTZ/Imaging Services** - Full ONVIF compliance for NVR integration
- **RTSP Streaming** - H.264 video streaming at configurable resolutions and bitrates
- **RTMP Push** - Stream to cloud services like Aliyun, Twitch, YouTube
- **WS-Discovery** - Automatic camera discovery on the network
- **Digital PTZ** - Pan/tilt/zoom via software cropping
- **Camera Controls** - Brightness, contrast, saturation, sharpness adjustment
- **Snapshot Support** - JPEG snapshots via HTTP endpoint
- **Low Memory Footprint** - ~15-30MB RAM usage
- **Cross-Platform Build** - Compile from x86 workstation to aarch64 RPi

## Quick Start

```bash
# Clone and build
git clone https://github.com/Mi-Bee-Studio/raspberrypi-camera
cd raspberrypi-camera
make build

# Copy and configure
cp configs/config.example.yaml config.yaml
# Edit config.yaml for your camera and network

# Run directly
./build/rpi-cam -config config.yaml

# Or deploy with systemd
sudo cp deploy/rpi-cam.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now rpi-cam
```

## Configuration

See `configs/config.example.yaml` for all configuration options. Key settings include:

- `camera.width/height` - Capture resolution (1280x720 default)
- `camera.fps` - Frames per second (15 default for RPi 3B)
- `camera.bitrate` - Video bitrate in bits per second
- `rtsp.port` - RTSP streaming port (8554 default)
- `onvif.port` - ONVIF HTTP/SOAP port (8080 default)
- `onvif.username/password` - ONVIF authentication credentials

Environment variables override any config setting with `RPICAM_` prefix:
```bash
RPICAM_ONVIF_PASSWORD=secret ./build/rpi-cam
```

## Deployment

Create a systemd service unit based on `deploy/rpi-cam.service`. Customize for your environment:

```bash
# Install and configure
sudo cp deploy/rpi-cam.service /etc/systemd/system/
# Edit paths and user for your setup
sudo systemctl daemon-reload
sudo systemctl enable --now rpi-cam
```

## Supported Cameras

| Module | Sensor | Resolution | Focus | DT Overlay | Notes |
|--------|--------|------------|-------|------------|-------|
| Pi Camera V1 | OV5647 | 2592×1944 | Fixed | `ov5647` | Current setup |
| Pi Camera V2 | IMX219 | 3280×2464 | Fixed | `imx219` | Better low light |
| Pi Camera V3 | IMX708 | 4608×2592 | Autofocus | `imx708` | PDAF, HDR support |
| Pi HQ Camera | IMX477 | 4056×3040 | Manual lens | `imx477` | Interchangeable lens |
| USB (UVC) | Various | Various | Various | Auto-detected | `/dev/video*` |

## Architecture

Built with pure Go using:
- `gortsplib/v5` - RTSP server (same library as MediaMTX)
- `onvif-go` - ONVIF Device/Media/PTZ/Imaging services
- `libcamera` - Camera capture via mtxrpicam helper

Minimal dependencies and zero CGO requirements for ONVIF services. Camera capture uses MediaMTX's existing mtxrpicam binary for proven CSI camera support.

## Development

```bash
# Build on workstation
make build

# Cross-compile for RPi 3B
make build GOOS=linux GOARCH=arm64

# Run tests
make test

# Deploy to remote
make deploy REMOTE_HOST=user@your-rpi-host
```

## License

MIT License - see [LICENSE](LICENSE) for details.