package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vladimirkreslin/todoshka/internal/db"
)

type APIError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(APIError{Error: msg, Code: code})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Unauthorized(w http.ResponseWriter, msg string) { writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", msg) }
func Forbidden(w http.ResponseWriter, msg string)    { writeError(w, http.StatusForbidden, "FORBIDDEN", msg) }
func NotFound(w http.ResponseWriter, msg string)     { writeError(w, http.StatusNotFound, "NOT_FOUND", msg) }
func BadRequest(w http.ResponseWriter, code, msg string) { writeError(w, http.StatusBadRequest, code, msg) }
func Conflict(w http.ResponseWriter, code, msg string)   { writeError(w, http.StatusConflict, code, msg) }
func Internal(w http.ResponseWriter, msg string)         { writeError(w, http.StatusInternalServerError, "INTERNAL", msg) }

func MapDBError(w http.ResponseWriter, err error, notFoundMsg, internalMsg string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, db.ErrNotFound) {
		NotFound(w, notFoundMsg)
		return true
	}
	Internal(w, internalMsg)
	return true
}
