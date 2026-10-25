package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	m := NewManager("secret")
	hash, err := m.HashPassword("hunter2")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "hunter2" {
		t.Fatal("password stored in plaintext")
	}
	if !m.CheckPassword(hash, "hunter2") {
		t.Error("CheckPassword should accept correct password")
	}
	if m.CheckPassword(hash, "wrong") {
		t.Error("CheckPassword should reject wrong password")
	}
}

func TestIssueParseToken(t *testing.T) {
	m := NewManager("secret")
	token, err := m.IssueToken(42, "gopher")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != 42 || claims.Login != "gopher" {
		t.Errorf("claims = %+v, want userID 42 login gopher", claims)
	}
}

func TestParseTokenRejects(t *testing.T) {
	m := NewManager("secret")
	other := NewManager("another-secret")

	token, err := m.IssueToken(1, "u")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	if _, err := other.ParseToken(token); err == nil {
		t.Error("token signed with another secret must be rejected")
	}
	if _, err := m.ParseToken("garbage"); err == nil {
		t.Error("garbage token must be rejected")
	}
	if _, err := m.ParseToken(""); err == nil {
		t.Error("empty token must be rejected")
	}
}

func TestParseTokenExpires(t *testing.T) {
	m := NewManager("secret")

	future := time.Now().Add(TokenTTL + time.Hour)
	m.now = func() time.Time { return future }

	raw, err := NewManager("secret").IssueToken(1, "u")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	mPast := NewManager("secret")
	past := time.Now().Add(-2 * TokenTTL)
	mPast.now = func() time.Time { return past }
	expired, err := mPast.IssueToken(1, "u")
	if err != nil {
		t.Fatalf("IssueToken past: %v", err)
	}

	if _, err := m.ParseToken(expired); err == nil {
		t.Error("expired token must be rejected")
	}
	_ = raw
}

func TestExtractToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, err := ExtractToken(req); err == nil {
		t.Error("missing token must yield error")
	}

	req.Header.Set("Authorization", "Bearer abc.def.ghi")
	tok, err := ExtractToken(req)
	if err != nil || tok != "abc.def.ghi" {
		t.Errorf("ExtractToken from header = %q, %v", tok, err)
	}

	req.AddCookie(&http.Cookie{Name: CookieName, Value: "cookie-token"})
	tok, err = ExtractToken(req)
	if err != nil || tok != "cookie-token" {
		t.Errorf("ExtractToken should prefer cookie, got %q, %v", tok, err)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	if _, err := ExtractToken(req2); err == nil {
		t.Error("non-Bearer Authorization must yield error")
	}
}

func TestMiddleware(t *testing.T) {
	m := NewManager("secret")
	var gotID int64
	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		id, ok := UserIDFromContext(r.Context())
		if !ok {
			t.Error("user id missing in context")
		}
		gotID = id
		w.WriteHeader(http.StatusOK)
	})
	h := m.Middleware(next)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no token: code = %d, want 401", rec.Code)
	}

	token, err := m.IssueToken(7, "alice")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !called || gotID != 7 {
		t.Errorf("valid token: code=%d called=%v id=%d", rec.Code, called, gotID)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer nope")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("bad token: code = %d, want 401", rec.Code)
	}
}

func TestSetCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	SetCookie(rec, "tok")
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != CookieName || cookies[0].Value != "tok" {
		t.Errorf("unexpected cookies: %+v", cookies)
	}
	if !cookies[0].HttpOnly {
		t.Error("auth cookie must be HttpOnly")
	}
}

func TestUserIDFromContextMissing(t *testing.T) {
	if _, ok := UserIDFromContext(httptest.NewRequest(http.MethodGet, "/", nil).Context()); ok {
		t.Error("expected ok=false for empty context")
	}
}
