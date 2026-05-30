[English](../architecture.md)

# Architecture - rpi-cam ONVIF 相机服务器

## 系统概述

rpi-cam 是一个轻量级的 Go 应用程序，为 Raspberry Pi 提供 ONVIF 兼容的相机服务。它用自定义实现替代 MediaMTX，以添加缺失的 ONVIF 服务器功能，同时保持低资源使用（约20MB RAM，实测：rpi-cam 9MB + mtxrpicam 10MB）和简化的部署。该系统支持 ONVIF Device/Media/PTZ/Imaging 服务、RTSP 流媒体、RTMP 推送和 WS-Discovery，用于 NVR 集成。

## 组件架构

```
┌─────────────────────────────────────────────────────────────┐
│                     主服务器                                │
│                   (cmd/server/main.go)                     │
└───────────────────────┬───────────────────────────────────────┘
                       │
┌───────────────────────▼───────────────────────────────────────┐
│                 ONVIF 服务器                                │
│              (internal/onvif/server.go)                     │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐│
│  │  设备服务      │  │  媒体服务      │  │  PTZ 服务      ││
│  │                │  │                │  │                ││
│  │ - 设备信息     │  │ - 获取配置文件 │  │ - 连续移动     ││
│  │ - 功能能力     │  │ - 获取流URI    │  │ - 绝对移动     ││
│  │ - WS-发现      │  │ - 获取快照    │  │ - 预设位置     ││
│  │                │  │                │  │                ││
│  └─────────────────┘  └─────────────────┘  └─────────────────┘│
│  ┌─────────────────┐  ┌─────────────────┐                     │
│  │  成像服务      │  │ WS-发现        │                     │
│  │                │  │                │                     │
│  │ - 亮度        │  │ - UDP 探测     │                     │
│  │ - 对比度      │  │ - HTTP 探测    │                     │
│  │ - 饱和度      │  │                │                     │
│  │ - 曝光        │  │                │                     │
│  └─────────────────┘  └─────────────────┘                     │
└───────────────────────┬───────────────────────────────────────┘
                       │
┌───────────────────────▼───────────────────────────────────────┐
│                AUHub (internal/h264/hub.go)                  │
│                       帧分发模式                           │
└───────────────────────┬───────────────────────────────────────┘
                       │
           ┌────────────┼────────────┐
           │            │            │
    ┌──────▼────┐  ┌────▼────┐  ┌────▼────┐
    │ RTSP 服务器│  │快照处理器│  │RTMP 推送│
    │           │  │         │  │         │
    └───────────┘  └─────────┘  └─────────┘
```

## 关键组件

### ONVIF 服务器 (`internal/onvif/server.go`)

ONVIF 服务器实现单端点 SOAP 框架，处理多个 ONVIF 服务：

- **服务路由**: 所有 SOAP 操作分发到 `/onvif/device_service`
- **身份验证**: WS-Security UsernameToken 摘要身份验证
- **WS-发现**: 支持 UDP 组播和 HTTP 探测请求
- **SOAP 处理**: XML 信封解析、操作路由、故障处理
- **配置**: 接口式的配置提供程序，用于身份验证和媒体参数

实现的服务：
- **设备**: 设备信息、功能能力、WS-发现
- **媒体**: 配置文件、流URI、快照访问
- **PTZ**: 虚拟平移/倾斜/缩放控制，支持预设位置
- **成像**: 相机参数控制（亮度、对比度等）

### 相机子系统 (`internal/camera/camera.go`)

相机捕获使用 MediaMTX 经过验证的 `mtxrpicam` C 二进制文件（来自
[mediamtx-rpicamera](https://github.com/bluenviron/mediamtx-rpicamera)）通过子进程运行。它捆绑了自带的 `libcamera.so.9.9`，以避免与系统 libcamera 版本冲突。

- **管道协议**: `PIPE_CONF_FD` 用于配置，`PIPE_VIDEO_FD` 用于 H.264 NALU 帧
- **子进程隔离**: 使用 `Setpgid=true` 生成，实现信号隔离
- **参数控制**: 通过配置管道实时更新相机参数
- **错误处理**: 进程监控和优雅关闭

`deploy/bin/` 中需要的文件：
`mtxrpicam`、`libcamera.so.9.9`、`libcamera-base.so.9.9`、
`ipa_module/ipa_rpi_vc4.so`、`ipa_module/ipa_rpi_vc4.so.sign`、
`libpisp/backend_default_config.json`、`ipa_conf/`

`LD_LIBRARY_PATH` 必须包含 `deploy/bin/`，以便 mtxrpicam 找到捆绑的 libcamera。
该接口支持启动/停止、参数更新和带缓冲的帧传递（15fps时2秒）。

### H.264 AUHub (`internal/h264/hub.go`)

AUHub 提供帧分发到多个消费者，采用扇出模式：

- **线程安全**: 内部互斥锁用于并发订阅者管理
- **非阻塞传递**: 丢弃帧以防止写入阻塞
- **订阅者管理**: 在上下文取消时自动清理
- **访问单元格式**: H.264 访问单元，带有时间戳和关键帧检测

消费者包括：
- RTSP 服务器用于视频流媒体
- 快照处理器用于 JPEG 捕获
- RTMP 推送用于云服务

### RTSP 服务器 (`internal/rtsp/server.go`)

RTSP 服务器基于 `gortsplib v5` 构建，用于 H.264 流媒体：

- **协议支持**: RTSP 1.0，支持 DESCRIBE、SETUP、PLAY 命令
- **身份验证**: 可选的摘要身份验证，用于流访问
- **按需流媒体**: 仅在客户端连接时开始帧消费
- **媒体描述**: 动态 H.264 格式，带有 SPS/PPS 更新
- **时间戳同步**: NTP 调整的时间戳，用于准确播放

主要功能：
- 单端口 RTSP 服务
- 客户端连接管理
- RTP 数据包编码和传输
- 流资源清理

### 数字 PTZ (`internal/ptz/state.go`)

虚拟 PTZ 实现，基于软件定位：

- **位置系统**: 平移 [-1,1]，倾斜 [-1,1]，缩放 [0,1] 坐标范围
- **移动模式**:
  - 连续：基于速度的移动，50ms 更新
  - 绝对：指数缓动到目标位置
  - 相对：立即增量定位
- **预设管理**: 命名位置存储和调用
- **状态管理**: 线程安全的位置跟踪和状态报告

PTZ 操作映射到相机裁剪参数，用于数字变焦，无需硬件更改。

## 数据流管道

```
OV5647 相机 → mtxrpicam → H.264 NALU → 解析器 → AUHub → 订阅者
                                       ↓
                             ┌─────────────┼─────────────┐
                             │             │             │
                       RTSP 服务器     快照处理器      RTMP 推送
                       (gortsplib v5)  (FFmpeg → JPEG)  (回环)
```

1. **捕获**: mtxrpicam 子进程从 OV5647 CSI 相机捕获帧
2. **传输**: H.264 数据通过二进制管道传输到 Go 进程
3. **处理**: 解析器提取 NALU 和时间戳，检测关键帧
4. **分发**: AUHub 将访问单元分发给多个消费者
5. **流媒体**: RTSP 服务器通过 gortsplib 向 NVR 客户端提供视频
6. **快照**: FFmpeg 子进程按需将 H.264 关键帧转换为 JPEG
7. **控制**: ONVIF 服务提供相机控制和发现功能

## 资源使用

实测于 Raspberry Pi 3B 720p@15fps：

| 进程 | RSS 内存 | 用途 |
|---------|----------|------|
| rpi-cam | ~9MB | Go 主进程（ONVIF + RTSP + 管道） |
| mtxrpicam | ~10MB | 相机捕获子进程 |
| **总计** | **~20MB** | |

- **CPU**: rpi-cam ~2%，mtxrpicam ~12%，720p@15fps
- **网络**: 720p@15fps H.264 流 ~2Mbps

## 依赖项

- **gortsplib v5**: RTSP 服务器功能（与 MediaMTX 相同的库）
- **pion/rtp**: H.264 流媒体的 RTP 数据包处理
- **yaml.v3**: 配置文件解析
- **onvif-go**: ONVIF 服务器实现（通过研究间接依赖）
- **mtxrpicam**: 相机捕获子进程，附带捆绑的 libcamera（来自 bluenviron/mediamtx-rpicamera v2.6.0）
- **FFmpeg**: 快照端点按需 JPEG 转换（设备上需安装）

## 部署架构

系统作为单个 systemd 服务运行，具有：

- **进程隔离**: 相机捕获在子进程中，主服务在 Go 进程中
- **资源使用**: RPi 3B 实测 ~20MB RAM
- **交叉编译**: 从 x86 工作站编译到 aarch64 RPi
- **配置**: 基于 YAML 的配置，支持环境变量覆盖
- **监控**: 用于操作可见性的 Prometheus 指标

### 摄像头捕获依赖

| 组件 | 类型 | 大小 | 用途 |
|------|------|------|------|
| mtxrpicam | C 二进制 (arm64) | 1.7MB | 相机捕获 + H.264 编码 |
| libcamera.so.9.9 | 共享库 (捆绑) | 5.7MB | 相机框架（来自 mediamtx-rpicamera） |
| libcamera-base.so.9.9 | 共享库 (捆绑) | 140KB | libcamera 基础支持 |
| ipa_module/ipa_rpi_vc4.so | IPA 模块 | 690KB | RPi VC4 图像处理 |
| libpisp/backend_default_config.json | 配置 | 11KB | PiSP 后端配置 |

这些依赖从 mediamtx-rpicamera 发布版捆绑，不依赖系统安装的 libcamera。
这避免了 Debian 的 libcamera (0.7.0) 与 mtxrpicam 编译版本之间的版本冲突。

此架构完全替代 MediaMTX，以提供 ONVIF 合规性，同时保持经过验证的相机捕获和 RTSP 流媒体组件。