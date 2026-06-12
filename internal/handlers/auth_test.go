package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vladimirkreslin/todoshka/internal/db"
)

func setup(t *testing.T) (*db.DB, string) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { d.Close() })
	return d, "test-jwt-secret-test-jwt-secret"
}

func registerAs(t *testing.T, mux http.Handler, username, password string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/register", strings.NewReader(`{"username":"`+username+`","password":"`+password+`"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	var r struct{ Token string }
	json.Unmarshal(rec.Body.Bytes(), &r)
	return r.Token
}

func TestRegisterAndLogin(t *testing.T) {
	d, secret := setup(t)
	mux := http.NewServeMux()
	Mount(mux, d, secret)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/register", strings.NewReader(`{"username":"alice","password":"hunter2hunter2"}`)))
	if rec.Code != http.StatusCreated { t.Fatalf("register: %d %s", rec.Code, rec.Body.String()) }

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"username":"alice","password":"hunter2hunter2"}`)))
	if rec.Code != http.StatusOK { t.Fatalf("login: %d %s", rec.Code, rec.Body.String()) }
}

func TestRegisterRejectsShortPassword(t *testing.T) {
	d, secret := setup(t)
	mux := http.NewServeMux()
	Mount(mux, d, secret)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/register", strings.NewReader(`{"username":"x","password":"short"}`)))
	if rec.Code != http.StatusBadRequest { t.Fatalf("got %d", rec.Code) }
}

func TestRegisterRejectsDuplicate(t *testing.T) {
	d, secret := setup(t)
	mux := http.NewServeMux()
	Mount(mux, d, secret)
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/register", strings.NewReader(`{"username":"abc","password":"hunter2hunter2"}`)))
		if i == 0 && rec.Code != 201 { t.Fatalf("first: %d", rec.Code) }
		if i == 1 && rec.Code != 409 { t.Fatalf("second: %d", rec.Code) }
	}
}
