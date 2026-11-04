package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

const adColumns = `id, title, price_cents, photo_url, user_id::text, created_at`

// DAO is a PostgreSQL-backed ad store.
type DAO struct {
	db *sql.DB
	// tx is set when the DAO is scoped to a transaction opened by WithTx.
	tx *sql.Tx
}

// queryRunner is the subset of *sql.DB and *sql.Tx the queries need.
type queryRunner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// New opens a connection pool to the given DSN and checks that the database is reachable. The caller is responsible for calling Close.
func New(ctx context.Context, dsn string) (*DAO, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to open database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("unable to reach database: %w", err)
	}

	return &DAO{db: db}, nil
}

// Close releases the connection pool.
func (d *DAO) Close() error {
	if err := d.db.Close(); err != nil {
		return fmt.Errorf("unable to close database: %w", err)
	}

	return nil
}

// runner returns the transaction the DAO is scoped to, or the connection pool.
func (d *DAO) runner() queryRunner {
	if d.tx != nil {
		return d.tx
	}

	return d.db
}

// Begin opens a transaction. Every call made on the returned Tx is part of it, and the caller must end it with Commit or Rollback.
func (d *DAO) Begin(ctx context.Context) (adsdao.Tx, error) {
	if d.tx != nil {
		return nil, errors.New("unable to open a nested transaction")
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to begin transaction: %w", err)
	}

	return &DAO{db: d.db, tx: tx}, nil
}

// Commit applies every change made through the transaction.
func (d *DAO) Commit() error {
	if d.tx == nil {
		return errors.New("unable to commit outside a transaction")
	}

	if err := d.tx.Commit(); err != nil {
		return fmt.Errorf("unable to commit transaction: %w", err)
	}

	return nil
}

// Rollback discards every change made through the transaction. Calling it on an already committed transaction does nothing, so `defer func() { _ = tx.Rollback() }()` is a safe guard.
func (d *DAO) Rollback() error {
	if d.tx == nil {
		return errors.New("unable to roll back outside a transaction")
	}

	if err := d.tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return fmt.Errorf("unable to roll back transaction: %w", err)
	}

	return nil
}

// SearchAds returns all ads matching the filters.
func (d *DAO) SearchAds(ctx context.Context, req adsdao.SearchAdsRequest) ([]domain.Ad, error) {
	const query = `
		SELECT ` + adColumns + `
		FROM ads
		WHERE ($1 = '' OR user_id::text = $1)
		  AND ($2 = 0 OR price_cents >= $2)
		  AND ($3 = 0 OR price_cents <= $3)
		  AND ($4 = '' OR title ILIKE '%' || $4 || '%')`

	rows, err := d.runner().QueryContext(ctx, query,
		req.OwnerID,
		req.MinPriceCents,
		req.MaxPriceCents,
		req.Title,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to search ads: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ads []domain.Ad
	for rows.Next() {
		var ad domain.Ad
		if err := rows.Scan(&ad.ID, &ad.Title, &ad.Price, &ad.PhotoURL, &ad.UserID, &ad.CreatedAt); err != nil {
			return nil, fmt.Errorf("unable to scan ad: %w", err)
		}

		ads = append(ads, ad)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("unable to read ads: %w", err)
	}

	return ads, nil
}

// GetAdByID returns a single ad, or dao.ErrNotFound.
func (d *DAO) GetAdByID(ctx context.Context, id domain.AdID) (domain.Ad, error) {
	const query = `SELECT ` + adColumns + ` FROM ads WHERE id = $1`

	ad, err := scanAd(d.runner().QueryRowContext(ctx, query, int64(id)))
	if err != nil {
		return domain.Ad{}, fmt.Errorf("unable to get ad %s: %w", id, err)
	}

	return ad, nil
}

// CreateAd inserts an ad and returns its generated identifier.
func (d *DAO) CreateAd(ctx context.Context, req adsdao.CreateAdRequest) (domain.AdID, error) {
	const query = `
		INSERT INTO ads (title, price_cents, photo_url, user_id)
		VALUES ($1, $2, $3, $4::uuid)
		RETURNING id`

	var id domain.AdID
	err := d.runner().QueryRowContext(ctx, query, req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return 0, fmt.Errorf("unable to create ad: %w", adsdao.ErrUnknownOwner)
		}

		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	return id, nil
}

// DeleteAdByID deletes a single ad, or returns dao.ErrNotFound.
func (d *DAO) DeleteAdByID(ctx context.Context, id domain.AdID) error {
	const query = `DELETE FROM ads WHERE id = $1`

	sqlResult, err := d.runner().ExecContext(ctx, query, int64(id))
	if err != nil {
		return fmt.Errorf("unable to delete ad %s: %w", id, err)
	}

	rowsAffected, err := sqlResult.RowsAffected()
	if err != nil {
		return fmt.Errorf("unable to get rows affected for ad %s: %w", id, err)
	}

	if rowsAffected == 0 {
		return adsdao.ErrNotFound
	}

	return nil
}

// ReserveImport reserves an idempotency key or returns an existing import.
func (d *DAO) ReserveImport(
	ctx context.Context,
	req adsdao.ReserveImportRequest,
) (*adsdao.ImportResult, error) {
	if d.tx == nil {
		return nil, errors.New("unable to reserve import outside a transaction")
	}

	const insertQuery = `
		INSERT INTO import_requests (user_id, idempotency_key, request_hash)
		VALUES ($1::uuid, $2, $3)
		ON CONFLICT (user_id, idempotency_key) DO NOTHING
		RETURNING 1`

	var inserted int
	err := d.tx.QueryRowContext(
		ctx,
		insertQuery,
		string(req.UserID),
		req.IdempotencyKey,
		req.RequestHash,
	).Scan(&inserted)

	if err == nil {
		// New key: the import can proceed.
		return nil, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("unable to reserve import: %w", err)
	}

	// The key already exists. Retrieve the recorded import.
	const selectQuery = `
		SELECT request_hash, array_to_json(ad_ids)::text
		FROM import_requests
		WHERE user_id = $1::uuid
		  AND idempotency_key = $2`

	var result adsdao.ImportResult
	var idsJSON string

	err = d.tx.QueryRowContext(
		ctx,
		selectQuery,
		string(req.UserID),
		req.IdempotencyKey,
	).Scan(&result.RequestHash, &idsJSON)
	if err != nil {
		return nil, fmt.Errorf("unable to get existing import: %w", err)
	}

	var ids []int64
	if err := json.Unmarshal([]byte(idsJSON), &ids); err != nil {
		return nil, fmt.Errorf("unable to decode existing import IDs: %w", err)
	}

	result.AdIDs = make([]domain.AdID, len(ids))
	for i, id := range ids {
		result.AdIDs[i] = domain.AdID(id)
	}

	return &result, nil
}

// CompleteImport records the IDs created by an import.
func (d *DAO) CompleteImport(
	ctx context.Context,
	userID domain.UserID,
	key string,
	ids []domain.AdID,
) error {
	if d.tx == nil {
		return errors.New("unable to complete import outside a transaction")
	}

	adIDs := make([]int64, len(ids))
	for i, id := range ids {
		adIDs[i] = int64(id)
	}

	const query = `
		UPDATE import_requests
		SET ad_ids = $3::bigint[]
		WHERE user_id = $1::uuid
		  AND idempotency_key = $2`

	result, err := d.tx.ExecContext(
		ctx,
		query,
		string(userID),
		key,
		adIDs,
	)
	if err != nil {
		return fmt.Errorf("unable to complete import: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("unable to get updated import rows: %w", err)
	}

	if rowsAffected != 1 {
		return fmt.Errorf("unable to complete import: expected one updated row, got %d", rowsAffected)
	}

	return nil
}

// LockUserPublications serializes publication attempts for a user.
func (d *DAO) LockUserPublications(ctx context.Context, userID domain.UserID) error {
	if d.tx == nil {
		return errors.New("unable to lock publications outside a transaction")
	}

	const query = `
		SELECT pg_advisory_xact_lock(
			hashtextextended('ads:publication-quota:' || $1::text, 0)
		)`

	var ignored any
	if err := d.tx.QueryRowContext(ctx, query, string(userID)).Scan(&ignored); err != nil {
		return fmt.Errorf("unable to lock user publications: %w", err)
	}

	return nil
}

// CountRecentPublications counts publications made in the last 24 hours.
func (d *DAO) CountRecentPublications(ctx context.Context, userID domain.UserID) (int, error) {
	if d.tx == nil {
		return 0, errors.New("unable to count publications outside a transaction")
	}

	const query = `
		SELECT COUNT(*)
		FROM ad_publications
		WHERE user_id = $1::uuid
		  AND created_at > statement_timestamp() - INTERVAL '24 hours'`

	var count int
	if err := d.tx.QueryRowContext(ctx, query, string(userID)).Scan(&count); err != nil {
		return 0, fmt.Errorf("unable to count recent publications: %w", err)
	}

	return count, nil
}

// RecordPublication records a successful single-ad publication.
func (d *DAO) RecordPublication(ctx context.Context, userID domain.UserID) error {
	if d.tx == nil {
		return errors.New("unable to record publication outside a transaction")
	}

	const query = `
		INSERT INTO ad_publications (user_id)
		VALUES ($1::uuid)`

	if _, err := d.tx.ExecContext(ctx, query, string(userID)); err != nil {
		return fmt.Errorf("unable to record publication: %w", err)
	}

	return nil
}

func scanAd(row *sql.Row) (domain.Ad, error) {
	var ad domain.Ad
	if err := row.Scan(&ad.ID, &ad.Title, &ad.Price, &ad.PhotoURL, &ad.UserID, &ad.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Ad{}, adsdao.ErrNotFound
		}

		return domain.Ad{}, err
	}

	return ad, nil
}
