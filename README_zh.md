# rpi-cam

[![CI](https://github.com/Mi-Bee-Studio/raspberrypi-camera/actions/workflows/ci.yml/badge.svg)](https://github.com/Mi-Bee-Studio/raspberrypi-camera/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.26-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://github.com/Mi-Bee-Studio/raspberrypi-camera/blob/main/LICENSE)

[English](README.md)

rpi-cam 是一个轻量级的树莓派 ONVIF 相机服务，使用 Go 语言开发。它提供 ONVIF 设备/媒体/PTZ/成像服务、RTSP 流媒体、RTMP 推流和 WS-Discovery 支持，用于 NVR/VMS 集成。

## 功能

- **ONVIF 设备/媒体/PTZ/成像服务** - 完全符合 ONVIF 标准，支持 NVR 集成
- **RTSP 流媒体** - H.264 视频流，支持可配置的分辨率和码率
- **RTMP 推流** - 推送到阿里云、Twitch、YouTube 等云服务
- **WS-Discovery** - 网络自动发现相机
- **数字 PTZ** - 通过软件裁剪实现平移/倾斜/缩放
- **相机控制** - 亮度、对比度、饱和度、锐度调节
- **快照支持** - 通过 HTTP 端点获取 JPEG 快照
- **低内存占用** - 约 15-30MB RAM 使用量
- **跨平台构建** - 从 x86 工作站交叉编译到 aarch64 树莓派

## 快速开始

```bash
# 克隆并构建
git clone https://github.com/Mi-Bee-Studio/raspberrypi-camera
cd raspberrypi-camera
make build

# 复制并配置
cp configs/config.example.yaml config.yaml
# 编辑 config.yaml 配置相机和网络

# 直接运行
./build/rpi-cam -config config.yaml

# 或使用 systemd 部署
sudo cp deploy/rpi-cam.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now rpi-cam
```

## 配置

查看 `configs/config.example.yaml` 了解所有配置选项。主要设置包括：

- `camera.width/height` - 采集分辨率（默认 1280x720）
- `camera.fps` - 每秒帧数（树莓派 3B 默认 15）
- `camera.bitrate` - 视频码率（比特/秒）
- `rtsp.port` - RTSP 流媒体端口（默认 8554）
- `onvif.port` - ONVIF HTTP/SOAP 端口（默认 8080）
- `onvif.username/password` - ONVIF 认证凭据

环境变量使用 `RPICAM_` 前缀覆盖任何配置设置：
```bash
RPICAM_ONVIF_PASSWORD=secret ./build/rpi-cam
```

## 部署

基于 `deploy/rpi-cam.service` 创建 systemd 服务单元。根据你的环境自定义：

```bash
# 安装和配置
sudo cp deploy/rpi-cam.service /etc/systemd/system/
# 为你的设置编辑路径和用户
sudo systemctl daemon-reload
sudo systemctl enable --now rpi-cam
```

## 支持的摄像头

| Module | Sensor | Resolution | Focus | DT Overlay | Notes |
|--------|--------|------------|-------|------------|-------|
| Pi Camera V1 | OV5647 | 2592×1944 | Fixed | `ov5647` | 当前配置 |
| Pi Camera V2 | IMX219 | 3280×2464 | Fixed | `imx219` | 更好的低光性能 |
| Pi Camera V3 | IMX708 | 4608×2592 | Autofocus | `imx708` | PDAF，HDR 支持 |
| Pi HQ Camera | IMX477 | 4056×3040 | Manual lens | `imx477` | 可更换镜头 |
| USB (UVC) | Various | Various | Various | Auto-detected | `/dev/video*` |

## 架构

使用纯 Go 构建，依赖库包括：
- `gortsplib/v5` - RTSP 服务器（与 MediaMTX 相同的库）
- `onvif-go` - ONVIF 设备/媒体/PTZ/成像服务
- `libcamera` - 通过 mtxrpicam 辅助程序进行相机采集

ONVIF 服务依赖最少，无需 CGO。相机采集使用 MediaMTX 现有的 mtxrpicam 二进制文件，提供成熟的 CSI 相机支持。

## 开发

```bash
# 在工作站构建
make build

# 交叉编译到树莓派 3B
make build GOOS=linux GOARCH=arm64

# 运行测试
make test

# 部署到远程主机
make deploy REMOTE_HOST=user@your-rpi-host
```

## 许可证

MIT License - 详见 [LICENSE](LICENSE)