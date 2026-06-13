package web

import (
	"net/http/httptest"
	"testing"
)

func TestCheckOrigin_SameOrigin(t *testing.T) {
	req := httptest.NewRequest("GET", "http://192.168.63.162:8088/ws", nil)
	req.Host = "192.168.63.162:8088"
	req.Header.Set("Origin", "http://192.168.63.162:8088")

	if !checkOrigin(req) {
		t.Error("expected same-origin request to be accepted")
	}
}

func TestCheckOrigin_CrossOrigin(t *testing.T) {
	req := httptest.NewRequest("GET", "http://192.168.63.162:8088/ws", nil)
	req.Host = "192.168.63.162:8088"
	req.Header.Set("Origin", "http://evil.com")

	if checkOrigin(req) {
		t.Error("expected cross-origin request to be rejected")
	}
}

func TestCheckOrigin_NoOrigin(t *testing.T) {
	req := httptest.NewRequest("GET", "http://192.168.63.162:8088/ws", nil)
	req.Host = "192.168.63.162:8088"

	if !checkOrigin(req) {
		t.Error("expected request without Origin header to be accepted")
	}
}

func TestCheckOrigin_HTTPS(t *testing.T) {
	req := httptest.NewRequest("GET", "https://192.168.63.162:8088/ws", nil)
	req.Host = "192.168.63.162:8088"
	req.Header.Set("Origin", "https://192.168.63.162:8088")

	if !checkOrigin(req) {
		t.Error("expected same-origin HTTPS request to be accepted")
	}
}

func TestCheckOrigin_SameOriginNoPort(t *testing.T) {
	req := httptest.NewRequest("GET", "http://localhost/ws", nil)
	req.Host = "localhost"
	req.Header.Set("Origin", "http://localhost")

	if !checkOrigin(req) {
		t.Error("expected same-origin request without port to be accepted")
	}
}

func TestCheckOrigin_SameOriginDifferentPort(t *testing.T) {
	req := httptest.NewRequest("GET", "http://192.168.63.162:8088/ws", nil)
	req.Host = "192.168.63.162:8088"
	req.Header.Set("Origin", "http://192.168.63.162:9999")

	if checkOrigin(req) {
		t.Error("expected request from different port to be rejected")
	}
}
