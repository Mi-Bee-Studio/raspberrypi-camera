[English](../onvif-compliance.md)
#TR
#TR|# ONVIF 合规性参考
#KM|
#KW|本文档提供了 rpi-cam ONVIF 相机服务的详细合规性信息。该实现提供了 ONVIF Device/Media/PTZ/Imaging 服务、WS-Discovery 支持和 WS-Security 身份验证，用于 NVR 集成。
#RW|
#YM|## ONVIF Profile S 合规性
#SY|
#MW|| 组件 | 状态 | 备注 |
#XM||-----------|--------|-------|
#VY|| **ONVIF Device 服务** | ✅ 完整 | WS-Discovery、GetDeviceInformation、GetCapabilities |
#WZ|| **ONVIF Media 服务** | ✅ 完整 | GetProfiles、GetStreamUri、VideoSource 支持 |
#HW|| **ONVIF PTZ 服务** | ✅ 虚拟 | 仅通过软件裁切的数字 PTZ |
#YP|| **ONVIF Imaging 服务** | ✅ 完整 | 亮度、对比度、饱和度、锐度、曝光、白平衡 |
#QZ|| **WS-Discovery** | ✅ 完整 | UDP 组播 + HTTP POST 探测支持 |
#BT|| **WS-Security** | ✅ 完整 | UsernameToken Text 和 Digest 身份验证 |
#RJ|
#PV|## 支持的服务
#HX|
#XZ|### Device 服务
#YT|
#QY|| 操作 | 已实现 | 备注 |
#XQ||-----------|-------------|-------|
#HY|| `GetSystemDateAndTime` | ✅ | 返回 UTC 时间，手动模式（无 NTP 同步） |
#JS|| `GetDeviceInformation` | ✅ | 制造商、型号、固件、序列号、硬件 ID 来自配置 |
|| `GetCapabilities` | ✅ | 广告 Media、PTZ、Device、Imaging 服务 |
#QX|| `GetServices` | ✅ | 列出所有 ONVIF 服务及 XAddr 端点 |
#VP|| `GetScopes` | ✅ | 名称、硬件和类型范围 |
#JJ|
#PS|**设备信息响应：**
#VY|```yaml
#NK|Manufacturer: "Raspberry Pi"
#NW|Model: "Camera V1" 
#ZK|Firmware: "rpi-cam-v1.0.0"
#NJ|SerialNumber: "OV5647-SERIAL"
#SY|HardwareId: "OV5647"
#XS|```
#MV|
#QR|**功能响应：**
#VY|```yaml
#VK|Device: XAddr: "http://<相机IP>:8080/onvif/device_service"
#YR|Media: XAddr: "http://<相机IP>:8080/onvif/media_service"  
#NY|PTZ: XAddr: "http://<相机IP>:8080/onvif/ptz_service"
Imaging: XAddr: "http://<相机IP>:8080/onvif/device_service"
#KW|```
#QB|
#JY|### Media 服务
#KT|
#QY|| 操作 | 已实现 | 备注 |
#BM||-----------|-------------|-------|
#RR|| `GetProfiles` | ✅ | 单个配置文件，包含 H.264 编码 |
#YB|| `GetStreamUri` | ✅ | 返回 RTSP URL（rtsp://host:port/stream） |
#PT|| `GetVideoSources` | ✅ | 单个视频源（Pi Camera） |
#PZ|
#XX|**配置文件配置：**
#VY|```yaml
#VN|Profiles:
#JZ|  - Token: "main"
#JS|    Name: "main"
#KK|    VideoSourceConfiguration:
#WY|      Token: "videoSrc0"
#QS|      Bounds: { x: 0, y: 0, width: 1280, height: 720 }
#HZ|    VideoEncoderConfiguration:
#VN|      Token: "enc0"
#SV|      Encoding: "H264"
#YP|      Resolution: { Width: 1280, Height: 720 }
#SY|      RateControl: { FrameRateLimit: 15, BitrateLimit: 2000000 }
#PV|```
#JQ|
#SS|**流 URI 响应：**
#VY|```yaml
#JZ|MediaUri:
#WP|  Uri: "rtsp://<相机IP>:8554/stream"
#RB|  InvalidAfterConnect: "false"
#NH|  InvalidAfterReboot: "false"
#TX|  Timeout: "PT0S"
#KJ|```
#SZ|
#RZ|### Imaging 服务
#VB|
#QY|| 操作 | 已实现 | 备注 |
#YS||-----------|-------------|-------|
#QN|| `GetImagingSettings` | ✅ | 当前相机参数 |
#SJ|| `SetImagingSettings` | ✅ | 亮度、对比度、饱和度、锐度、曝光 |
#TK|| `GetOptions` | ✅ | 参数范围和可用模式 |
#YX|
#WY|**支持的成像参数：**
#XN|- **亮度**：-1.0 到 1.0（实时调整）
#MH|- **对比度**：0.0 到 32.0（实时调整）
#MS|- **饱和度**：0.0 到 32.0（实时调整）
#SS|- **锐度**：0.0 到 16.0（实时调整）
#BZ|- **曝光**：AUTO/MANUAL 模式，0.0 到 10000 μs 曝光时间
#JS|- **白平衡**：AUTO/MANUAL 模式，0.0 到 1.0 Cr/Cb 增益
#RT|
#SB|**成像设置响应：**
#VY|```yaml
#PY|Settings:
#PH|  Brightness: { Value: 0.0 }
#ZK|  Contrast: { Value: 1.0 }
#JW|  Saturation: { Value: 1.0 }
#QZ|  Sharpness: { Value: 0.0 }
#XQ|  Exposure: { Mode: "AUTO", Time: 16667 }
#KK|  WhiteBalance: { Mode: "AUTO" }
#ZR|```
#PJ|
#ZN|### PTZ 服务
#NJ|
#QY|| 操作 | 已实现 | 备注 |
#SY||-----------|-------------|-------|
#JQ|| `ContinuousMove` | ✅ | 带动量的平移/倾斜/缩放 |
#RM|| `AbsoluteMove` | ✅ | 移动到特定坐标 |
#NP|| `RelativeMove` | ✅ | 按相对偏移移动 |
#JK|| `Stop` | ✅ | 停止所有 PTZ 移动 |
#JN|| `GetStatus` | ✅ | 当前位置和移动状态 |
#WP|| `GetPresets` | ✅ | 列出存储的预设 |
#XS|| `SetPreset` | ✅ | 将当前位置存储为预设 |
#PS|| `GotoPreset` | ✅ | 移动到存储的预设 |
#PH|| `RemovePreset` | ✅ | 删除存储的预设 |
#XS|| `GetNodes` | ✅ | PTZ 节点配置 |
#VS|| `GetConfigurations` | ✅ | PTZ 配置选项 |
#RM|
#PB|**PTZ 坐标空间：**
#XH|- **绝对坐标**：Pan（-1.0 到 1.0）、Tilt（-1.0 到 1.0）、Zoom（0.0 到 1.0）
#YW|- **相对坐标**：Pan（-1.0 到 1.0）、Tilt（-1.0 到 1.0）、Zoom（-1.0 到 1.0）
#QX|- **连续移动**：Pan（-1.0 到 1.0）、Tilt（-1.0 到 1.0）、Zoom（0.0 到 1.0）
#WY|
#TP|**状态响应：**
#VY|```yaml
#WJ|PTZStatus:
#PJ|  PanTilt:
#XP|    Position: 0.0
#VB|    MoveStatus: "IDLE"
#NS|  Zoom:
#HT|    Position: 1.0
#VB|    MoveStatus: "IDLE"
#QB|```
#QZ|
#BV|## WS-Discovery 支持
#NQ|
#KT|服务支持两种 WS-Discovery 探测方法：
#KK|
#RS|### UDP 组播 (239.255.255.250:3702)
#ZH|- 在组播地址上侦听 Probe 消息
#YX|- 发送包含设备元数据的 ProbeMatches 响应
#YZ|- 自动检测本地 IP 用于 XAddr 生成
#RS|
#RY|### HTTP POST 探测 (/onvif/device_service)
#YN|- 通过 HTTP POST 到设备服务端点处理 Probe 消息
#BS|- 支持通过防火墙/代理进行发现
#SX|- 与 UDP 组播相同的 XML 响应
#SS|
#MJ|**ProbeMatches 响应：**
#YP|```xml
#NV|<ProbeMatch>
#TW|  <EndpointReference>
#KT|    <Address>uuid:device-uuid-here</Address>
#RB|  </EndpointReference>
#VK|  <Scopes>onvif://www.onvif.org/name/Pi Camera V1 onvif://www.onvif.org/hardware/OV5647</Scopes>
#MS|  <XAddrs>http://<相机IP>:8080/onvif/device_service</XAddrs>
#VR|  <Types>tdn:NetworkVideoTransmitter tdn:Device</Types>
#PS|  <MetadataVersion>1</MetadataVersion>
#WH|</ProbeMatch>
#YT|```
#JB|
#VY|## WS-Security 支持
#VQ|
#NR|UsernameToken 身份验证实现了两种密码类型：
#NX|
#XK|### PasswordText 模式
#TQ|- 直接密码比较
#WX|- 在没有 Nonce 时使用
#VH|- 简单但安全性较低
#HM|
#YP|### PasswordDigest 模式  
#WZ|- SHA1 摘要：`base64(SHA1(base64(Nonce) + Created + Password))`
#SV>- 更安全，需要 Nonce 和 Created 时间戳
#XP>- 推荐生产环境使用
#BN|
#PQ|**身份验证流程：**
#XN|1. 解析 SOAP Security 头部的 UsernameToken
#YX|2. 如果存在 Nonce，计算摘要并比较
#HH|3. 如果没有 Nonce，使用直接密码比较
#BT|4. 返回包含用户名和成功状态的 AuthResult
#WS|
#KY|## 已知限制
#VB|
#WW|### 功能限制
#KH|- **单一配置文件**：只支持一个媒体配置文件（主配置文件）
#NW|- **虚拟 PTZ**：仅基于软件的平移/倾斜/缩放，无物理移动
#PS|- **无音频**：音频流未实现
#BZ|- **无事件**：事件服务不支持
#YJ|- **无分析**：视频分析不可用
#TR|- **无扩展**：ONVIF 扩展（如 Analytics、Search）未实现
#BH|
#PT|### 硬件约束  
#TR|- **OV5647 相机**：固定对焦，无自动对焦功能
#HX|- **无红外截止滤镜**：固定红外滤镜（不支持 NoIR 变体）
#HZ|- **无 WDR**：OV5647 不支持宽动态范围
#MB|- **无物理 PTZ**：无电机用于机械定位
#MH|
#RP|### 协议限制
#SP|- **手动系统时间**：无 NTP 同步，UTC 时间在启动时固定
#NV|- **单一视频源**：只支持一个 CSI 相机
#VX|- **无 HTTPS**：仅 HTTP（无 SSL/TLS 加密）
#BK|- **基本身份验证**：无高级身份验证方法（如证书）
#QQ|
#PH|## 已测试的 ONVIF 客户端
#ZX|
#PY|### MiBee NVR
#TR|- **协议**：ONVIF 客户端（0x524a/onvif-go 库）
#WX|- **发现**：WS-Discovery + HTTP 探测
#VM|- **身份验证**：UsernameToken Digest
#BS|- **操作**：GetDeviceInformation、GetCapabilities、GetProfiles、GetStreamUri
#YY|- **集成状态**：✅ 完全兼容
#SR|
#BP|### 兼容性说明
#BK|- 使用与服务器相同的 `0x524a/onvif-go` 库，确保协议兼容性
#NV|- 支持 UDP 和 HTTP 发现方法
#XW|- 符合 Profile S 合规性要求
#JP|- RTSP DESCRIBE 用于流验证
#WS|- 支持 SOAP 回退处理边缘情况
#MH|
#YN|## 服务端点
#HT|
#JT|| 服务 | 端点 | 协议 | 描述 |
#HV||---------|----------|----------|-------------|
#BX|| Device 服务 | `/onvif/device_service` | HTTP/SOAP | 设备管理 |
#QH|| Media 服务 | `/onvif/media_service` | HTTP/SOAP | 媒体配置文件/URI |
#XV|| PTZ 服务 | `/onvif/ptz_service` | HTTP/SOAP | PTZ 控制 |
#NW|| 快照 | `/snapshot` | HTTP | JPEG 快照 |
#HS|| RTSP 流 | `/stream` | RTSP | H.264 视频流 |
#TZ|
#YY|## 错误处理
#NQ|
#YK|服务遵循 ONVIF SOAP 1.2 错误约定：
#YZ|
#RV|- **soap:Sender**：客户端请求错误（无效操作、身份验证失败）
#QT|- **soap:Receiver**：服务器处理错误（相机访问、参数无效）
#MN|- **SOAP 错误代码**：带有描述性文本的标准错误代码
#ZY|
#XP|**错误响应示例：**
#YP|```xml
#WM|<SOAP-ENV:Fault>
#VW|  <faultcode>soap:Sender</faultcode>
#VB|  <faulttext>authentication failed: password mismatch</faulttext>
#VM|</SOAP-ENV:Fault>
#WZ|```