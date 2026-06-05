[English](../deployment.md)

# RPi-CAM ONVIF 服务器部署指南

本指南涵盖 rpi-cam ONVIF 相机服务在 Raspberry Pi 上的部署，包括从 MediaMTX 迁移以及与 NVR 系统的集成。

## 前置要求

### 目标设备要求
- Raspberry Pi 3B（或其他 ARM64 设备）
- 已连接的相机模块（OV5647、IMX219、IMX477 或 USB）
- Debian 13（trixie），内核 6.12.75+rpt-rpi-v8
- 最少 905MB 内存
- 具有 sudo NOPASSWD 权限的用户账户
- 连接到目标 RPi 的网络访问
- FFmpeg 安装在设备上（用于 HLS 流媒体和快照）

### 工作站要求
- 已安装 Go 1.26+
- 可访问目标设备的 SSH
- 具备 SCP 文件传输功能
## 摄像头捕获依赖

### mtxrpicam 二进制文件包

rpi-cam 使用来自 [mediamtx-rpicamera](https://github.com/bluenviron/mediamtx-rpicamera) 项目的 `mtxrpicam` 进行摄像头捕获。此二进制文件捆绑了自己的 `libcamera` 共享库，以避免与系统安装的 libcamera 产生版本冲突。

**`deploy/bin/` 中所需的文件：**
```
deploy/bin/
├── mtxrpicam                        # 摄像头捕获 + H.264 编码二进制文件 (1.7MB)
├── libcamera.so.9.9                  # 捆绑的 libcamera (5.7MB)
├── libcamera-base.so.9.9             # 捆绑的 libcamera base (140KB)
├── ipa_module/
│   ├── ipa_rpi_vc4.so                # VC4 ISP 模块
│   └── ipa_rpi_vc4.so.sign           # 模块签名
├── libpisp/
│   └── backend_default_config.json   # PiSP 后端配置
└── ipa_conf/                         # IPA 调优文件
```

**安装 mtxrpicam 包：**
```bash
# 从 mediamtx-rpicamera 下载最新发布版本
# 适用于 arm64（RPi 3B/4/5）：
gh release download v2.6.0 --repo bluenviron/mediamtx-rpicamera \
  --pattern "mtxrpicam_64.tar.gz" --dir /tmp
tar xzf /tmp/mtxrpicam_64.tar.gz -C /tmp/

# 部署到目标设备
scp -r /tmp/mtxrpicam_64/* <your-rpi-user>@<your-rpi-ip>:~/rpi-cam/deploy/bin/
scp /tmp/mtxrpicam_64/mtxrpicam <your-rpi-user>@<your-rpi-ip>:~/rpi-cam/deploy/bin/mtxrpicam
```

**为什么捆绑 libcamera？** Debian 13 附带的 libcamera 0.7.0（`libcamera.so.0.7`），但 mtxrpicam 编译时链接的是特定版本的 libcamera（`libcamera.so.9.9`）。捆绑的库避免了这种版本不匹配。如果遇到 `encoder_create(): unable to activate output stream` 错误，请确保捆绑的库文件在 `deploy/bin/` 中，并且 `LD_LIBRARY_PATH` 设置正确。

**额外的设备依赖：**
```bash
# FFmpeg 用于快照 JPEG 转换
ssh <your-rpi-user>@<your-rpi-ip> 'sudo apt install -y ffmpeg'
```
## 构建过程

### 从工作站交叉编译

```bash
# 克隆仓库
git clone https://github.com/Mi-Bee-Studio/raspberrypi-camera
cd raspberrypi-camera

# 构建 ARM64 架构版本
make build GOOS=linux GOARCH=arm64

# 验证二进制文件创建
ls -la build/rpi-cam
```

## 安装步骤

### 1. 准备目标设备

```bash
# 在目标设备上创建工作目录
ssh <your-rpi-user>@<your-rpi-ip> "mkdir -p ~/rpi-cam"

# 停止 MediaMTX 以释放相机访问权限
ssh <your-rpi-user>@<your-rpi-ip> 'sudo systemctl stop mediamtx'
ssh <your-rpi-user>@<your-rpi-ip> 'sudo systemctl disable mediamtx'
```

### 2. 部署文件

```bash
# 复制二进制文件和配置
scp build/rpi-cam <your-rpi-user>@<your-rpi-ip>:~/rpi-cam/
scp configs/config.example.yaml <your-rpi-user>@<your-rpi-ip>:~/rpi-cam/config.yaml

# 复制 systemd 服务单元文件
scp deploy/rpi-cam.service <your-rpi-user>@<your-rpi-ip>:/tmp/

# 复制摄像头捕获依赖
scp -r deploy/bin/ <your-rpi-user>@<your-rpi-ip>:~/rpi-cam/deploy/
```

### 3. 安装 Systemd 服务

```bash
# 安装服务并启用
ssh <your-rpi-user>@<your-rpi-ip> "sudo mv /tmp/rpi-cam.service /etc/systemd/system/"
ssh <your-rpi-user>@<your-rpi-ip> "sudo systemctl daemon-reload"
ssh <your-rpi-user>@<your-rpi-ip> "sudo systemctl enable rpi-cam"
```

> **注意：** systemd 服务单元中设置了 `Environment=LD_LIBRARY_PATH=/home/mickey/rpi-cam/deploy/bin`，以便运行时找到捆绑的 libcamera 库。如果您安装到不同路径，请相应更新服务文件中的此值。

### 4. 自动化部署

```bash
# 运行自动化部署脚本
./deploy/deploy.sh

# 脚本会自动执行：
# 1. 停止并禁用 MediaMTX
# 2. 部署 rpi-cam 二进制文件和配置
# 3. 安装 systemd 服务
# 4. 启用服务
```

## 配置

### 配置文件设置

```bash
# 复制示例配置
cp configs/config.example.yaml config.yaml

# 编辑配置以适配您的环境
nano config.yaml
```

### 关键配置部分

```yaml
# 相机设置
camera:
  device: /dev/video0
  width: 1280
  height: 720
  fps: 15
  codec: h264
  bitrate: 2000000

# ONVIF 设置
onvif:
  port: 8080
  username: "admin"
  password: "your-password"  # 必须：设置安全密码

# 设备信息
device:
  name: "Pi Camera V1"
  manufacturer: "Raspberry Pi"
  model: "OV5647"
  firmware: "1.0.0"

# 日志
logging:
  level: "info"  # debug, info, warn, error
```

### 环境变量

```bash
# 使用特定密码启动
RPICAM_ONVIF_PASSWORD=secret123 ./build/rpi-cam -config config.yaml

# 调试日志
RPICAM_LOGGING_LEVEL=debug ./build/rpi-cam -config config.yaml
```

## 启动服务

### Systemd 服务管理

```bash
# 启动服务
sudo systemctl start rpi-cam

# 检查状态
systemctl status rpi-cam

# 查看日志
journalctl -u rpi-cam -f

# 启用开机自启动
sudo systemctl enable rpi-cam

# 重启服务
sudo systemctl restart rpi-cam

# 停止服务
sudo systemctl stop rpi-cam
```

## 验证

### 1. 服务状态检查

```bash
# 检查服务是否运行
systemctl status rpi-cam

# 验证日志中无错误
journalctl -u rpi-cam --since "5 minutes ago"
```

### 2. RTSP 流测试

```bash
# 使用 FFmpeg 测试 RTSP 流
ffmpeg -rtsp_transport tcp -i rtsp://<your-rpi-ip>:8554/stream -t 10 test.mp4

# 或使用 VLC
vlc rtsp://<your-rpi-ip>:8554/stream
```

在设备本机上测试：
```bash
ffprobe -rtsp_transport tcp -i rtsp://localhost:8554/stream
# 预期输出：Stream #0:0: Video: h264 (High), yuv420p, 1280x720, 15 fps
```

### 3. ONVIF 发现测试

```bash
# 使用 onvif-scan 测试 WS-Discovery
onvif-scan --host <your-rpi-ip> --port 8080

# 测试 HTTP POST 发现
curl -X POST http://<your-rpi-ip>:8080/onvif/device_service \
  -H "Content-Type: application/soap+xml" \
  -d @probe.xml
```

### 4. MiBee NVR 集成测试

```bash
# 在 MiBee NVR 网页界面中添加相机：
# 1. 导航到 NVR 网页界面（使用配置的管理员凭据）
# 2. 进入 相机管理 → 添加相机
# 3. 选择 ONVIF 协议
# 4. 输入相机 IP 地址和凭据
# 5. 验证相机显示在线并传输视频
```

### 5. 快照测试

```bash
# 测试快照端点
curl -s http://<your-rpi-ip>:8080/snapshot -o snapshot.jpg
file snapshot.jpg
# 预期输出：JPEG image data, baseline, precision 8, 1280x720
```

### 6. Web UI 访问测试
```bash
# 打开 Web 管理面板
# http://<your-rpi-ip>:8088/

# 使用 ONVIF 凭据登录（或配置的 Web 专用凭据）

# 验证 HLS 实时预览加载（检查浏览器控制台 hls.js 错误）

# 测试快照按钮
curl -s http://<your-rpi-ip>:8080/snapshot -o snapshot.jpg
file snapshot.jpg
# 预期：JPEG 图像数据
```

> 注意：Web UI 嵌入在二进制文件中 -- 无需部署额外文件。

### 快速健康检查

```bash
# 检查内存使用（目标：总计约 20MB）
ps -o pid,rss,comm -p $(pgrep -f "rpi-cam|mtxrpicam")

# 检查 Web UI
curl -s -o /dev/null -w "%{http_code}" http://localhost:8088/

# 检查 HLS 播放列表
curl -s http://localhost:8088/hls/stream.m3u8 | head -3
```

## 常见问题故障排除

### 相机访问问题

**问题：** "Camera device busy" 错误
**解决方案：** 确保 MediaMTX 完全停止：
```bash
sudo systemctl stop mediamtx
sudo systemctl disable mediamtx
```

**问题：** 相机未检测到
**解决方案：** 检查 `/boot/firmware/config.txt` 中的 DT 覆盖层：
```
dtoverlay=ov5647  # 或 imx219, imx477 等
```

### 摄像头编码器问题

**问题：** "encoder_create(): unable to activate output stream"
**根因：** mtxrpicam 找不到捆绑的 libcamera 共享库。
**解决方案：**
```bash
# 验证捆绑的库文件是否存在
ls ~/rpi-cam/deploy/bin/libcamera*.so*

# 验证服务中的 LD_LIBRARY_PATH
grep LD_LIBRARY_PATH /etc/systemd/system/rpi-cam.service

# 如果库文件缺失，从 mediamtx-rpicamera 发布版本重新部署
# 参见上方「摄像头捕获依赖」章节
```

### 网络访问问题

**问题：** ONVIF 发现失败
**解决方案：** 检查防火墙设置：
```bash
# 检查端口 8080 是否开放
sudo ufw status

# 如需要，允许 ONVIF 端口
sudo ufw allow 8080/tcp
```

**问题：** RTSP 连接被拒绝
**解决方案：** 验证 RTSP 端口配置和服务状态：
```bash
# 检查 RTSP 服务器日志
journalctl -u rpi-cam --grep "RTSP"

# 测试端口连接性
telnet <your-rpi-ip> 8554
```

### 配置问题

**问题：** 服务启动失败
**解决方案：** 检查配置语法：
```bash
# 验证 YAML 语法
yamllint config.yaml

# 检查配置值
./build/rpi-cam --validate-config --config config.yaml
```

**问题：** 无效的 ONVIF 密码
**解决方案：** 在 config.yaml 中设置强密码：
```yaml
onvif:
  password: "secure-password-123"
```

## 维护

### 更新和升级

```bash
# 拉取最新更改
git pull origin main

# 重新构建和重新部署
make build
make deploy

# 重启服务
make service-restart
```

### 备份配置

```bash
# 备份当前配置
sudo cp /etc/systemd/system/rpi-cam.service ~/backups/
cp config.yaml ~/backups/config.yaml.$(date +%Y%m%d).backup
```

## 支持

如需额外支持：
- 查看故障排除文档
- 使用 `journalctl -u rpi-cam -f` 检查服务日志
- 使用 `--validate-config` 标志验证配置
- 使用调试日志进行测试：`RPICAM_LOGGING_LEVEL=debug`