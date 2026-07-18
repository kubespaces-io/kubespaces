package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// respondJSON writes v as a JSON response with the given status.
func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "error", err)
	}
}

// respondError writes the contract error envelope {"error": "..."}.
func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

// respondInternal logs the underlying error and hides it from the client.
func respondInternal(w http.ResponseWriter, action string, err error) {
	slog.Error(action, "error", err)
	respondError(w, http.StatusInternalServerError, "internal server error")
}
