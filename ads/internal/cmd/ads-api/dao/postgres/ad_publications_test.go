package postgres_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/postgres"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

func TestAdPublications(t *testing.T) {
	dsn := os.Getenv("ADS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ADS_TEST_DATABASE_URL not set")
	}

	ctx := context.Background()

	adDAO, err := postgres.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = adDAO.Close() })

	// Used to prepare and clean up test data.
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// Each test uses a unique user to avoid interfering
	// with other tests or existing publications.
	newUserID := func(t *testing.T) domain.UserID {
		t.Helper()

		var b [16]byte
		_, err := rand.Read(b[:])
		require.NoError(t, err)

		userID := domain.UserID(fmt.Sprintf(
			"%x-%x-%x-%x-%x",
			b[0:4],
			b[4:6],
			b[6:8],
			b[8:10],
			b[10:16],
		))

		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(
				context.Background(),
				5*time.Second,
			)
			defer cancel()

			_, err := db.ExecContext(
				cleanupCtx,
				`DELETE FROM ad_publications WHERE user_id = $1::uuid`,
				string(userID),
			)
			require.NoError(t, err)
		})

		return userID
	}

	t.Run("counts only publications from last 24 hours", func(t *testing.T) {
		userID := newUserID(t)
		otherUserID := newUserID(t)

		_, err := db.ExecContext(ctx, `
			INSERT INTO ad_publications (user_id, created_at)
			VALUES
				($1::uuid, clock_timestamp() - INTERVAL '25 hours'),
				($1::uuid, clock_timestamp() - INTERVAL '23 hours'),
				($1::uuid, clock_timestamp() - INTERVAL '1 hour'),
				($2::uuid, clock_timestamp())
		`, string(userID), string(otherUserID))
		require.NoError(t, err)

		tx, err := adDAO.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		count, err := tx.CountRecentPublications(ctx, userID)

		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("records publication after commit", func(t *testing.T) {
		userID := newUserID(t)

		tx, err := adDAO.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		require.NoError(t, tx.RecordPublication(ctx, userID))
		require.NoError(t, tx.Commit())

		// Verify that the publication is persisted.
		checkTx, err := adDAO.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = checkTx.Rollback() }()

		count, err := checkTx.CountRecentPublications(ctx, userID)

		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("rollback discards recorded publication", func(t *testing.T) {
		userID := newUserID(t)

		tx, err := adDAO.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		require.NoError(t, tx.RecordPublication(ctx, userID))
		require.NoError(t, tx.Rollback())

		// The rolled-back publication must not be counted.
		checkTx, err := adDAO.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = checkTx.Rollback() }()

		count, err := checkTx.CountRecentPublications(ctx, userID)

		require.NoError(t, err)
		assert.Zero(t, count)
	})

	t.Run("concurrent requests cannot exceed quota", func(t *testing.T) {
		userID := newUserID(t)

		// The user already has 4 publications.
		_, err := db.ExecContext(ctx, `
			INSERT INTO ad_publications (user_id)
			SELECT $1::uuid
			FROM generate_series(1, 4)
		`, string(userID))
		require.NoError(t, err)

		// Transaction A acquires the lock first.
		tx1, err := adDAO.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx1.Rollback() }()

		require.NoError(t, tx1.LockUserPublications(ctx, userID))

		count, err := tx1.CountRecentPublications(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, 4, count)

		// Transaction B attempts a publication concurrently.
		tx2, err := adDAO.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx2.Rollback() }()

		retryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		type publicationResult struct {
			allowed bool
			count   int
			err     error
		}

		done := make(chan publicationResult, 1)

		go func() {
			if err := tx2.LockUserPublications(retryCtx, userID); err != nil {
				done <- publicationResult{err: err}
				return
			}

			count, err := tx2.CountRecentPublications(retryCtx, userID)
			if err != nil {
				done <- publicationResult{err: err}
				return
			}

			if count >= 5 {
				done <- publicationResult{
					allowed: false,
					count:   count,
				}
				return
			}

			if err := tx2.RecordPublication(retryCtx, userID); err != nil {
				done <- publicationResult{err: err}
				return
			}

			if err := tx2.Commit(); err != nil {
				done <- publicationResult{err: err}
				return
			}

			done <- publicationResult{
				allowed: true,
				count:   count,
			}
		}()

		// Wait until PostgreSQL confirms that transaction B
		// is blocked while trying to acquire the advisory lock.
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
					  AND query LIKE '%pg_advisory_xact_lock%'
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
					"second publication finished before first commit: %+v",
					got,
				)

			case <-retryCtx.Done():
				t.Fatal("second publication never reached a blocked state")

			case <-ticker.C:
			}
		}

		// Transaction B is now blocked.
		// Transaction A records the 5th publication and commits.
		require.NoError(t, tx1.RecordPublication(ctx, userID))
		require.NoError(t, tx1.Commit())

		// Transaction B should now see 5 publications
		// and reject the attempt.
		select {
		case got := <-done:
			require.NoError(t, got.err)
			assert.False(t, got.allowed)
			assert.Equal(t, 5, got.count)

		case <-retryCtx.Done():
			t.Fatal("concurrent publication did not finish")
		}

		// Verify that the total number of publications is 5,
		// not 6.
		checkTx, err := adDAO.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = checkTx.Rollback() }()

		finalCount, err := checkTx.CountRecentPublications(ctx, userID)

		require.NoError(t, err)
		assert.Equal(t, 5, finalCount)
	})
}
