[中文](zh/deployment.md)
# RPi-CAM ONVIF Camera Server Deployment Guide

This guide covers deployment of the rpi-cam ONVIF camera server for Raspberry Pi, including migration from MediaMTX and integration with NVR systems.

## Prerequisites

### Target Device Requirements
- Raspberry Pi 3B (or similar ARM64 device)
- Camera module connected (OV5647, IMX219, IMX477, or USB)
- Debian 13 (trixie) with kernel 6.12.75+rpt-rpi-v8
- 905MB RAM minimum
- User account with sudo NOPASSWD privileges
- Network connectivity to target RPi

### Workstation Requirements
- Go 1.26+ installed
- SSH access to target device
- SCP file transfer capability

## Build Process

### Cross-Compilation from Workstation

```bash
# Clone the repository
git clone https://github.com/Mi-Bee-Studio/raspberrypi-camera
cd raspberrypi-camera

# Build for ARM64 architecture
make build GOOS=linux GOARCH=arm64

# Verify binary creation
ls -la build/rpi-cam
```

## Installation Steps

### 1. Prepare Target Device

```bash
# Create working directory on target
ssh <your-rpi-user>@<your-rpi-ip> "mkdir -p ~/rpi-cam"

# Stop MediaMTX to free camera access
ssh <your-rpi-user>@<your-rpi-ip> 'sudo systemctl stop mediamtx'
ssh <your-rpi-user>@<your-rpi-ip> 'sudo systemctl disable mediamtx'
```

### 2. Deploy Files

```bash
# Copy binary and configuration
scp build/rpi-cam <your-rpi-user>@<your-rpi-ip>:~/rpi-cam/
scp configs/config.example.yaml <your-rpi-user>@<your-rpi-ip>:~/rpi-cam/config.yaml

# Copy systemd service unit
scp deploy/rpi-cam.service <your-rpi-user>@<your-rpi-ip>:/tmp/
```

### 3. Install Systemd Service

```bash
# Install service and enable
ssh <your-rpi-user>@<your-rpi-ip> "sudo mv /tmp/rpi-cam.service /etc/systemd/system/"
ssh <your-rpi-user>@<your-rpi-ip> "sudo systemctl daemon-reload"
ssh <your-rpi-user>@<your-rpi-ip> "sudo systemctl enable rpi-cam"
```

### 4. Automated Deployment

```bash
# Run automated deployment
./deploy/deploy.sh

# The script automatically:
# 1. Stops and disables MediaMTX
# 2. Deploys rpi-cam binary and config
# 3. Installs systemd service
# 4. Enables the service
```

## Configuration

### Configuration File Setup

```bash
# Copy example configuration
cp configs/config.example.yaml config.yaml

# Edit configuration for your environment
nano config.yaml
```

### Key Configuration Sections

```yaml
# Camera settings
camera:
  device: /dev/video0
  width: 1280
  height: 720
  fps: 15
  codec: h264
  bitrate: 2000000

# ONVIF settings
onvif:
  port: 8080
  username: "admin"
  password: "your-password"  # REQUIRED: set secure password

# Device information
device:
  name: "Pi Camera V1"
  manufacturer: "Raspberry Pi"
  model: "OV5647"
  firmware: "1.0.0"

# Logging
logging:
  level: "info"  # debug, info, warn, error
```

### Environment Variables

```bash
# Start with specific password
RPICAM_ONVIF_PASSWORD=secret123 ./build/rpi-cam -config config.yaml

# Debug logging
RPICAM_LOGGING_LEVEL=debug ./build/rpi-cam -config config.yaml
```

## Starting the Service

### Systemd Service Management

```bash
# Start the service
sudo systemctl start rpi-cam

# Check status
systemctl status rpi-cam

# View logs
journalctl -u rpi-cam -f

# Enable auto-start on boot
sudo systemctl enable rpi-cam

# Restart service
sudo systemctl restart rpi-cam

# Stop service
sudo systemctl stop rpi-cam
```

## Verification

### 1. Service Status Check

```bash
# Check if service is running
systemctl status rpi-cam

# Verify no errors in logs
journalctl -u rpi-cam --since "5 minutes ago"
```

### 2. RTSP Stream Test

```bash
# Test RTSP stream with FFmpeg
ffmpeg -rtsp_transport tcp -i rtsp://<your-rpi-ip>:8554/stream -t 10 test.mp4

# Or with VLC
vlc rtsp://<your-rpi-ip>:8554/stream
```

### 3. ONVIF Discovery Test

```bash
# Test WS-Discovery with onvif-scan
onvif-scan --host <your-rpi-ip> --port 8080

# Test HTTP POST discovery
curl -X POST http://<your-rpi-ip>:8080/onvif/device_service \
  -H "Content-Type: application/soap+xml" \
  -d @probe.xml
```

### 4. MiBee NVR Integration Test

```bash
# Add camera to MiBee NVR web interface:
# 1. Navigate to NVR web UI (use configured admin credentials)
# 2. Go to Camera Management → Add Camera
# 3. Select ONVIF protocol
# 4. Enter camera IP address and credentials
# 5. Verify camera appears online and streams video
```

## Troubleshooting Common Issues

### Camera Access Issues

**Problem:** "Camera device busy" error
**Solution:** Ensure MediaMTX is completely stopped:
```bash
sudo systemctl stop mediamtx
sudo systemctl disable mediamtx
```

**Problem:** Camera not detected
**Solution:** Check DT overlay in `/boot/firmware/config.txt`:
```
dtoverlay=ov5647  # or imx219, imx477, etc.
```

### Network Access Issues

**Problem:** ONVIF discovery fails
**Solution:** Check firewall settings:
```bash
# Check if port 8080 is open
sudo ufw status

# Allow ONVIF port if needed
sudo ufw allow 8080/tcp
```

**Problem:** RTSP connection refused
**Solution:** Verify RTSP port configuration and service status:
```bash
# Check RTSP server logs
journalctl -u rpi-cam --grep "RTSP"

# Test port connectivity
telnet <your-rpi-ip> 8554
```

### Configuration Issues

**Problem:** Service fails to start
**Solution:** Check configuration syntax:
```bash
# Validate YAML syntax
yamllint config.yaml

# Check config values
./build/rpi-cam --validate-config --config config.yaml
```

**Problem:** Invalid ONVIF password
**Solution:** Set a strong password in config.yaml:
```yaml
onvif:
  password: "secure-password-123"
```

## Maintenance

### Updates and Upgrades

```bash
# Pull latest changes
git pull origin main

# Rebuild and redeploy
make build
make deploy

# Restart service
make service-restart
```

### Backup Configuration

```bash
# Backup current configuration
sudo cp /etc/systemd/system/rpi-cam.service ~/backups/
cp config.yaml ~/backups/config.yaml.$(date +%Y%m%d).backup
```

## Support

For additional support:
- Review troubleshooting documentation
- Check service logs with `journalctl -u rpi-cam -f`
- Validate configuration with `--validate-config` flag
- Test with debug logging: `RPICAM_LOGGING_LEVEL=debug`