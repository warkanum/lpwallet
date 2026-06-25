package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"gorm.io/gorm"
)

type Handler struct {
	db     *gorm.DB
	logger *slog.Logger
}

func New(db *gorm.DB, logger *slog.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg, code string) {
	writeJSON(w, status, errorResponse{Error: msg, Code: code})
}

func parseJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
