package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/model"
)

type contextKey string

const contextKeyUserID contextKey = "user_id"

// UserIDFromHeader extracts the user ID from the Authorization header. Returns an empty string if the header is absent.
//
// Deprecated: use UserIDFromRequest instead. A handler reading the header itself returns whatever the caller sent, and silently yields an empty user ID the day its route is no longer wrapped in RequireAuth.
func UserIDFromHeader(r *http.Request) model.UserID {
	return model.UserID(strings.TrimSpace(r.Header.Get("Authorization")))
}

// RequireAuth rejects requests that do not carry a user ID in the Authorization header and makes the authenticated user available to the handler through the request context.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		userID := UserIDFromHeader(r)
		if userID == "" {
			rw.WriteHeader(http.StatusUnauthorized)

			return
		}

		next.ServeHTTP(rw, r.WithContext(context.WithValue(r.Context(), contextKeyUserID, userID)))
	})
}

// UserIDFromRequest returns the authenticated user set by RequireAuth.
func UserIDFromRequest(r *http.Request) model.UserID {
	userID, _ := r.Context().Value(contextKeyUserID).(model.UserID)

	return userID
}
