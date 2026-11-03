package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// postJSON sends a JSON POST request and registers
// response body cleanup at the end of the test.
func postJSON(t *testing.T, url string, body any, authHeader string) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := http.DefaultClient.Do(req)
	requireHTTPResponse(t, resp, err)

	return resp
}

// requireHTTPResponse checks the request error and registers
// response body cleanup using t.Cleanup.
func requireHTTPResponse(t *testing.T, resp *http.Response, err error) {
	t.Helper()

	require.NoError(t, err)
	require.NotNil(t, resp)

	t.Cleanup(func() {
		assert.NoError(t, resp.Body.Close())
	})
}
