package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func createTask(t *testing.T, mux http.Handler, tok, body string) int64 {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var r struct {
		ID int64
	}
	json.Unmarshal(rec.Body.Bytes(), &r)
	return r.ID
}

func itoa(i int64) string { return strconv.FormatInt(i, 10) }

func TestTaskCRUDFlow(t *testing.T) {
	d, secret := setup(t)
	mux := http.NewServeMux()
	Mount(mux, d, secret)
	tok := registerAs(t, mux, "alice", "hunter2hunter2")
	tid := createTask(t, mux, tok, `{"title":"Buy milk","priority":"high"}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("list: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("PATCH", "/api/tasks/"+itoa(tid), strings.NewReader(`{"status":"done"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("update: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("DELETE", "/api/tasks/"+itoa(tid), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Fatalf("delete: %d", rec.Code)
	}
}

func TestSubtasksAndTags(t *testing.T) {
	d, secret := setup(t)
	mux := http.NewServeMux()
	Mount(mux, d, secret)
	tok := registerAs(t, mux, "alice", "hunter2hunter2")
	tid := createTask(t, mux, tok, `{"title":"A"}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/tasks/"+itoa(tid)+"/subtasks", strings.NewReader(`{"title":"step 1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("subtask: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/tasks/"+itoa(tid)+"/tags", strings.NewReader(`{"tag":"work"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Fatalf("tag add: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/tasks/"+itoa(tid), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("get: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"tags":["work"]`) {
		t.Fatalf("body: %s", rec.Body.String())
	}
}

func TestShareFlow(t *testing.T) {
	d, secret := setup(t)
	mux := http.NewServeMux()
	Mount(mux, d, secret)
	tokA := registerAs(t, mux, "alice", "hunter2hunter2")
	tokB := registerAs(t, mux, "bob", "bobpassbobpass")
	tid := createTask(t, mux, tokA, `{"title":"A"}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/tasks/"+itoa(tid), nil)
	req.Header.Set("Authorization", "Bearer "+tokB)
	mux.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/share", strings.NewReader(`{"resource_type":"task","resource_id":`+itoa(tid)+`,"username":"bob"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokA)
	mux.ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Fatalf("share: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/tasks/"+itoa(tid), nil)
	req.Header.Set("Authorization", "Bearer "+tokB)
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
