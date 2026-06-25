package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type contextKey int

const ctxKey contextKey = iota

func New() string {
	b := make([]byte, 8)
	rand.Read(b) //nolint:errcheck
	return hex.EncodeToString(b)
}

func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-Id")
		if rid == "" {
			rid = New()
		}
		ctx := context.WithValue(r.Context(), ctxKey, rid)
		w.Header().Set("X-Request-Id", rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func FromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKey).(string)
	return v
}
