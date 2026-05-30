package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// CameraConfig holds camera capture settings.
type CameraConfig struct {
	Device      string  `yaml:"device"`       // Camera device path
	Width       int     `yaml:"width"`        // Capture width in pixels
	Height      int     `yaml:"height"`       // Capture height in pixels
	FPS         int     `yaml:"fps"`          // Frames per second
	Codec       string  `yaml:"codec"`        // Video codec (h264)
	Bitrate     int     `yaml:"bitrate"`      // Target bitrate in bps
	Brightness  float64 `yaml:"brightness"`   // -1.0 to 1.0
	Contrast    float64 `yaml:"contrast"`     // 0.0 to 32.0
	Saturation  float64 `yaml:"saturation"`   // 0.0 to 32.0
	Sharpness   float64 `yaml:"sharpness"`    // 0.0 to 16.0
}

// RTSPConfig holds RTSP server settings.
type RTSPConfig struct {
	Port     int    `yaml:"port"`     // RTSP port
	Username string `yaml:"username"` // RTSP authentication username
	Password string `yaml:"password"` // RTSP authentication password
}

// ONVIFConfig holds ONVIF server settings.
type ONVIFConfig struct {
	Port     int    `yaml:"port"`     // ONVIF HTTP port
	Username string `yaml:"username"` // ONVIF WS-UsernameToken username
	Password string `yaml:"password"` // ONVIF WS-UsernameToken password
}

// RTMPConfig holds RTMP push settings.
type RTMPConfig struct {
	Enabled bool   `yaml:"enabled"` // Enable RTMP push
	URL     string `yaml:"url"`     // RTMP push URL
}

// DeviceConfig holds ONVIF device information.
type DeviceConfig struct {
	Name         string `yaml:"name"`         // Camera friendly name
	Manufacturer string `yaml:"manufacturer"` // Device manufacturer
	Model        string `yaml:"model"`        // Device model
	Firmware     string `yaml:"firmware"`     // Firmware version
	HardwareID   string `yaml:"hardware_id"`  // Hardware identifier
	SerialNumber string `yaml:"serial_number"` // Device serial number
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level string `yaml:"level"` // Log level (debug, info, warn, error)
}

// Config is the top-level configuration for rpi-cam.
type Config struct {
	Camera  CameraConfig  `yaml:"camera"`
	RTSP    RTSPConfig    `yaml:"rtsp"`
	ONVIF   ONVIFConfig   `yaml:"onvif"`
	RTMP    RTMPConfig    `yaml:"rtmp"`
	Device  DeviceConfig  `yaml:"device"`
	Logging LoggingConfig `yaml:"logging"`
}

// DefaultConfig returns a Config with all default values.
func DefaultConfig() *Config {
	return &Config{
		Camera: CameraConfig{
			Device:     "/dev/video0",
			Width:      1280,
			Height:     720,
			FPS:        15,
			Codec:      "h264",
			Bitrate:    2_000_000,
			Brightness: 0.0,
			Contrast:   1.0,
			Saturation: 1.0,
			Sharpness:  1.0,
		},
		RTSP: RTSPConfig{
			Port:     8554,
			Username: "",
			Password: "",
		},
		ONVIF: ONVIFConfig{
			Port:     8080,
			Username: "admin",
			Password: "",
		},
		RTMP: RTMPConfig{
			Enabled: false,
			URL:     "rtmp://push-server/app/stream",
		},
		Device: DeviceConfig{
			Name:         "Pi Camera V1",
			Manufacturer: "Raspberry Pi",
			Model:        "OV5647",
			Firmware:     "1.0.0",
			HardwareID:   "OV5647",
			SerialNumber: "",
		},
		Logging: LoggingConfig{
			Level: "info",
		},
	}
}

// Load reads a YAML configuration file at path and returns a Config.
// Values from the file are merged over DefaultConfig().
// Environment variables with the RPICAM_ prefix override both.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	applyEnvOverrides(cfg)

	return cfg, nil
}

// applyEnvOverrides applies RPICAM_ prefixed environment variables to config fields.
// Environment variable names follow the pattern RPICAM_<SECTION>_<FIELD>.
func applyEnvOverrides(cfg *Config) {
	// Camera section
	overrideString("RPICAM_CAMERA_DEVICE", &cfg.Camera.Device)
	overrideInt("RPICAM_CAMERA_WIDTH", &cfg.Camera.Width)
	overrideInt("RPICAM_CAMERA_HEIGHT", &cfg.Camera.Height)
	overrideInt("RPICAM_CAMERA_FPS", &cfg.Camera.FPS)
	overrideString("RPICAM_CAMERA_CODEC", &cfg.Camera.Codec)
	overrideInt("RPICAM_CAMERA_BITRATE", &cfg.Camera.Bitrate)
	overrideFloat("RPICAM_CAMERA_BRIGHTNESS", &cfg.Camera.Brightness)
	overrideFloat("RPICAM_CAMERA_CONTRAST", &cfg.Camera.Contrast)
	overrideFloat("RPICAM_CAMERA_SATURATION", &cfg.Camera.Saturation)
	overrideFloat("RPICAM_CAMERA_SHARPNESS", &cfg.Camera.Sharpness)

	// RTSP section
	overrideInt("RPICAM_RTSP_PORT", &cfg.RTSP.Port)
	overrideString("RPICAM_RTSP_USERNAME", &cfg.RTSP.Username)
	overrideString("RPICAM_RTSP_PASSWORD", &cfg.RTSP.Password)

	// ONVIF section
	overrideInt("RPICAM_ONVIF_PORT", &cfg.ONVIF.Port)
	overrideString("RPICAM_ONVIF_USERNAME", &cfg.ONVIF.Username)
	overrideString("RPICAM_ONVIF_PASSWORD", &cfg.ONVIF.Password)

	// RTMP section
	overrideBool("RPICAM_RTMP_ENABLED", &cfg.RTMP.Enabled)
	overrideString("RPICAM_RTMP_URL", &cfg.RTMP.URL)

	// Device section
	overrideString("RPICAM_DEVICE_NAME", &cfg.Device.Name)
	overrideString("RPICAM_DEVICE_MANUFACTURER", &cfg.Device.Manufacturer)
	overrideString("RPICAM_DEVICE_MODEL", &cfg.Device.Model)
	overrideString("RPICAM_DEVICE_FIRMWARE", &cfg.Device.Firmware)
	overrideString("RPICAM_DEVICE_HARDWAREID", &cfg.Device.HardwareID)
	overrideString("RPICAM_DEVICE_SERIALNUMBER", &cfg.Device.SerialNumber)

	// Logging section
	overrideString("RPICAM_LOGGING_LEVEL", &cfg.Logging.Level)
}

func overrideString(envName string, dest *string) {
	if v, ok := os.LookupEnv(envName); ok {
		*dest = v
	}
}

func overrideInt(envName string, dest *int) {
	if v, ok := os.LookupEnv(envName); ok {
		if n, err := strconv.Atoi(v); err == nil {
			*dest = n
		}
	}
}

func overrideFloat(envName string, dest *float64) {
	if v, ok := os.LookupEnv(envName); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			*dest = f
		}
	}
}

func overrideBool(envName string, dest *bool) {
	if v, ok := os.LookupEnv(envName); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			*dest = b
		}
	}
}
