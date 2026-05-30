# rpi-cam 故障排除指南

## English](../troubleshooting.md)

本文档涵盖了 rpi-cam（树莓 Pi ONVIF 相机服务）的常见问题和解决方案。

## 快速健康检查

```bash
# 检查 rpi-cam 是否运行
systemctl status rpi-cam

# 检查相机设备
ls -la /dev/video0

# 检查网络连接
netstat -tlnp | grep -E '8554|8080|3702'

# 检查内存使用
free -h

# 检查 CPU 使用
top -bn1 | head -20
```

## 相机检测问题

### 症状
- NVR 中找不到相机
- rpi-cam 日志显示 "camera not detected"
- 流显示黑屏

### 诊断
```bash
# 检查相机设备是否存在
ls -la /dev/video0
# 应显示 /dev/video0 字符设备

# 直接使用 libcamera 测试相机
rpicam-hello

# 检查设备树覆盖层
cat /boot/firmware/config.txt | grep dtoverlay
# 应显示：dtoverlay=ov5647

# 检查内核相机支持（如果设备通过 libcamera 工作，可忽略）
vcgencmd get_camera
```

### 解决方案
1. **MediaMTX 冲突**：首先停止 MediaMTX
   ```bash
   sudo systemctl stop mediamtx
   sudo systemctl disable mediamtx
   ```

2. **缺少 DT 覆盖层**：添加到 config.txt
   ```bash
   sudo nano /boot/firmware/config.txt
   # 添加：dtoverlay=ov5647
   sudo reboot
   ```

3. **相机模块未连接**：检查 CSI 电缆

4. **设备路径错误**：更新 config.yaml
   ```yaml
   camera:
     device: "/dev/video0"  # 或你的相机设备
   ```

## RTSP 流媒体问题

### 症状
- RTSP 流无法访问
- 连接超时
- NVR 无法连接到流

### 诊断
```bash
# 检查 RTSP 端口是否在监听
netstat -tlnp | grep 8554

# 本地测试 RTSP 连接
ffplay rtsp://localhost:8554/stream

# 检查防火墙规则
sudo ufw status

# 检查相机独占性
lsof /dev/video0
```

### 解决方案
1. **端口冲突**：在 config.yaml 中更改 RTSP 端口
   ```yaml
   rtsp:
     port: 8555  # 如需要则更改
   ```

2. **防火墙阻塞**：允许 RTSP 端口
   ```bash
   sudo ufw allow 8554/tcp
   ```

3. **相机访问冲突**：确保只有一个进程使用 /dev/video0
   ```bash
   sudo systemctl stop mediamtx  # 如果正在运行
   ```

4. **网络问题**：从客户端系统检查
   ```bash
   telnet <camera-ip> 8554
   ```

## ONVIF 发现问题

### 症状
- NVR 无法发现相机
- WS-Discovery 探测失败
- 找不到 ONVIF 设备服务

### 诊断
```bash
# 检查 ONVIF HTTP 端口
netstat -tlnp | grep 8080

# 测试 UDP 多播端口
nc -ul 3702

# 检查 ONVIF 服务日志
journalctl -u rpi-cam -f

# 手动测试 ONVIF 端点
curl -X POST http://localhost:8080/onvif/device_service
```

### 解决方案
1. **网络问题**：检查多播路由
   ```bash
   # 如果需要，启用多播
   sudo sysctl -w net.ipv4.conf.all.mc_forwarding=1
   ```

2. **端口冲突**：更改 ONVIF 端口
   ```yaml
   onvif:
     port: 8081  # 如需要则更改
   ```

3. **防火墙阻塞**：允许 ONVIF 端口
   ```bash
   sudo ufw allow 8080/tcp
   sudo ufw allow 3702/udp
   ```

4. **发现超时**：如需要，在 NVR 配置中增加超时时间

## NVR 集成问题

### 症状
- NVR 显示相机但无法添加
- GetStreamUri 失败
- 身份验证问题

### 诊断
```bash
# 检查 config.yaml 中的 ONVIF 凭据
# 手动测试 ONVIF 客户端
curl -X POST -H "Content-Type: application/soap+xml" \
  -d "<soap:Envelope>...</soap:Envelope>" \
  http://localhost:8080/onvif/device_service

# 检查 RTSP URL 格式
echo "rtsp://localhost:8554/stream"

# 检查设备信息响应
curl -s http://localhost:8080/onvif/device_service | grep -i device
```

### 解决方案
1. **身份验证**：设置 ONVIF 凭据
   ```yaml
   onvif:
     username: "admin"
     password: "your-password"
   ```

2. **无效的 RTSP URL**：确保 URL 与配置匹配
   ```yaml
   rtsp:
     username: ""  # 如果没有身份验证则留空
     password: ""
   ```

3. **配置文件问题**：检查视频编码器配置
   ```yaml
   camera:
     width: 1280
     height: 720
     codec: "h264"
   ```

4. **设备信息**：更新设备元数据
   ```yaml
   device:
     name: "我的相机"
     manufacturer: "树莓 Pi"
     model: "OV5647"
   ```

## 快照问题

### 症状
- 快照端点返回错误
- NVR 无法捕获图像
- 日志中显示 FFmpeg 错误

### 诊断
```bash
# 检查快照端点
curl -I http://localhost:8080/snapshot

# 手动测试 FFmpeg
ffmpeg -rtsp_transport tcp -i rtsp://localhost:8554/stream \
  -vf "scale=640:480" -frames:v 1 snapshot.jpg

# 检查 FFmpeg 安装
ffmpeg -version
```

### 解决方案
1. **缺少 FFmpeg**：安装 FFmpeg
   ```bash
   sudo apt install ffmpeg
   ```

2. **相机未运行**：确保 rpi-cam 处于活动状态
   ```bash
   sudo systemctl restart rpi-cam
   ```

3. **分辨率问题**：调整快照参数
   ```yaml
   camera:
     width: 1280
     height: 720
   ```

## 性能问题

### 症状
- 高内存使用
- 视频流延迟
- CPU 过载

### 诊断
```bash
# 检查内存使用
free -h

# 检查 CPU 使用
top -bn1 | grep -E 'rpi-cam|mediamtx'

# 检查 RTSP 连接
lsof -i :8554

# 检查网络带宽
ip -s link show wlan0
```

### 解决方案
1. **降低分辨率**：降低捕获设置
   ```yaml
   camera:
     width: 640
     height: 480
     fps: 10
     bitrate: 1000000  # 1 Mbps
   ```

2. **限制并发流**：添加连接限制
   ```yaml
   rtsp:
     max_clients: 5  # 限制并发观看者
   ```

3. **监控资源**：添加监控
   ```bash
   # 每 5 秒监控内存
   watch -n 5 "free -h && ps aux | grep rpi-cam"
   ```

## 调试模式和日志

### 启用调试日志
```bash
# 通过环境设置调试级别
RPICAM_LOGGING_LEVEL=debug ./rpi-cam

# 或者在 config.yaml 中
logging:
  level: "debug"
```

### 详细模式选项
```bash
# 启用 ONVIF 调试日志
RPICAM_LOGGING_LEVEL=debug ./rpi-cam -onvif-debug

# 启用 RTSP 调试日志
RPICAM_LOGGING_LEVEL=debug ./rpi-cam -rtsp-debug
```

### 日志分析技巧
```bash
# 实时查看日志
journalctl -u rpi-cam -f

# 过滤错误消息
journalctl -u rpi-cam | grep ERROR

# 查找超时模式
journalctl -u rpi-cam | grep -i timeout

# 检查资源警告
journalctl -u rpi-cam | grep -i "memory\|cpu"
```

## 常见错误消息

### 相机访问问题
```
ERROR: camera device not available
```
- 解决方案：停止 MediaMTX，检查设备路径

### 端口已使用
```
ERROR: address already in use
```
- 解决方案：在配置中更改端口或停止冲突服务

### 网络问题
```
ERROR: connection refused
```
- 解决方案：检查防火墙，网络连接

### 内存问题
```
WARNING: high memory usage detected
```
- 解决方案：降低分辨率、FPS 或比特率

## 系统状态命令

```bash
# 完整系统健康检查
echo "=== 系统状态 ==="
echo "相机设备："
ls -la /dev/video0 2>/dev/null || echo "未找到相机"

echo "服务："
systemctl is-active rpi-cam mediamtx

echo "网络："
netstat -tlnp | grep -E '8554|8080|3702'

echo "内存："
free -h

echo "相机进程："
pgrep -f rpi-cam || echo "rpi-cam 未运行"

echo "冲突进程："
lsof /dev/video0 2>/dev/null | grep -v rpi-cam || echo "无冲突"
```

## 联系支持

如果问题持续存在：
1. 检查日志：`journalctl -u rpi-cam`
2. 包含系统信息：`uname -a`，`dpkg -l | grep rpi-cam`
3. 提供确切的错误消息和重现步骤
4. 包含配置文件（删除敏感数据）