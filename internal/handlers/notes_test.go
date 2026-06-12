package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNoteFlowWithVersions(t *testing.T) {
	d, secret := setup(t)
	mux := http.NewServeMux(); Mount(mux, d, secret)
	tok := registerAs(t, mux, "alice", "hunter2hunter2")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/notes", strings.NewReader(`{"title":"First","body_md":"# Hi"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 201 { t.Fatalf("create: %d", rec.Code) }
	var n struct{ ID int64 }
	json.Unmarshal(rec.Body.Bytes(), &n)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("PATCH", "/api/notes/"+itoa(n.ID), strings.NewReader(`{"title":"Second","body_md":"# v2"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 { t.Fatalf("update: %d", rec.Code) }

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/notes/"+itoa(n.ID)+"/versions", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	var versions []struct {
		ID    int64
		Title string
	}
	json.Unmarshal(rec.Body.Bytes(), &versions)
	if len(versions) != 1 || versions[0].Title != "First" {
		t.Fatalf("expected 1 version titled First, got %+v", versions)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/notes/"+itoa(n.ID)+"/restore/"+itoa(versions[0].ID), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 { t.Fatalf("restore: %d", rec.Code) }
	var restored struct{ Title string }
	json.Unmarshal(rec.Body.Bytes(), &restored)
	if restored.Title != "First" { t.Fatalf("got %q", restored.Title) }
}
