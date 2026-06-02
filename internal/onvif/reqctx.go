package onvif

import (
	"context"
	"net"
)

// ctxKey is an unexported type for context keys to avoid collisions.
type ctxKey struct{}

// serverIPKey is the context key for the per-request server-side local IP
// (i.e. the IP of the RPi interface that accepted the connection).
// NVR clients need this to construct reachable XAddr/RTSP URLs.
var serverIPKey = ctxKey{}

// WithServerIP returns a derived context carrying the server's local IP for
// the current connection. This is the IP the NVR should use to reach us back,
// not the NVR's own source IP.
func WithServerIP(ctx context.Context, ip string) context.Context {
	if ip == "" {
		return ctx
	}
	return context.WithValue(ctx, serverIPKey, ip)
}

// ServerIPFromContext extracts the server-side local IP from the context.
// Returns the provided fallback if no IP was set on the context.
func ServerIPFromContext(ctx context.Context, fallback string) string {
	if v, ok := ctx.Value(serverIPKey).(string); ok && v != "" {
		return v
	}
	return fallback
}

// ServerIPFromConn derives the local IP (host portion) from a net.Conn's
// LocalAddr (e.g. "192.168.63.162:8080" -> "192.168.63.162"). Returns an
// empty string for nil conns or unparseable addresses.
func ServerIPFromConn(conn net.Conn) string {
	if conn == nil {
		return ""
	}
	addr := conn.LocalAddr()
	if addr == nil {
		return ""
	}
	return extractHost(addr.String())
}

// extractHost pulls the host portion out of a "host:port" string.
func extractHost(addr string) string {
	if addr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// Already a bare address (no port) — return as-is.
		return addr
	}
	return host
}

// ExtractClientIP pulls the host portion out of an HTTP RemoteAddr
// ("1.2.3.4:5678" -> "1.2.3.4", "[::1]:8080" -> "::1").
// Kept for tests and any future code that needs the client's source address.
func ExtractClientIP(remoteAddr string) string {
	return extractHost(remoteAddr)
}
