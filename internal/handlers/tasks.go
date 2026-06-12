package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/vladimirkreslin/todoshka/internal/auth"
	"github.com/vladimirkreslin/todoshka/internal/db"
	"github.com/vladimirkreslin/todoshka/internal/models"
)

func mountTasks(mux *http.ServeMux, d *db.DB, secret string) {
	auth := auth.RequireUser(secret, userCtxKey)
	mux.Handle("GET /api/tasks", auth(http.HandlerFunc(taskList(d))))
	mux.Handle("POST /api/tasks", auth(http.HandlerFunc(taskCreate(d))))
	mux.Handle("GET /api/tasks/{id}", auth(http.HandlerFunc(taskGet(d))))
	mux.Handle("PATCH /api/tasks/{id}", auth(http.HandlerFunc(taskUpdate(d))))
	mux.Handle("DELETE /api/tasks/{id}", auth(http.HandlerFunc(taskDelete(d))))
	mux.Handle("POST /api/tasks/{id}/subtasks", auth(http.HandlerFunc(subtaskAdd(d))))
	mux.Handle("PATCH /api/tasks/{id}/subtasks/{sid}", auth(http.HandlerFunc(subtaskUpdate(d))))
	mux.Handle("DELETE /api/tasks/{id}/subtasks/{sid}", auth(http.HandlerFunc(subtaskDelete(d))))
	mux.Handle("POST /api/tasks/{id}/tags", auth(http.HandlerFunc(taskAddTag(d))))
	mux.Handle("DELETE /api/tasks/{id}/tags/{tag}", auth(http.HandlerFunc(taskRemoveTag(d))))
	mux.Handle("POST /api/tasks/{id}/notes/{nid}", auth(http.HandlerFunc(taskLinkNote(d))))
	mux.Handle("DELETE /api/tasks/{id}/notes/{nid}", auth(http.HandlerFunc(taskUnlinkNote(d))))
}

func taskList(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		f := db.TaskFilter{Status: r.URL.Query().Get("status"), Tag: r.URL.Query().Get("tag"), Q: r.URL.Query().Get("q")}
		tasks, err := d.ListTasksForUser(u.ID, f)
		if err != nil {
			Internal(w, "list")
			return
		}
		if tasks == nil {
			tasks = []models.Task{}
		}
		writeJSON(w, http.StatusOK, tasks)
	}
}

func taskCreate(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		var t models.Task
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			BadRequest(w, "INVALID_JSON", "bad json")
			return
		}
		if strings.TrimSpace(t.Title) == "" {
			BadRequest(w, "INVALID_TITLE", "title required")
			return
		}
		id, err := d.CreateTask(u.ID, &t)
		if err != nil {
			Internal(w, "create")
			return
		}
		task, err := d.GetTask(id, u.ID)
		if MapDBError(w, err, "task not found", "refetch") {
			return
		}
		writeJSON(w, http.StatusCreated, task)
	}
}

func taskGet(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		task, err := d.GetTask(id, u.ID)
		if MapDBError(w, err, "task not found", "refetch") {
			return
		}
		writeJSON(w, http.StatusOK, task)
	}
}

func taskUpdate(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			BadRequest(w, "INVALID_JSON", "bad json")
			return
		}
		if MapDBError(w, d.UpdateTask(id, u.ID, body), "task not found", "update") {
			return
		}
		task, err := d.GetTask(id, u.ID)
		if MapDBError(w, err, "task not found", "refetch") {
			return
		}
		writeJSON(w, http.StatusOK, task)
	}
}

func taskDelete(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		if MapDBError(w, d.DeleteTask(id, u.ID), "task not found", "delete task") {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func subtaskAdd(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		tid, ok := parseID(w, r)
		if !ok {
			return
		}
		var body struct {
			Title string
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" {
			BadRequest(w, "INVALID_BODY", "title required")
			return
		}
		sid, err := d.CreateSubtask(tid, u.ID, body.Title)
		if MapDBError(w, err, "task not found", "create subtask") {
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": sid, "task_id": tid, "title": body.Title, "done": false})
	}
}

func subtaskUpdate(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		sid, err := strconv.ParseInt(r.PathValue("sid"), 10, 64)
		if err != nil {
			BadRequest(w, "INVALID_ID", "bad id")
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			BadRequest(w, "INVALID_JSON", "bad json")
			return
		}
		if MapDBError(w, d.UpdateSubtask(sid, u.ID, body), "subtask not found", "update subtask") {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func subtaskDelete(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		sid, err := strconv.ParseInt(r.PathValue("sid"), 10, 64)
		if err != nil {
			BadRequest(w, "INVALID_ID", "bad id")
			return
		}
		if MapDBError(w, d.DeleteSubtask(sid, u.ID), "subtask not found", "delete subtask") {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func taskAddTag(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		tid, ok := parseID(w, r)
		if !ok {
			return
		}
		var body struct {
			Tag string
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Tag == "" {
			BadRequest(w, "INVALID_BODY", "tag required")
			return
		}
		if MapDBError(w, d.AddTaskTag(tid, u.ID, body.Tag), "task not found", "tag") {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func taskRemoveTag(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		tid, ok := parseID(w, r)
		if !ok {
			return
		}
		if MapDBError(w, d.RemoveTaskTag(tid, u.ID, r.PathValue("tag")), "task not found", "tag") {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func taskLinkNote(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		tid, ok := parseID(w, r)
		if !ok {
			return
		}
		nid, err := strconv.ParseInt(r.PathValue("nid"), 10, 64)
		if err != nil {
			BadRequest(w, "INVALID_ID", "bad note id")
			return
		}
		if MapDBError(w, d.LinkNoteToTask(nid, tid, u.ID), "task or note not found", "link") {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func taskUnlinkNote(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context(), userCtxKey)
		tid, ok := parseID(w, r)
		if !ok {
			return
		}
		nid, err := strconv.ParseInt(r.PathValue("nid"), 10, 64)
		if err != nil {
			BadRequest(w, "INVALID_ID", "bad note id")
			return
		}
		if MapDBError(w, d.UnlinkNoteFromTask(nid, tid, u.ID), "task or note not found", "unlink") {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		BadRequest(w, "INVALID_ID", "id must be an integer")
		return 0, false
	}
	return id, true
}
