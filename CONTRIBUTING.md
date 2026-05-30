# Contributing

Welcome! We're glad you want to contribute to the Raspberry Pi 3B Camera Service.

## Development Setup

- Go 1.26+ required
- Build: `go build ./cmd/server`
- Test: `go test ./...`
- Install dependencies: `go mod download`

## Code Style

- Use `gofmt -w` for formatting
- Run `go vet ./...` for static analysis
- Follow standard Go conventions
- No CGO dependencies
- Keep CGO_ENABLED=1 only if V4L2 capture is used

## Commit Convention

Use conventional commits:

- `feat(onvif): add WS-Discovery support`
- `fix(camera): handle device disconnect gracefully`  
- `docs: update README`
- `ci: add golangci-lint`
- `test: add camera backend unit tests`

## Pull Request Process

1. Fork the repository
2. Create feature branch
3. Make commits following conventional format
4. Push to your fork
5. Submit pull request
6. Ensure CI checks pass
7. Address review feedback

## Project Structure

```
cmd/server/          # Main application binary
internal/            # Internal packages
  - camera/         # Camera capture backends
  - onvif/          # ONVIF server implementation
  - rtsp/           # RTSP server utilities
  - config/         # Configuration management
```

## Adding Camera Support

New camera backends implement the `CameraBackend` interface:

```go
type CameraBackend interface {
    Name() string
    Detect() ([]CameraInfo, error)
    Open(config CameraConfig) (CameraDevice, error)
}
```

Add backend to `internal/camera/backends/` and register in `camera.go`.