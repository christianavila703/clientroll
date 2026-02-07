package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
)

var (
	ErrNoToken      = errors.New("no authorization token provided")
	ErrInvalidFormat = errors.New("invalid authorization header format")
)

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
