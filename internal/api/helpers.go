package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
)

// uuidRegex matches a valid UUID v4 format.
var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// isValidUUID checks whether a string is a valid UUID format.
func isValidUUID(s string) bool {
	return uuidRegex.MatchString(s)
}

// isDBConstraintError checks whether a database error is a constraint violation
// (duplicate key, foreign key, etc.) rather than a server-side failure.
func isDBConflictError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique") ||
		strings.Contains(msg, "23505")
}

// isDBForeignKeyError checks whether a database error is a foreign key violation.
func isDBForeignKeyError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "violates foreign key") || strings.Contains(msg, "23503")
}

// isDBInvalidInputError checks whether a database error is an invalid input syntax error
// (e.g., invalid UUID format passed to a UUID column).
func isDBInvalidInputError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "invalid input syntax") || strings.Contains(msg, "22P02")
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// maxRequestBodySize is the maximum allowed request body size (1MB).
const maxRequestBodySize = 1 << 20 // 1 MiB

// decodeJSON decodes a JSON request body with a size limit to prevent abuse.
func decodeJSON(r *http.Request, v interface{}) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxRequestBodySize)
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
