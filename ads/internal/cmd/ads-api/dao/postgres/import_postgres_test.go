package postgres_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/postgres"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

func TestImportRequests(t *testing.T) {
	dsn := os.Getenv("ADS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ADS_TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	adDAO, err := postgres.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = adDAO.Close() })

	// Used only to clean up rows committed by the tests.
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	userID := domain.UserID("00000000-0000-0000-0000-000000000001")

	newKey := func(t *testing.T) string {
		t.Helper()

		var bytes [16]byte
		_, err := rand.Read(bytes[:])
		require.NoError(t, err)

		return "test-" + hex.EncodeToString(bytes[:])
	}

	cleanup := func(t *testing.T, key string) {
		t.Helper()

		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := db.ExecContext(
				cleanupCtx,
				`DELETE FROM import_requests WHERE user_id = $1::uuid AND idempotency_key = $2`,
				string(userID),
				key,
			)
			require.NoError(t, err)
		})
	}

	request := func(key, hash string) adsdao.ReserveImportRequest {
		return adsdao.ReserveImportRequest{
			UserID:         userID,
			IdempotencyKey: key,
			RequestHash:    hash,
		}
	}

	t.Run("reserves a new key", func(t *testing.T) {
		tx, err := adDAO.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		result, err := tx.ReserveImport(ctx, request(newKey(t), "hash-123"))

		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns existing import after commit", func(t *testing.T) {
		key := newKey(t)
		cleanup(t, key)

		ids := []domain.AdID{10, 11, 12}

		tx, err := adDAO.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		result, err := tx.ReserveImport(ctx, request(key, "hash-123"))
		require.NoError(t, err)
		require.Nil(t, result)

		err = tx.CompleteImport(ctx, userID, key, ids)
		require.NoError(t, err)

		require.NoError(t, tx.Commit())

		// Simulate a retry of the same request.
		retryTx, err := adDAO.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = retryTx.Rollback() }()

		existing, err := retryTx.ReserveImport(ctx, request(key, "hash-123"))

		require.NoError(t, err)
		require.NotNil(t, existing)
		assert.Equal(t, "hash-123", existing.RequestHash)
		assert.Equal(t, ids, existing.AdIDs)

		// The DAO must also return the original hash when the
		// key is reused with a different payload.
		otherTx, err := adDAO.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = otherTx.Rollback() }()

		existing, err = otherTx.ReserveImport(ctx, request(key, "different-hash"))

		require.NoError(t, err)
		require.NotNil(t, existing)
		assert.Equal(t, "hash-123", existing.RequestHash)
		assert.Equal(t, ids, existing.AdIDs)
	})

	t.Run("rollback releases the key", func(t *testing.T) {
		key := newKey(t)

		tx, err := adDAO.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		result, err := tx.ReserveImport(ctx, request(key, "hash-123"))
		require.NoError(t, err)
		require.Nil(t, result)

		require.NoError(t, tx.Rollback())

		// The same key must now be available again.
		retryTx, err := adDAO.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = retryTx.Rollback() }()

		result, err = retryTx.ReserveImport(ctx, request(key, "hash-123"))

		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("concurrent requests cannot reserve the same key", func(t *testing.T) {
		key := newKey(t)
		cleanup(t, key)

		ids := []domain.AdID{42, 43}

		// First transaction reserves the key without committing.
		tx1, err := adDAO.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx1.Rollback() }()

		result, err := tx1.ReserveImport(ctx, request(key, "hash-123"))
		require.NoError(t, err)
		require.Nil(t, result)

		require.NoError(t, tx1.CompleteImport(ctx, userID, key, ids))

		// Second transaction tries to reserve the same key.
		tx2, err := adDAO.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx2.Rollback() }()

		retryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		type reserveResult struct {
			existing *adsdao.ImportResult
			err      error
		}

		done := make(chan reserveResult, 1)

		go func() {
			existing, err := tx2.ReserveImport(
				retryCtx,
				request(key, "hash-123"),
			)
			done <- reserveResult{existing: existing, err: err}
		}()

		// Wait until PostgreSQL confirms that the second
		// INSERT is blocked by the first transaction.
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for {
			var blocked bool

			err := db.QueryRowContext(retryCtx, `
			SELECT EXISTS (
				SELECT 1
				FROM pg_stat_activity
				WHERE datname = current_database()
				  AND pid <> pg_backend_pid()
				  AND state = 'active'
				  AND wait_event_type = 'Lock'
				  AND query LIKE '%INSERT INTO import_requests%'
				  AND cardinality(pg_blocking_pids(pid)) > 0
			)
		`).Scan(&blocked)
			require.NoError(t, err)

			if blocked {
				break
			}

			select {
			case got := <-done:
				t.Fatalf(
					"second reservation finished before commit: %+v",
					got,
				)

			case <-retryCtx.Done():
				t.Fatal("second reservation never reached a blocked state")

			case <-ticker.C:
			}
		}

		// PostgreSQL has confirmed that the second
		// transaction is blocked. Commit the first one.
		require.NoError(t, tx1.Commit())

		// The second transaction should now retrieve
		// the existing import instead of creating a new one.
		select {
		case got := <-done:
			require.NoError(t, got.err)
			require.NotNil(t, got.existing)

			assert.Equal(t, "hash-123", got.existing.RequestHash)
			assert.Equal(t, ids, got.existing.AdIDs)

		case <-retryCtx.Done():
			t.Fatal("concurrent reservation did not finish")
		}
	})

	t.Run("complete import without reservation returns an error", func(t *testing.T) {
		tx, err := adDAO.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		err = tx.CompleteImport(
			ctx,
			userID,
			newKey(t),
			[]domain.AdID{10},
		)

		require.Error(t, err)
		assert.ErrorContains(t, err, "expected one updated row")
	})
}
