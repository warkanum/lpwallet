package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/warkanum/lpwallet/internal/models"
)

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "value"})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var got map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "value", got["key"])
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "invalid id", "bad_request")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var got errorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "invalid id", got.Error)
	assert.Equal(t, "bad_request", got.Code)
}

func TestIsAdmin(t *testing.T) {
	tests := []struct {
		name string
		role string
		want bool
	}{
		{"admin", "admin", true},
		{"member", "member", false},
		{"nil user", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "nil user" {
				assert.False(t, isAdmin(nil))
				return
			}
			u := userWithRole(tc.role)
			assert.Equal(t, tc.want, isAdmin(u))
		})
	}
}

func TestIsUniqueViolation(t *testing.T) {
	assert.False(t, isUniqueViolation(nil))
	assert.True(t, isUniqueViolation(errorString("duplicate key value violates unique constraint")))
	assert.True(t, isUniqueViolation(errorString("ERROR: 23505 unique violation")))
	assert.False(t, isUniqueViolation(errorString("some other error")))
}

type errorString string

func (e errorString) Error() string { return string(e) }

func userWithRole(role string) *models.ModelPublicUser {
	return &models.ModelPublicUser{Role: sql.NullString{String: role, Valid: true}}
}
