# Configuration Documentation

[English](../configuration.md)

rpi-cam 配置采用 YAML 格式，控制摄像头服务的所有方面，包括捕获设置、流媒体协议和设备标识。

## 配置文件

### 文件位置
配置默认从 `configs/config.yaml` 加载。通过复制 `configs/config.example.yaml` 并根据您的设置进行修改来创建此文件。

### 文件格式
```yaml
# 注释使用 # 符号
# 顶级部分定义功能区域
camera:        # 摄像头捕获设置
rtsp:          # RTSP 流媒体服务器
onvif:         # ONVIF 设备服务
rtmp:          # RTMP 推送流媒体
device:        # 设备标识
logging:       # 日志配置
```

## 配置部分

### 摄像头配置

摄像头捕获设置控制如何从摄像头设备捕获视频帧。

```yaml
camera:
  # 摄像头设备路径（V4L2 或 libcamera）
  device: /dev/video0
  
  # 捕获分辨率（宽度 x 高度）
  # 支持的分辨率：640x480, 1296x972, 1920x1080, 2592x1944
  width: 1280
  height: 720
  
  # 每秒帧数（OV5647 传感器最大 30）
  fps: 15
  
  # 视频编解码器（h264 或 h265）
  codec: h264
  
  # 目标比特率（每秒位数）
  # 示例：2000000 = 2 Mbps
  bitrate: 2000000
  
  # 图像控制（硬件特定范围适用）
  # 亮度：-1.0 到 1.0（0.0 = 默认值，负值 = 更暗，正值 = 更亮）
  brightness: 0.0
  
  # 对比度：0.0 到 32.0（1.0 = 默认值）
  contrast: 1.0
  
  # 饱和度：0.0 到 32.0（1.0 = 默认值）
  saturation: 1.0
  
  # 锐度：0.0 到 16.0（1.0 = 默认值）
  sharpness: 1.0
```

### RTSP 配置

RTSP 服务器设置用于视频流客户端。

```yaml
rtsp:
  # RTSP 服务器端口（默认：8554）
  port: 8554
  
  # 可选的 RTSP 身份验证
  # 留空字符串表示无身份验证
  username: ""
  password: ""
```

### ONVIF 配置

ONVIF 服务器设置，用于通过 NVR 系统进行设备发现和控制。

```yaml
onvif:
  # ONVIF HTTP/SOAP 端口（默认：8080）
  port: 8080
  
  # ONVIF WS-UsernameToken 身份验证
  # MiBee NVR 集成必需
  username: "admin"
  
  # ONVIF 密码（生产环境必须设置）
  password: ""
```

### RTMP 配置

RTMP 推送设置，用于流式传输到云服务。

```yaml
rtmp:
  # 启用 RTMP 推送流媒体
  enabled: false
  
  # 云服务的 RTMP 推送 URL
  # 示例： 
  # - rtmp://push-server/app/stream
  # - rtmp://live.twitch.tv/app/channel-key
  url: "rtmp://push-server/app/stream"
```

### 设备配置

通过 ONVIF GetDeviceInformation 服务公开的设备信息。

```yaml
device:
  # NVR 中显示的友好摄像头名称
  name: "Pi Camera V1"
  
  # 设备制造商
  manufacturer: "Raspberry Pi"
  
  # 摄像头传感器型号
  model: "OV5647"
  
  # 固件版本字符串
  firmware: "1.0.0"
  
  # 硬件标识符
  hardware_id: "OV5647"
  
  # 序列号（如果不可用则为空）
  serial_number: ""
```

### 日志配置

用于调试和监控的日志设置。

```yaml
logging:
  # 日志级别（debug, info, warn, error）
  # debug：最详细，包含所有调试消息
  # info：标准操作日志
  # warn：仅警告和错误
  # error：仅错误
  level: "info"
```

## 默认值参考

| 部分 | 字段 | 默认值 | 类型 | 描述 |
|------|------|--------|------|------|
| **camera** | device | `/dev/video0` | string | 摄像头设备路径 |
| | width | `1280` | int | 捕获宽度（像素） |
| | height | `720` | int | 捕获高度（像素） |
| | fps | `15` | int | 每秒帧数 |
| | codec | `"h264"` | string | 视频编解码器 |
| | bitrate | `2000000` | int | 比特率（每秒位数） |
| | brightness | `0.0` | float | 亮度控制 |
| | contrast | `1.0` | float | 对比度控制 |
| | saturation | `1.0` | float | 饱和度控制 |
| | sharpness | `1.0` | float | 锐度控制 |
| **rtsp** | port | `8554` | int | RTSP 服务器端口 |
| | username | `""` | string | RTSP 用户名 |
| | password | `""` | string | RTSP 密码 |
| **onvif** | port | `8080` | int | ONVIF HTTP 端口 |
| | username | `"admin"` | string | ONVIF 用户名 |
| | password | `""` | string | ONVIF 密码 |
| **rtmp** | enabled | `false` | bool | 启用 RTMP 推送 |
| | url | `"rtmp://push-server/app/stream"` | string | RTMP 推送 URL |
| **device** | name | `"Pi Camera V1"` | string | 友好摄像头名称 |
| | manufacturer | `"Raspberry Pi"` | string | 设备制造商 |
| | model | `"OV5647"` | string | 摄像头传感器型号 |
| | firmware | `"1.0.0"` | string | 固件版本 |
| | hardware_id | `"OV5647"` | string | 硬件标识符 |
| | serial_number | `""` | string | 设备序列号 |
| **logging** | level | `"info"` | string | 日志级别 |

## 环境变量覆盖

所有配置值都可以使用 `RPICAM_` 前缀的环境变量覆盖。这对于部署、测试和容器化环境很有用。

### 格式
环境变量遵循模式：`RPICAM_<部分>_<字段>`

### 示例
```bash
# 覆盖摄像头分辨率
RPICAM_CAMERA_WIDTH=1920 RPICAM_CAMERA_HEIGHT=1080 ./rpi-cam

# 为生产环境设置 ONVIF 密码
RPICAM_ONVIF_PASSWORD=securepassword123 ./rpi-cam

# 更改 RTSP 端口
RPICAM_RTSP_PORT=554 ./rpi-cam

# 启用调试日志
RPICAM_LOGGING_LEVEL=debug ./rpi-cam

# 设置设备信息
RPICAM_DEVICE_NAME="Office Camera" ./rpi-cam
```

### 所有环境变量

| 部分 | 字段 | 环境变量 |
|------|------|----------|
| **camera** | device | `RPICAM_CAMERA_DEVICE` |
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

## 示例配置

### 基本配置（默认设置）
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

### 高分辨率配置
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

### 云流媒体配置
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

### 低带宽配置
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

## 配置提示

1. **摄像头兼容性**：并非所有分辨率和设置都与所有摄像头模块兼容。请使用您的特定摄像头硬件测试配置。

2. **性能**：更高的分辨率和比特率需要更多的 CPU 和带宽。在树莓派 3B 上，720p @ 15fps 是推荐的平衡点。

3. **安全性**：在生产环境中，始终为 ONVIF 身份验证设置强密码。

4. **网络**：RTSP 流媒体可能消耗大量带宽。确保您的网络基础设施能够处理所选的比特率。

5. **调试**：使用 `RPICAM_LOGGING_LEVEL=debug` 来解决配置问题。

6. **环境变量**：使用环境变量存储像密码这样的敏感数据，避免将它们存储在配置文件中。

7. **验证**：服务将根据硬件约束验证配置值。无效的设置将被记录或设置为默认值。