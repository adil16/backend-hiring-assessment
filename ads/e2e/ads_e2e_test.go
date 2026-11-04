//go:build e2e

package e2e_test

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	aliceID = "00000000-0000-0000-0000-000000000001"
	bobID   = "00000000-0000-0000-0000-000000000002"
	carolID = "00000000-0000-0000-0000-000000000003"
	daveID  = "00000000-0000-0000-0000-000000000004"
)

var e2eHTTPClient = &http.Client{Timeout: 5 * time.Second}

type adResponse struct {
	ID         int64       `json:"id"`
	Title      string      `json:"title"`
	Price      json.Number `json:"price"`
	PriceCents int64       `json:"price_cents"`
	UserID     string      `json:"user_id"`
	Username   string      `json:"username"`
}

type listResponse struct {
	Ads []adResponse `json:"ads"`
}

type createResponse struct {
	ID int64 `json:"id"`
}

type importResponse struct {
	IDs []int64 `json:"ids"`
}

func baseURL(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return strings.TrimRight(value, "/")
	}
	return fallback
}

// request talks to the Compose stack through its public HTTP endpoints.
func request(t *testing.T, method, path, body, userID, idempotencyKey string) (int, []byte) {
	t.Helper()

	req, err := http.NewRequest(method,
		baseURL("ADS_E2E_BASE_URL", "http://localhost:8080")+path,
		strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if userID != "" {
		req.Header.Set("Authorization", userID)
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := e2eHTTPClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, payload
}

func expectStatus(t *testing.T, got, want int, body []byte) {
	t.Helper()
	if got != want {
		t.Fatalf("status = %d, want %d; body: %s", got, want, body)
	}
}

func decodeJSON[T any](t *testing.T, payload []byte) T {
	t.Helper()
	var result T
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("decode JSON %s: %v", payload, err)
	}
	return result
}

func newKey(t *testing.T) string {
	t.Helper()
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	return "e2e-" + hex.EncodeToString(b[:])
}

// TestE2EHealthAndSeed checks service readiness, seeded ads and real username enrichment.
func TestE2EHealthAndSeed(t *testing.T) {
	status, body := request(t, http.MethodGet, "/health", "", "", "")
	expectStatus(t, status, http.StatusOK, body)

	usersURL := baseURL("USERS_E2E_BASE_URL", "http://localhost:8081")
	resp, err := e2eHTTPClient.Get(usersURL + "/health")
	if err != nil {
		t.Fatalf("users API health check: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("users API health check returned %d, want 200", resp.StatusCode)
	}

	status, body = request(t, http.MethodGet,
		"/ads?title="+url.QueryEscape("Leather sofa #1"), "", "", "")
	expectStatus(t, status, http.StatusOK, body)
	ads := decodeJSON[listResponse](t, body).Ads
	if len(ads) != 1 {
		t.Fatalf("seed search returned %d ads, want 1: %s", len(ads), body)
	}
	if ads[0].UserID != bobID || ads[0].PriceCents != 25000 {
		t.Fatalf("unexpected seed ad: %+v", ads[0])
	}

	// Username enrichment must use the real users service.
	status, body = request(t, http.MethodGet, "/ads/"+strconv.FormatInt(ads[0].ID, 10), "", "", "")
	expectStatus(t, status, http.StatusOK, body)
	ad := decodeJSON[adResponse](t, body)
	if ad.Username != "bob" || ad.Price.String() != "250.00" {
		t.Fatalf("unexpected enriched seed ad: %+v", ad)
	}
}

// TestE2ECreateReadDelete checks authentication, persistence and owner-only deletion.
func TestE2ECreateReadDelete(t *testing.T) {
	const createBody = `{"title":"E2E mountain bike","price":149.99,"photo_url":"https://example.com/e2e-bike.jpg"}`

	status, body := request(t, http.MethodPost, "/ads", createBody, "", "")
	expectStatus(t, status, http.StatusUnauthorized, body)

	status, body = request(t, http.MethodPost, "/ads", createBody, aliceID, "")
	expectStatus(t, status, http.StatusCreated, body)
	created := decodeJSON[createResponse](t, body)
	if created.ID <= 0 {
		t.Fatalf("invalid created ad ID: %d", created.ID)
	}
	path := "/ads/" + strconv.FormatInt(created.ID, 10)

	status, body = request(t, http.MethodGet, path, "", "", "")
	expectStatus(t, status, http.StatusOK, body)
	ad := decodeJSON[adResponse](t, body)
	if ad.ID != created.ID || ad.Title != "E2E mountain bike" ||
		ad.UserID != aliceID || ad.Username != "alice" ||
		ad.PriceCents != 14999 || ad.Price.String() != "149.99" {
		t.Fatalf("unexpected created ad: %+v", ad)
	}

	status, body = request(t, http.MethodDelete, path, "", bobID, "")
	expectStatus(t, status, http.StatusForbidden, body)

	// A forbidden deletion must leave the ad intact.
	status, body = request(t, http.MethodGet, path, "", "", "")
	expectStatus(t, status, http.StatusOK, body)

	status, body = request(t, http.MethodDelete, path, "", aliceID, "")
	expectStatus(t, status, http.StatusOK, body)

	// A successful deletion must actually remove the ad.
	status, body = request(t, http.MethodGet, path, "", "", "")
	expectStatus(t, status, http.StatusNotFound, body)
}

// TestE2EImportIdempotency checks retries, equivalent prices and key conflicts.
func TestE2EImportIdempotency(t *testing.T) {
	key := newKey(t)
	const importBody = `{"ads":[{"title":"E2E helmet","price":35.50,"photo_url":"https://example.com/e2e-helmet.jpg"},{"title":"E2E lock","price":12.34,"photo_url":"https://example.com/e2e-lock.jpg"}]}`
	const equivalentBody = `{"ads":[{"title":"E2E helmet","price_cents":3550,"photo_url":"https://example.com/e2e-helmet.jpg"},{"title":"E2E lock","price_cents":1234,"photo_url":"https://example.com/e2e-lock.jpg"}]}`
	const conflictingBody = `{"ads":[{"title":"Different title","price":35.50,"photo_url":"https://example.com/e2e-helmet.jpg"}]}`

	status, body := request(t, http.MethodPost, "/ads/import", importBody, carolID, key)
	expectStatus(t, status, http.StatusCreated, body)
	first := decodeJSON[importResponse](t, body)
	if len(first.IDs) != 2 || first.IDs[0] <= 0 || first.IDs[1] <= 0 || first.IDs[0] == first.IDs[1] {
		t.Fatalf("invalid import IDs: %+v", first.IDs)
	}

	// Equivalent prices in euros and cents must produce the same IDs.
	status, body = request(t, http.MethodPost, "/ads/import", equivalentBody, carolID, key)
	expectStatus(t, status, http.StatusCreated, body)
	second := decodeJSON[importResponse](t, body)
	if !reflect.DeepEqual(first.IDs, second.IDs) {
		t.Fatalf("retry IDs = %v, want %v", second.IDs, first.IDs)
	}

	// Reusing a key for different content must be rejected.
	status, body = request(t, http.MethodPost, "/ads/import", conflictingBody, carolID, key)
	expectStatus(t, status, http.StatusConflict, body)

	for _, id := range first.IDs {
		status, body = request(t, http.MethodGet, "/ads/"+strconv.FormatInt(id, 10), "", "", "")
		expectStatus(t, status, http.StatusOK, body)
		ad := decodeJSON[adResponse](t, body)
		if ad.UserID != carolID || ad.Username != "carol" {
			t.Fatalf("unexpected imported ad: %+v", ad)
		}
	}
}

// TestE2EPublicationQuota checks the five-ad limit and the bulk-import exemption.
func TestE2EPublicationQuota(t *testing.T) {
	const body = `{"title":"E2E quota ad","price":5.50,"photo_url":"https://example.com/e2e-quota.jpg"}`

	var firstID int64
	for i := 0; i < 5; i++ {
		status, payload := request(t, http.MethodPost, "/ads", body, daveID, "")
		expectStatus(t, status, http.StatusCreated, payload)
		id := decodeJSON[createResponse](t, payload).ID
		if id <= 0 {
			t.Fatalf("created invalid ad ID: %d", id)
		}
		if i == 0 {
			firstID = id
		}
	}

	status, payload := request(t, http.MethodPost, "/ads", body, daveID, "")
	expectStatus(t, status, http.StatusTooManyRequests, payload)

	// Deleting an ad must not refund the publication quota.
	status, payload = request(t, http.MethodDelete, "/ads/"+strconv.FormatInt(firstID, 10), "", daveID, "")
	expectStatus(t, status, http.StatusOK, payload)
	status, payload = request(t, http.MethodPost, "/ads", body, daveID, "")
	expectStatus(t, status, http.StatusTooManyRequests, payload)

	// Bulk imports remain possible after the single-ad quota is exhausted.
	key := newKey(t)
	importBody := fmt.Sprintf(`{"ads":[{"title":"E2E quota-exempt import","price":1.25,"photo_url":"https://example.com/e2e-import-%s.jpg"}]}`, key)
	status, payload = request(t, http.MethodPost, "/ads/import", importBody, daveID, key)
	expectStatus(t, status, http.StatusCreated, payload)
	imported := decodeJSON[importResponse](t, payload)
	if len(imported.IDs) != 1 || imported.IDs[0] <= 0 {
		t.Fatalf("invalid quota-exempt import response: %s", payload)
	}
}
