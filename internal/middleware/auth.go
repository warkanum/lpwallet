package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/warkanum/lpwallet/internal/models"
)

type contextKey int

const ctxUser contextKey = iota

// AuthUserContextKey is exported so the audit plugin can read the user ID
// from the request context without importing the handlers package.
const AuthUserContextKey = ctxUser

func WithUser(ctx context.Context, u *models.ModelPublicUser) context.Context {
	return context.WithValue(ctx, ctxUser, u)
}

func UserFromContext(ctx context.Context) (*models.ModelPublicUser, bool) {
	u, ok := ctx.Value(ctxUser).(*models.ModelPublicUser)
	return u, ok
}

// Auth returns a middleware that validates the Bearer token.
// skipPaths are matched against r.URL.Path exactly — requests to these paths pass through unauthenticated.
func Auth(db *gorm.DB, skipPaths ...string) func(http.Handler) http.Handler {
	skip := make(map[string]bool, len(skipPaths))
	for _, p := range skipPaths {
		skip[p] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skip[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			token := bearerToken(r)
			if token == "" {
				writeUnauthorized(w)
				return
			}

			var session models.ModelPublicUserSession
			result := db.WithContext(r.Context()).
				Where("authtoken = ? AND expiresat > ?", token, time.Now()).
				First(&session)
			if result.Error != nil {
				writeUnauthorized(w)
				return
			}

			var user models.ModelPublicUser
			if err := db.WithContext(r.Context()).First(&user, session.RIDUser).Error; err != nil {
				writeUnauthorized(w)
				return
			}

			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), &user)))
		})
	}
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	after, ok := strings.CutPrefix(h, "Bearer ")
	if !ok {
		return ""
	}
	return strings.TrimSpace(after)
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"unauthorized","code":"unauthorized"}`)) //nolint:errcheck
}
