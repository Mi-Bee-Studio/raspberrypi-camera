package onvif

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestWithServerIP(t *testing.T) {
	ctx := WithServerIP(context.Background(), "10.0.0.5")
	got := ServerIPFromContext(ctx, "fallback")
	if got != "10.0.0.5" {
		t.Errorf("expected 10.0.0.5, got %q", got)
	}
}

func TestServerIPFromContextFallback(t *testing.T) {
	// Empty context returns the fallback.
	got := ServerIPFromContext(context.Background(), "fallback")
	if got != "fallback" {
		t.Errorf("expected fallback, got %q", got)
	}

	// WithServerIP with empty string is a no-op.
	ctx := WithServerIP(context.Background(), "")
	got = ServerIPFromContext(ctx, "fallback")
	if got != "fallback" {
		t.Errorf("expected fallback (empty IP shouldn't override), got %q", got)
	}
}

func TestExtractClientIP(t *testing.T) {
	tests := []struct {
		remoteAddr string
		want       string
	}{
		{"192.168.1.1:55123", "192.168.1.1"},
		{"10.0.0.5:80", "10.0.0.5"},
		{"[2001:db8::1]:8080", "2001:db8::1"},
		{"192.168.1.1", "192.168.1.1"}, // no port — return as-is
		{"", ""},                        // empty input
	}

	for _, tt := range tests {
		got := ExtractClientIP(tt.remoteAddr)
		if got != tt.want {
			t.Errorf("ExtractClientIP(%q) = %q, want %q", tt.remoteAddr, got, tt.want)
		}
	}
}

func TestServerIPFromConn(t *testing.T) {
	// Nil conn returns empty.
	if got := ServerIPFromConn(nil); got != "" {
		t.Errorf("ServerIPFromConn(nil) = %q, want empty", got)
	}

	// Real TCPAddr
	conn := &fakeConn{addr: &net.TCPAddr{IP: net.ParseIP("192.168.63.162"), Port: 8080}}
	if got := ServerIPFromConn(conn); got != "192.168.63.162" {
		t.Errorf("ServerIPFromConn = %q, want 192.168.63.162", got)
	}
}

// fakeConn lets us test ServerIPFromConn without a real network conn.
type fakeConn struct {
	addr net.Addr
}

func (f *fakeConn) Read(b []byte) (int, error)         { return 0, nil }
func (f *fakeConn) Write(b []byte) (int, error)        { return len(b), nil }
func (f *fakeConn) Close() error                       { return nil }
func (f *fakeConn) LocalAddr() net.Addr                { return f.addr }
func (f *fakeConn) RemoteAddr() net.Addr               { return nil }
func (f *fakeConn) SetDeadline(t time.Time) error      { return nil }
func (f *fakeConn) SetReadDeadline(t time.Time) error  { return nil }
func (f *fakeConn) SetWriteDeadline(t time.Time) error { return nil }
