package postgres

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

var adCols = []string{"id", "title", "price_cents", "photo_url", "user_id", "created_at"}

func newMockDAO(t *testing.T) (*DAO, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return &DAO{db: db}, mock
}

func TestSearchAds(t *testing.T) {
	ctx := context.Background()
	createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("returns matching ads", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		rows := sqlmock.NewRows(adCols).
			AddRow(int64(1), "bike", int64(5000), "https://example.com/bike.jpg", "user-1", createdAt).
			AddRow(int64(2), "scooter", int64(12000), "https://example.com/scooter.jpg", "user-2", createdAt)
		mock.ExpectQuery("SELECT").
			WithArgs("", int64(0), int64(0), "").
			WillReturnRows(rows)

		got, err := dao.SearchAds(ctx, adsdao.SearchAdsRequest{})

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, domain.AdID(1), got[0].ID)
		assert.Equal(t, "bike", got[0].Title)
		assert.Equal(t, int64(5000), got[0].Price)
		assert.Equal(t, domain.AdID(2), got[1].ID)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns nil when no ads match", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectQuery("SELECT").
			WithArgs("", int64(0), int64(0), "bike").
			WillReturnRows(sqlmock.NewRows(adCols))

		got, err := dao.SearchAds(ctx, adsdao.SearchAdsRequest{Title: "bike"})

		require.NoError(t, err)
		assert.Nil(t, got)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when query fails", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("db down")
		mock.ExpectQuery("SELECT").
			WithArgs("", int64(0), int64(0), "").
			WillReturnError(boom)

		_, err := dao.SearchAds(ctx, adsdao.SearchAdsRequest{})

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestGetAdByID(t *testing.T) {
	ctx := context.Background()
	createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("returns ad on success", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		rows := sqlmock.NewRows(adCols).
			AddRow(int64(42), "mountain bike", int64(15000), "https://example.com/bike.jpg", "user-1", createdAt)
		mock.ExpectQuery("SELECT").
			WithArgs(int64(42)).
			WillReturnRows(rows)

		got, err := dao.GetAdByID(ctx, 42)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(42), got.ID)
		assert.Equal(t, "mountain bike", got.Title)
		assert.Equal(t, int64(15000), got.Price)
		assert.Equal(t, domain.UserID("user-1"), got.UserID)
		assert.Equal(t, createdAt, got.CreatedAt)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns ErrNotFound when no row", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectQuery("SELECT").
			WithArgs(int64(99)).
			WillReturnRows(sqlmock.NewRows(adCols))

		_, err := dao.GetAdByID(ctx, 99)

		require.Error(t, err)
		assert.ErrorIs(t, err, adsdao.ErrNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error on query failure", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("connection reset")
		mock.ExpectQuery("SELECT").
			WithArgs(int64(1)).
			WillReturnError(boom)

		_, err := dao.GetAdByID(ctx, 1)

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestCreateAd(t *testing.T) {
	ctx := context.Background()
	req := adsdao.CreateAdRequest{
		Title:      "mountain bike",
		PriceCents: 15000,
		PhotoURL:   "https://example.com/bike.jpg",
		OwnerID:    "user-1",
	}

	t.Run("returns generated ID on success", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))

		got, err := dao.CreateAd(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(7), got)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns ErrUnknownOwner on FK violation", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		pgErr := &pgconn.PgError{Code: "23503"}
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnError(pgErr)

		_, err := dao.CreateAd(ctx, req)

		require.Error(t, err)
		assert.ErrorIs(t, err, adsdao.ErrUnknownOwner)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error on other failures", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("db down")
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnError(boom)

		_, err := dao.CreateAd(ctx, req)

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		assert.NotErrorIs(t, err, adsdao.ErrUnknownOwner)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTransaction(t *testing.T) {
	ctx := context.Background()
	req := adsdao.CreateAdRequest{
		Title:      "mountain bike",
		PriceCents: 15000,
		PhotoURL:   "https://example.com/bike.jpg",
		OwnerID:    "user-1",
	}

	t.Run("commits every call made on the transaction", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectBegin()
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(2)))
		mock.ExpectCommit()

		tx, err := dao.Begin(ctx)
		require.NoError(t, err)

		first, err := tx.CreateAd(ctx, req)
		require.NoError(t, err)
		second, err := tx.CreateAd(ctx, req)
		require.NoError(t, err)

		require.NoError(t, tx.Commit())
		assert.Equal(t, domain.AdID(1), first)
		assert.Equal(t, domain.AdID(2), second)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rolls back without committing", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectBegin()
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
		mock.ExpectRollback()

		tx, err := dao.Begin(ctx)
		require.NoError(t, err)

		_, err = tx.CreateAd(ctx, req)
		require.NoError(t, err)

		require.NoError(t, tx.Rollback())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when the transaction cannot be opened", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("db down")
		mock.ExpectBegin().WillReturnError(boom)

		tx, err := dao.Begin(ctx)

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		assert.Nil(t, tx)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rollback after commit does nothing", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		tx, err := dao.Begin(ctx)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())

		require.NoError(t, tx.Rollback())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rejects a nested transaction", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		tx, err := dao.Begin(ctx)
		require.NoError(t, err)

		_, nestedErr := tx.(*DAO).Begin(ctx)

		require.Error(t, nestedErr)
		require.NoError(t, tx.Rollback())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("refuses commit and rollback outside a transaction", func(t *testing.T) {
		dao, _ := newMockDAO(t)

		require.Error(t, dao.Commit())
		require.Error(t, dao.Rollback())
	})

	t.Run("refuses transactional operations outside a transaction", func(t *testing.T) {
		dao, mock := newMockDAO(t)

		userID := domain.UserID("user-1")

		tests := []struct {
			name string
			call func() error
		}{
			{
				name: "reserve import",
				call: func() error {
					_, err := dao.ReserveImport(ctx, adsdao.ReserveImportRequest{
						UserID: userID,
					})
					return err
				},
			},
			{
				name: "complete import",
				call: func() error {
					return dao.CompleteImport(ctx, userID, "key", nil)
				},
			},
			{
				name: "lock user publications",
				call: func() error {
					return dao.LockUserPublications(ctx, userID)
				},
			},
			{
				name: "count recent publications",
				call: func() error {
					_, err := dao.CountRecentPublications(ctx, userID)
					return err
				},
			},
			{
				name: "record publication",
				call: func() error {
					return dao.RecordPublication(ctx, userID)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				require.ErrorContains(t, tt.call(), "outside a transaction")
			})
		}

		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestDeleteAdByID(t *testing.T) {
	ctx := context.Background()

	t.Run("returns nil (no error) on success", func(t *testing.T) {
		dao, mock := newMockDAO(t)

		mock.ExpectExec("DELETE").
			WithArgs(int64(42)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := dao.DeleteAdByID(ctx, 42)

		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns ErrNotFound when no row", func(t *testing.T) {
		dao, mock := newMockDAO(t)

		mock.ExpectExec("DELETE").
			WithArgs(int64(99)).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := dao.DeleteAdByID(ctx, 99)

		require.Error(t, err)
		assert.ErrorIs(t, err, adsdao.ErrNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error on query failure", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("connection reset")

		mock.ExpectExec("DELETE").
			WithArgs(int64(1)).
			WillReturnError(boom)

		err := dao.DeleteAdByID(ctx, 1)

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error on retrievel of affected rows", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		rowsErr := errors.New("rows affected error")

		mock.ExpectExec("DELETE").
			WithArgs(int64(42)).
			WillReturnResult(sqlmock.NewErrorResult(rowsErr))

		err := dao.DeleteAdByID(ctx, 42)

		require.Error(t, err)
		assert.ErrorIs(t, err, rowsErr)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestReserveImport_ExistingImportQueryFailure(t *testing.T) {
	ctx := context.Background()
	dao, mock := newMockDAO(t)

	req := adsdao.ReserveImportRequest{
		UserID:         "user-1",
		IdempotencyKey: "import-key",
		RequestHash:    "request-hash",
	}
	boom := errors.New("database read failed")

	mock.ExpectBegin()
	tx, err := dao.Begin(ctx)
	require.NoError(t, err)

	// The key already exists: INSERT ... RETURNING returns no row.
	mock.ExpectQuery("INSERT INTO import_requests").
		WithArgs(string(req.UserID), req.IdempotencyKey, req.RequestHash).
		WillReturnRows(sqlmock.NewRows([]string{"1"}))

	// Reading the existing import fails.
	mock.ExpectQuery("SELECT request_hash").
		WithArgs(string(req.UserID), req.IdempotencyKey).
		WillReturnError(boom)

	_, err = tx.ReserveImport(ctx, req)
	require.ErrorIs(t, err, boom)

	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCompleteImport_QueryFailure(t *testing.T) {
	ctx := context.Background()

	db, mock, err := sqlmock.New(
		sqlmock.ValueConverterOption(emptyIDArrayConverter{}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	dao := &DAO{db: db}
	userID := domain.UserID("user-1")
	key := "import-key"
	boom := errors.New("update failed")

	mock.ExpectBegin()
	tx, err := dao.Begin(ctx)
	require.NoError(t, err)

	mock.ExpectExec("UPDATE import_requests").
		WithArgs(string(userID), key, "{}").
		WillReturnError(boom)

	err = tx.CompleteImport(ctx, userID, key, nil)
	require.ErrorIs(t, err, boom)

	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCompleteImport_InvalidRowsAffected(t *testing.T) {
	ctx := context.Background()
	userID := domain.UserID("user-1")
	key := "import-key"
	boom := errors.New("rows affected failed")

	tests := []struct {
		name    string
		result  driver.Result
		wantErr error
	}{
		{
			name:    "returns error when rows affected cannot be read",
			result:  sqlmock.NewErrorResult(boom),
			wantErr: boom,
		},
		{
			name:   "returns error when no row was updated",
			result: sqlmock.NewResult(0, 0),
		},
		{
			name:   "returns error when multiple rows were updated",
			result: sqlmock.NewResult(0, 2),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(
				sqlmock.ValueConverterOption(emptyIDArrayConverter{}),
			)
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })

			dao := &DAO{db: db}

			mock.ExpectBegin()
			tx, err := dao.Begin(ctx)
			require.NoError(t, err)

			mock.ExpectExec("UPDATE import_requests").
				WithArgs(string(userID), key, "{}").
				WillReturnResult(tt.result)

			err = tx.CompleteImport(ctx, userID, key, nil)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.Error(t, err)
			}

			mock.ExpectRollback()
			require.NoError(t, tx.Rollback())

			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestLockUserPublications_QueryFailure(t *testing.T) {
	ctx := context.Background()
	dao, mock := newMockDAO(t)

	userID := domain.UserID("user-1")
	boom := errors.New("advisory lock failed")

	mock.ExpectBegin()
	tx, err := dao.Begin(ctx)
	require.NoError(t, err)

	mock.ExpectQuery("SELECT pg_advisory_xact_lock").
		WithArgs(string(userID)).
		WillReturnError(boom)

	err = tx.LockUserPublications(ctx, userID)
	require.ErrorIs(t, err, boom)

	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCountRecentPublications_QueryFailure(t *testing.T) {
	ctx := context.Background()
	dao, mock := newMockDAO(t)

	userID := domain.UserID("user-1")
	boom := errors.New("count failed")

	mock.ExpectBegin()
	tx, err := dao.Begin(ctx)
	require.NoError(t, err)

	mock.ExpectQuery("SELECT COUNT").
		WithArgs(string(userID)).
		WillReturnError(boom)

	count, err := tx.CountRecentPublications(ctx, userID)

	require.ErrorIs(t, err, boom)
	assert.Zero(t, count)

	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRecordPublication_QueryFailure(t *testing.T) {
	ctx := context.Background()
	dao, mock := newMockDAO(t)

	userID := domain.UserID("00000000-0000-0000-0000-000000000001")
	dbErr := errors.New("insert failed")

	mock.ExpectBegin()

	tx, err := dao.Begin(ctx)
	require.NoError(t, err)

	mock.ExpectExec("INSERT INTO ad_publications").
		WithArgs(string(userID)).
		WillReturnError(dbErr)

	err = tx.RecordPublication(ctx, userID)

	require.ErrorIs(t, err, dbErr)

	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())

	require.NoError(t, mock.ExpectationsWereMet())
}

type emptyIDArrayConverter struct{}

func (emptyIDArrayConverter) ConvertValue(v any) (driver.Value, error) {
	if ids, ok := v.([]int64); ok && len(ids) == 0 {
		return "{}", nil
	}

	return driver.DefaultParameterConverter.ConvertValue(v)
}
