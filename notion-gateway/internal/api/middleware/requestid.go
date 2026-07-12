package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type key struct{}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			var b [16]byte
			if _, err := rand.Read(b[:]); err != nil {
				http.Error(w, "request id generation failed", http.StatusInternalServerError)
				return
			}
			id = hex.EncodeToString(b[:])
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), key{}, id)))
	})
}
func ID(ctx context.Context) string { v, _ := ctx.Value(key{}).(string); return v }
