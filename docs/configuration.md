# Configuration Documentation

[中文](zh/configuration.md)

The rpi-cam configuration is written in YAML format and controls all aspects of the camera service, including capture settings, streaming protocols, and device identification.

## Configuration File

### File Location
Configuration is loaded from `configs/config.yaml` by default. Create this file by copying `configs/config.example.yaml` and modifying it for your setup.

### File Format
```yaml
# Comments use the # symbol
# Top-level sections define functional areas
camera:        # Camera capture settings
rtsp:          # RTSP streaming server
onvif:         # ONVIF device services
rtmp:          # RTMP push streaming
device:        # Device identification
logging:       # Logging configuration
```

## Configuration Sections

### Camera Configuration

Camera capture settings control how video frames are captured from the camera device.

```yaml
camera:
  # Path to mtxrpicam binary (camera capture subprocess)
  # This binary and its bundled libcamera libraries must be present at this path
  bin_path: deploy/bin/mtxrpicam

  # Camera device path (V4L2 or libcamera)
  device: /dev/video0
  
  # Capture resolution (width x height)
  # Supported resolutions: 640x480, 1296x972, 1920x1080, 2592x1944
  width: 1280
  height: 720
  
  # Frames per second (max 30 for OV5647 sensor)
  fps: 15
  
  # Video codec (h264 or h265)
  codec: h264
  
  # Target bitrate in bits per second
  # Example: 2000000 = 2 Mbps
  bitrate: 2000000
  
  # Image controls (hardware-specific ranges apply)
  # Brightness: -1.0 to 1.0 (0.0 = default, negative = darker, positive = brighter)
  brightness: 0.0
  
  # Contrast: 0.0 to 32.0 (1.0 = default)
  contrast: 1.0
  
  # Saturation: 0.0 to 32.0 (1.0 = default)
  saturation: 1.0
  
  # Sharpness: 0.0 to 16.0 (1.0 = default)
  sharpness: 1.0
```

### RTSP Configuration

RTSP server settings for video streaming clients.

```yaml
rtsp:
  # RTSP server port (default: 8554)
  port: 8554
  
  # Optional RTSP authentication
  # Leave empty strings for no authentication
  username: ""
  password: ""
```

### ONVIF Configuration

ONVIF server settings for device discovery and control via NVR systems.

```yaml
onvif:
  # ONVIF HTTP/SOAP port (default: 8080)
  port: 8080
  
  # ONVIF WS-UsernameToken authentication
  # Required for MiBee NVR integration
  username: "admin"
  
  # ONVIF password (MUST be set for production)
  password: ""
```

### RTMP Configuration

RTMP push settings for streaming to cloud services.

```yaml
rtmp:
  # Enable RTMP push streaming
  enabled: false
  
  # RTMP push URL for cloud services
  # Examples: 
  # - rtmp://push-server/app/stream
  # - rtmp://live.twitch.tv/app/channel-key
  url: "rtmp://push-server/app/stream"
```

### Device Configuration

Device information exposed via ONVIF GetDeviceInformation service.

```yaml
device:
  # Friendly camera name for display in NVR
  name: "Pi Camera V1"
  
  # Device manufacturer
  manufacturer: "Raspberry Pi"
  
  # Camera sensor model
  model: "OV5647"
  
  # Firmware version string
  firmware: "1.0.0"
  
  # Hardware identifier
  hardware_id: "OV5647"
  
  # Serial number (empty if not available)
  serial_number: ""
```

### Logging Configuration

Logging settings for debugging and monitoring.

```yaml
logging:
  # Log level (debug, info, warn, error)
  # debug: Most verbose, includes all debug messages
  # info: Standard operational logging
  # warn: Only warnings and errors
  # error: Only errors
  level: "info"
```

## Default Values Reference

| Section | Field | Default Value | Type | Description |
|---------|-------|---------------|------|-------------|
| **camera** | bin_path | `"deploy/bin/mtxrpicam"` | string | Path to mtxrpicam binary |
| | device | `/dev/video0` | string | Camera device path |
| | width | `1280` | int | Capture width in pixels |
| | height | `720` | int | Capture height in pixels |
| | fps | `15` | int | Frames per second |
| | codec | `"h264"` | string | Video codec |
| | bitrate | `2000000` | int | Bitrate in bits per second |
| | brightness | `0.0` | float | Brightness control |
| | contrast | `1.0` | float | Contrast control |
| | saturation | `1.0` | float | Saturation control |
| | sharpness | `1.0` | float | Sharpness control |
| **rtsp** | port | `8554` | int | RTSP server port |
| | username | `""` | string | RTSP username |
| | password | `""` | string | RTSP password |
| **onvif** | port | `8080` | int | ONVIF HTTP port |
| | username | `"admin"` | string | ONVIF username |
| | password | `""` | string | ONVIF password |
| **rtmp** | enabled | `false` | bool | Enable RTMP push |
| | url | `"rtmp://push-server/app/stream"` | string | RTMP push URL |
| **device** | name | `"Pi Camera V1"` | string | Friendly camera name |
| | manufacturer | `"Raspberry Pi"` | string | Device manufacturer |
| | model | `"OV5647"` | string | Camera sensor model |
| | firmware | `"1.0.0"` | string | Firmware version |
| | hardware_id | `"OV5647"` | string | Hardware identifier |
| | serial_number | `""` | string | Device serial number |
| **logging** | level | `"info"` | string | Log level |

## Environment Variable Overrides

All configuration values can be overridden using environment variables with the `RPICAM_` prefix. This is useful for deployment, testing, and containerized environments.

### Format
Environment variables follow the pattern: `RPICAM_<SECTION>_<FIELD>`

### Examples

```bash
# Override camera resolution
RPICAM_CAMERA_WIDTH=1920 RPICAM_CAMERA_HEIGHT=1080 ./rpi-cam

# Set ONVIF password for production
RPICAM_ONVIF_PASSWORD=securepassword123 ./rpi-cam

# Change RTSP port
RPICAM_RTSP_PORT=554 ./rpi-cam

# Enable debug logging
RPICAM_LOGGING_LEVEL=debug ./rpi-cam

# Set device information
RPICAM_DEVICE_NAME="Office Camera" ./rpi-cam
```

### All Environment Variables

| Section | Field | Environment Variable |
|---------|-------|---------------------|
| **camera** | bin_path | `RPICAM_CAMERA_BINPATH` |
| | device | `RPICAM_CAMERA_DEVICE` |
| | width | `RPICAM_CAMERA_WIDTH` |
| | height | `RPICAM_CAMERA_HEIGHT` |
| | fps | `RPICAM_CAMERA_FPS` |
| | codec | `RPICAM_CAMERA_CODEC` |
| | bitrate | `RPICAM_CAMERA_BITRATE` |
| | brightness | `RPICAM_CAMERA_BRIGHTNESS` |
| | contrast | `RPICAM_CAMERA_CONTRAST` |
| | saturation | `RPICAM_CAMERA_SATURATION` |
| | sharpness | `RPICAM_CAMERA_SHARPNESS` |
| **rtsp** | port | `RPICAM_RTSP_PORT` |
| | username | `RPICAM_RTSP_USERNAME` |
| | password | `RPICAM_RTSP_PASSWORD` |
| **onvif** | port | `RPICAM_ONVIF_PORT` |
| | username | `RPICAM_ONVIF_USERNAME` |
| | password | `RPICAM_ONVIF_PASSWORD` |
| **rtmp** | enabled | `RPICAM_RTMP_ENABLED` |
| | url | `RPICAM_RTMP_URL` |
| **device** | name | `RPICAM_DEVICE_NAME` |
| | manufacturer | `RPICAM_DEVICE_MANUFACTURER` |
| | model | `RPICAM_DEVICE_MODEL` |
| | firmware | `RPICAM_DEVICE_FIRMWARE` |
| | hardware_id | `RPICAM_DEVICE_HARDWAREID` |
| | serial_number | `RPICAM_DEVICE_SERIALNUMBER` |
| **logging** | level | `RPICAM_LOGGING_LEVEL` |

## Example Configurations

### Basic Configuration (Default Settings)

```yaml
# configs/config.yaml
camera:
  device: /dev/video0
  width: 1280
  height: 720
  fps: 15
  codec: h264
  bitrate: 2000000
  brightness: 0.0
  contrast: 1.0
  saturation: 1.0
  sharpness: 1.0

rtsp:
  port: 8554
  username: ""
  password: ""

onvif:
  port: 8080
  username: "admin"
  password: ""

rtmp:
  enabled: false
  url: "rtmp://push-server/app/stream"

device:
  name: "Pi Camera V1"
  manufacturer: "Raspberry Pi"
  model: "OV5647"
  firmware: "1.0.0"
  hardware_id: "OV5647"
  serial_number: ""

logging:
  level: "info"
```

### High-Resolution Configuration

```yaml
camera:
  device: /dev/video0
  width: 1920
  height: 1080
  fps: 25
  codec: h264
  bitrate: 4000000  # 4 Mbps
  brightness: 0.2
  contrast: 1.5
  saturation: 1.2
  sharpness: 2.0

rtsp:
  port: 8554
  username: "stream"
  password: "streampass"

onvif:
  port: 8080
  username: "admin"
  password: "onvif123"

device:
  name: "HD Security Camera"
  manufacturer: "Raspberry Pi"
  model: "OV5647"
  firmware: "2.0.0"
  hardware_id: "OV5647-HD"
  serial_number: "SN-2024-001"
```

### Cloud Streaming Configuration

```yaml
camera:
  width: 1280
  height: 720
  fps: 15
  codec: h264
  bitrate: 2000000

rtsp:
  port: 8554
  username: ""
  password: ""

onvif:
  port: 8080
  username: "admin"
  password: "secure123"

rtmp:
  enabled: true
  url: "rtmp://live.example.com/live/stream-key"

device:
  name: "Cloud Stream Camera"
  manufacturer: "Raspberry Pi"
  model: "OV5647"
  firmware: "1.2.0"
  hardware_id: "OV5647-CLOUD"

logging:
  level: "warn"
```

### Low-Bandwidth Configuration

```yaml
camera:
  width: 640
  height: 480
  fps: 10
  codec: h264
  bitrate: 500000  # 0.5 Mbps
  brightness: 0.0
  contrast: 1.0
  saturation: 1.0
  sharpness: 1.0

rtsp:
  port: 8554
  username: ""
  password: ""

onvif:
  port: 8080
  username: "admin"
  password: "lowpass"

device:
  name: "Low Bandwidth Camera"
  manufacturer: "Raspberry Pi"
  model: "OV5647"
  firmware: "1.0.0"
  hardware_id: "OV5647-LBW"

logging:
  level: "error"
```

## Configuration Tips

1. **Camera Compatibility**: Not all resolutions and settings work with all camera modules. Test your configuration with your specific camera hardware.

2. **Performance**: Higher resolutions and bitrates require more CPU and bandwidth. On Raspberry Pi 3B, 720p @ 15fps is the recommended balance.

3. **Security**: Always set a strong password for ONVIF authentication in production environments.

4. **Network**: RTSP streaming can consume significant bandwidth. Ensure your network infrastructure can handle the chosen bitrate.

5. **Debugging**: Use `RPICAM_LOGGING_LEVEL=debug` to troubleshoot configuration issues.

6. **Environment Variables**: Use environment variables for sensitive data like passwords to avoid storing them in configuration files.

7. **Validation**: The service will validate configuration values against hardware constraints. Invalid settings will be logged or defaulted.

8. **Camera Binary**: The `bin_path` must point to a valid mtxrpicam binary. The directory containing this binary must also contain the bundled libcamera shared libraries (libcamera.so.9.9, libcamera-base.so.9.9) and IPA modules. See deployment documentation for details.