package web

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestServer creates a Server with a fresh SessionStore for testing.
func newTestServer(user, pass string) *Server {
	return &Server{
		sessions: NewSessionStore(user, pass),
		logger:   log.New(io.Discard, "", 0),
	}
}

func TestSessionStore_LoginSuccess(t *testing.T) {
	s := newTestServer("admin", "admin123")
	token, _, err := s.sessions.Login("admin", "admin123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if token == "" {
		t.Fatal("empty token")
	}
	username, err := s.sessions.Validate(token)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if username != "admin" {
		t.Errorf("expected username 'admin', got %q", username)
	}
}

func TestSessionStore_LoginBadPassword(t *testing.T) {
	s := newTestServer("admin", "admin123")
	_, _, err := s.sessions.Login("admin", "wrong")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestSessionStore_LoginBadUsername(t *testing.T) {
	s := newTestServer("admin", "admin123")
	_, _, err := s.sessions.Login("root", "admin123")
	if err == nil {
		t.Fatal("expected error for wrong username")
	}
}

func TestSessionStore_EmptyPassword(t *testing.T) {
	s := newTestServer("admin", "")
	_, _, err := s.sessions.Login("admin", "")
	if err != nil {
		t.Fatalf("Login with empty password should work: %v", err)
	}
}

func TestSessionStore_InvalidateOnLogout(t *testing.T) {
	s := newTestServer("admin", "admin123")
	token, _, _ := s.sessions.Login("admin", "admin123")

	if _, err := s.sessions.Validate(token); err != nil {
		t.Fatalf("expected valid token before logout: %v", err)
	}

	s.sessions.Logout(token)

	if _, err := s.sessions.Validate(token); err == nil {
		t.Fatal("expected token to be invalid after logout")
	}
}

func TestAuthMiddleware_BearerHeader(t *testing.T) {
	s := newTestServer("admin", "admin123")
	token, _, _ := s.sessions.Login("admin", "admin123")

	called := false
	handler := s.authRequired(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
	if called {
		t.Error("handler should not be called without token")
	}

	req = httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 with valid token, got %d", rr.Code)
	}
	if !called {
		t.Error("handler should be called with valid token")
	}

	called = false
	req = httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rr = httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with invalid token, got %d", rr.Code)
	}
	if called {
		t.Error("handler should not be called with invalid token")
	}
}

func TestAuthMiddleware_QueryToken(t *testing.T) {
	s := newTestServer("admin", "admin123")
	token, _, _ := s.sessions.Login("admin", "admin123")

	called := false
	handler := s.authRequired(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest("GET", "/ws?token="+token, nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if !called {
		t.Error("handler should be called with ?token= query param")
	}

	req = httptest.NewRequest("GET", "/ws?token=", nil)
	rr = httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for empty ?token=, got %d", rr.Code)
	}
}

func TestHandleLogin_HTTP(t *testing.T) {
	s := newTestServer("admin", "admin123")

	body := `{"username":"admin","password":"admin123"}`
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.handleLogin(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	token, ok := resp["token"].(string)
	if !ok || token == "" {
		t.Error("expected non-empty token in response")
	}
	if resp["username"] != "admin" {
		t.Errorf("expected username=admin, got %v", resp["username"])
	}
}

func TestHandleLogin_HTTPWrongPassword(t *testing.T) {
	s := newTestServer("admin", "admin123")
	body := `{"username":"admin","password":"WRONG"}`
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.handleLogin(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestHandleLogin_HTTPEmptyFields(t *testing.T) {
	s := newTestServer("admin", "admin123")
	tests := []struct {
		name string
		body string
	}{
		{"empty username", `{"username":"","password":"admin123"}`},
		{"empty password", `{"username":"admin","password":""}`},
		{"both empty", `{"username":"","password":""}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/login", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			s.handleLogin(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", rr.Code)
			}
		})
	}
}

func TestHandleLogin_HTTPInvalidBody(t *testing.T) {
	s := newTestServer("admin", "admin123")
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.handleLogin(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestHandleLogout_HTTP(t *testing.T) {
	s := newTestServer("admin", "admin123")
	token, _, _ := s.sessions.Login("admin", "admin123")

	req := httptest.NewRequest("POST", "/api/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	s.authRequired(s.handleLogout)(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rr.Code)
	}

	_, err := s.sessions.Validate(token)
	if err == nil {
		t.Error("expected token to be invalid after logout")
	}
}

func TestExtractToken_Priorities(t *testing.T) {
	headerToken := "header-token"
	queryToken := "query-token"

	req := httptest.NewRequest("GET", "/ws?token="+queryToken, nil)
	req.Header.Set("Authorization", "Bearer "+headerToken)
	if got := extractToken(req); got != headerToken {
		t.Errorf("expected header token to win, got %q", got)
	}

	req = httptest.NewRequest("GET", "/ws?token="+queryToken, nil)
	if got := extractToken(req); got != queryToken {
		t.Errorf("expected query token, got %q", got)
	}

	req = httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Authorization", "Bearer "+headerToken)
	if got := extractToken(req); got != headerToken {
		t.Errorf("expected header token, got %q", got)
	}

	req = httptest.NewRequest("GET", "/ws", nil)
	if got := extractToken(req); got != "" {
		t.Errorf("expected empty token, got %q", got)
	}
}

func TestSessionStore_LogoutUnknownToken(t *testing.T) {
	s := newTestServer("admin", "admin123")
	s.sessions.Logout("never-issued-token")
}

func TestHandleLogin_ResponseShape(t *testing.T) {
	s := newTestServer("admin", "admin123")
	body := `{"username":"admin","password":"admin123"}`
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.handleLogin(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "json") {
		t.Errorf("expected JSON content-type, got %q", ct)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	required := []string{"token", "username", "expires_at", "expires_in"}
	for _, k := range required {
		if _, ok := resp[k]; !ok {
			t.Errorf("response missing %q field: %v", k, resp)
		}
	}
}
