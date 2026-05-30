# rpi-cam Troubleshooting Guide

[中文](zh/troubleshooting.md)

This guide covers common issues and solutions for rpi-cam, the Raspberry Pi ONVIF camera service.

## Quick Health Check

```bash
# Check if rpi-cam is running
systemctl status rpi-cam

# Check camera device
ls -la /dev/video0

# Check network connectivity
netstat -tlnp | grep -E '8554|8080|3702'

# Check memory usage
free -h

# Check CPU usage
top -bn1 | head -20
```

## Camera Detection Issues

### Symptoms
- Camera not found in NVR discovery
- rpi-cam logs show "camera not detected"
- Stream shows black screen

### Diagnosis
```bash
# Check if camera device exists
ls -la /dev/video0
# Should show /dev/video0 character device

# Test camera with libcamera directly
rpicam-hello

# Check device tree overlay
cat /boot/firmware/config.txt | grep dtoverlay
# Should show: dtoverlay=ov5647

# Check kernel camera support (ignore if device works via libcamera)
vcgencmd get_camera
```

### Solutions
1. **MediaMTX conflicts**: Stop MediaMTX first
   ```bash
   sudo systemctl stop mediamtx
   sudo systemctl disable mediamtx
   ```

2. **Missing DT overlay**: Add to config.txt
   ```bash
   sudo nano /boot/firmware/config.txt
   # Add: dtoverlay=ov5647
   sudo reboot
   ```

3. **Camera module not connected**: Check CSI cable

4. **Wrong device path**: Update config.yaml
   ```yaml
   camera:
     device: "/dev/video0"  # or your camera device
   ```

## RTSP Streaming Issues

### Symptoms
- RTSP stream not accessible
- Connection timeouts
- NVR can't connect to stream

### Diagnosis
```bash
# Check RTSP port is listening
netstat -tlnp | grep 8554

# Test RTSP connection locally
ffplay rtsp://localhost:8554/stream

# Check firewall rules
sudo ufw status

# Check camera exclusivity
lsof /dev/video0
```

### Solutions
1. **Port conflict**: Change RTSP port in config.yaml
   ```yaml
   rtsp:
     port: 8555  # Change if needed
   ```

2. **Firewall block**: Allow RTSP port
   ```bash
   sudo ufw allow 8554/tcp
   ```

3. **Camera access conflict**: Ensure only one process uses /dev/video0
   ```bash
   sudo systemctl stop mediamtx  # if running
   ```

4. **Network issues**: Check from client system
   ```bash
   telnet <camera-ip> 8554
   ```

## ONVIF Discovery Issues

### Symptoms
- NVR can't discover camera
- WS-Discovery probe fails
- ONVIF device service not found

### Diagnosis
```bash
# Check ONVIF HTTP port
netstat -tlnp | grep 8080

# Test UDP multicast port
nc -ul 3702

# Check ONVIF service logs
journalctl -u rpi-cam -f

# Test ONVIF endpoint manually
curl -X POST http://localhost:8080/onvif/device_service
```

### Solutions
1. **Network issues**: Check multicast routing
   ```bash
   # Enable multicast if needed
   sudo sysctl -w net.ipv4.conf.all.mc_forwarding=1
   ```

2. **Port conflict**: Change ONVIF port
   ```yaml
   onvif:
     port: 8081  # Change if needed
   ```

3. **Firewall blocks**: Allow ONVIF ports
   ```bash
   sudo ufw allow 8080/tcp
   sudo ufw allow 3702/udp
   ```

4. **Discovery timeout**: Increase in NVR config if needed

## NVR Integration Issues

### Symptoms
- NVR shows camera but can't add
- GetStreamUri fails
- Authentication issues

### Diagnosis
```bash
# Check ONVIF credentials in config.yaml
# Test ONVIF client manually
curl -X POST -H "Content-Type: application/soap+xml" \
  -d "<soap:Envelope>...</soap:Envelope>" \
  http://localhost:8080/onvif/device_service

# Check RTSP URL format
echo "rtsp://localhost:8554/stream"

# Check device info response
curl -s http://localhost:8080/onvif/device_service | grep -i device
```

### Solutions
1. **Authentication**: Set ONVIF credentials
   ```yaml
   onvif:
     username: "admin"
     password: "your-password"
   ```

2. **Invalid RTSP URL**: Ensure URL matches config
   ```yaml
   rtsp:
     username: ""  # Leave empty if no auth
     password: ""
   ```

3. **Profile issues**: Check video encoder config
   ```yaml
   camera:
     width: 1280
     height: 720
     codec: "h264"
   ```

4. **Device info**: Update device metadata
   ```yaml
   device:
     name: "My Camera"
     manufacturer: "Raspberry Pi"
     model: "OV5647"
   ```

## Snapshot Issues

### Symptoms
- Snapshot endpoint returns error
- NVR can't capture images
- FFmpeg errors in logs

### Diagnosis
```bash
# Check snapshot endpoint
curl -I http://localhost:8080/snapshot

# Test FFmpeg manually
ffmpeg -rtsp_transport tcp -i rtsp://localhost:8554/stream \
  -vf "scale=640:480" -frames:v 1 snapshot.jpg

# Check FFmpeg installation
ffmpeg -version
```

### Solutions
1. **FFmpeg missing**: Install FFmpeg
   ```bash
   sudo apt install ffmpeg
   ```

2. **Camera not running**: Ensure rpi-cam is active
   ```bash
   sudo systemctl restart rpi-cam
   ```

3. **Resolution issues**: Adjust snapshot parameters
   ```yaml
   camera:
     width: 1280
     height: 720
   ```

## Performance Issues

### Symptoms
- High memory usage
- Lag in video stream
- CPU overload

### Diagnosis
```bash
# Check memory usage
free -h

# Check CPU usage
top -bn1 | grep -E 'rpi-cam|mediamtx'

# Check RTSP connections
lsof -i :8554

# Check network bandwidth
ip -s link show wlan0
```

### Solutions
1. **Reduce resolution**: Lower capture settings
   ```yaml
   camera:
     width: 640
     height: 480
     fps: 10
     bitrate: 1000000  # 1 Mbps
   ```

2. **Limit concurrent streams**: Add connection limits
   ```yaml
   rtsp:
     max_clients: 5  # Limit concurrent viewers
   ```

3. **Monitor resources**: Add monitoring
   ```bash
   # Monitor memory every 5 seconds
   watch -n 5 "free -h && ps aux | grep rpi-cam"
   ```

## Debug Mode and Logging

### Enable Debug Logging
```bash
# Set debug level via environment
RPICAM_LOGGING_LEVEL=debug ./rpi-cam

# Or in config.yaml
logging:
  level: "debug"
```

### Verbose Mode Options
```bash
# Enable ONVIF debug logging
RPICAM_LOGGING_LEVEL=debug ./rpi-cam -onvif-debug

# Enable RTSP debug logging
RPICAM_LOGGING_LEVEL=debug ./rpi-cam -rtsp-debug
```

### Log Analysis Tips
```bash
# Follow logs in real-time
journalctl -u rpi-cam -f

# Filter error messages
journalctl -u rpi-cam | grep ERROR

# Look for timeout patterns
journalctl -u rpi-cam | grep -i timeout

# Check for resource warnings
journalctl -u rpi-cam | grep -i "memory\|cpu"
```

## Common Error Messages

### Camera Access Issues
```
ERROR: camera device not available
```
- Solution: Stop MediaMTX, check device path

### Port Already in Use
```
ERROR: address already in use
```
- Solution: Change port in config or stop conflicting service

### Network Issues
```
ERROR: connection refused
```
- Solution: Check firewall, network connectivity

### Memory Issues
```
WARNING: high memory usage detected
```
- Solution: Reduce resolution, FPS, or bitrate

## System Status Commands

```bash
# Complete system health check
echo "=== System Status ==="
echo "Camera Device:"
ls -la /dev/video0 2>/dev/null || echo "No camera found"

echo "Services:"
systemctl is-active rpi-cam mediamtx

echo "Network:"
netstat -tlnp | grep -E '8554|8080|3702'

echo "Memory:"
free -h

echo "Camera Process:"
pgrep -f rpi-cam || echo "rpi-cam not running"

echo "Conflicting Processes:"
lsof /dev/video0 2>/dev/null | grep -v rpi-cam || echo "No conflicts"
```

## Contact Support

If issues persist:
1. Check logs: `journalctl -u rpi-cam`
2. Include system info: `uname -a`, `dpkg -l | grep rpi-cam`
3. Provide exact error messages and reproduction steps
4. Include configuration file (redact sensitive data)