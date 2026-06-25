package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/warkanum/lpwallet/internal/models"
)

func TestWithUser_RoundTrip(t *testing.T) {
	user := &models.ModelPublicUser{IDUser: 42}
	ctx := WithUser(context.Background(), user)
	got, ok := UserFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, int64(42), got.IDUser)
}

func TestUserFromContext_Missing(t *testing.T) {
	_, ok := UserFromContext(context.Background())
	assert.False(t, ok)
}

func TestBearerToken(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		{"Bearer abc123", "abc123"},
		{"bearer abc123", ""},
		{"", ""},
		{"Basic abc", ""},
		{"Bearer  spaced ", "spaced"},
	}
	for _, tc := range tests {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if tc.header != "" {
			r.Header.Set("Authorization", tc.header)
		}
		got := bearerToken(r)
		assert.Equal(t, tc.want, got, "header: %q", tc.header)
	}
}
