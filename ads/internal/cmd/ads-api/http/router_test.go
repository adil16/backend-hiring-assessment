package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRouter_InvalidAuthorization(t *testing.T) {
	router := NewRouter(Usecases{})

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/ads"},
		{http.MethodPost, "/ads/import"},
		{http.MethodDelete, "/ads/1"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			tests := []struct {
				name          string
				authorization string
				wantStatus    int
			}{
				{"missing header", "", http.StatusUnauthorized},
				{"malformed UUID", "nimportequoi", http.StatusBadRequest},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					req := httptest.NewRequest(route.method, route.path, nil)
					if tt.authorization != "" {
						req.Header.Set("Authorization", tt.authorization)
					}

					rw := httptest.NewRecorder()
					router.ServeHTTP(rw, req)

					assert.Equal(t, tt.wantStatus, rw.Code)
				})
			}
		})
	}
}
