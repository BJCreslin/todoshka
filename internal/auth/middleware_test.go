package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const testKey = "user"

func TestMiddlewareRejectsMissing(t *testing.T) {
	h := RequireUser("secretsecretsecretsecret", testKey)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != http.StatusUnauthorized { t.Fatalf("got %d, want 401", rec.Code) }
}

func TestMiddlewareAcceptsValid(t *testing.T) {
	tok, _ := IssueToken(7, "bob", "secretsecretsecretsecret", 60_000_000_000)
	var id int64; var name string
	h := RequireUser("secretsecretsecretsecret", testKey)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context(), testKey)
		id, name = u.ID, u.Username
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK { t.Fatalf("got %d", rec.Code) }
	if id != 7 || name != "bob" { t.Fatalf("got %d %s", id, name) }
}
