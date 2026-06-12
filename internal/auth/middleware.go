package auth

import (
	"context"
	"net/http"
	"strings"
)

type CtxUser struct {
	ID       int64
	Username string
}

func RequireUser(secret string, key any) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				http.Error(w, `{"error":"missing token","code":"UNAUTHORIZED"}`, http.StatusUnauthorized)
				return
			}
			uid, uname, err := ParseToken(strings.TrimPrefix(h, "Bearer "), secret)
			if err != nil {
				http.Error(w, `{"error":"invalid token","code":"UNAUTHORIZED"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), key, CtxUser{ID: uid, Username: uname})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserFromContext(ctx context.Context, key any) CtxUser {
	v, _ := ctx.Value(key).(CtxUser)
	return v
}
