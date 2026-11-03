# Way of working

This repository is read-only, click on "Use this template" => "Create a new repository" and work on your copy just as you would on a live one. \
Once you are satisfied with your work, please send us your repository url.

# Hiring instructions

Please refer to [these instructions](INSTRUCTIONS.md).

# Ads API

A small Go HTTP API around a single domain: *ads* (classified ads).

The API needs an up-and-running postgres database, which can be launched via the provided Docker Compose file.

## Requirements

- Go 1.26 or later
- Docker with the Compose plugin

## Launching the stack

```bash
docker compose up -d
```

This starts PostgreSQL with the schema and seed data already applied, as well as the `ads` api.

## Model

There are 2 entities in this sample:
- ad (title, price, etc...)
- user (id and nickname only)

## API

| Method | Path          | Auth | Description                                                                       |
|--------|---------------|------|-----------------------------------------------------------------------------------|
| `POST` | `/ads`        | yes | Create an ad                                                                      |
| `GET` | `/ads/{id}`   | no | Get a single ad                                                                   |
| `GET` | `/ads`        | no | List ads (filterable by `title`, `user_id`, `min_price_cents`, `max_price_cents`) |
| `POST` | `/ads/import` | yes | Bulk ad import                                                                    |
| `DELETE` | `/ads/{id}`   | yes | Delete a single ad                                                              |

```bash
# Health check
curl -s -v http://localhost:8080/health

# List ads (with optional filters)
curl -s 'http://localhost:8080/ads?title=bike&min_price_cents=1000'

# Get a single ad
curl -s http://localhost:8080/ads/1

# Create an ad (requires Authorization header)
curl -s -X POST http://localhost:8080/ads \
  -H 'Authorization: 00000000-0000-0000-0000-000000000001' \
  -H 'Content-Type: application/json' \
  -d '{"title":"Mountain bike","price":120.00,"photo_url":"https://example.com/photos/bike.jpg"}'

# Bulk import ads (requires Authorization header)
curl -i -X POST http://localhost:8080/ads/import \
  -H 'Authorization: 00000000-0000-0000-0000-000000000001' \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: import-example-001' \
  -d '{
    "ads": [
      {
        "title": "Mountain bike",
        "price": 120.00,
        "photo_url": "https://example.com/photos/bike.jpg"
      },
      {
        "title": "Bike helmet",
        "price": 35.50,
        "photo_url": "https://example.com/photos/helmet.jpg"
      }
    ]
  }'

# Delete an ad owned by the authenticated user
curl -i -X DELETE http://localhost:8080/ads/1 \
  -H 'Authorization: 00000000-0000-0000-0000-000000000002'
```

For more details please refer to [openapi.yaml](ads/internal/cmd/ads-api/specs/openapi.yaml).

## Authentication

To pass authentication information to a call, simply add the `Authorization` header with the user ID in plain text.

It will be extracted by the service.

```
Authorization: 00000000-0000-0000-0000-000000000001
```

## Bulk import idempotency

The `POST /ads/import` endpoint supports the optional `Idempotency-Key` header to prevent duplicate ads when a client retries an import request.

Clients should generate a unique key for each new import operation and reuse the same key when retrying that operation.

**Recommendation:** Use a UUID as the idempotency key in production. Generate a new key for each import and retain it until the operation has completed successfully.

The key is scoped to the authenticated user, so different users can independently use the same key.

| Scenario | HTTP status | Behavior |
|---|---|---|
| First request with a new key | `201 Created` | Creates all ads and stores the generated IDs |
| Retry with the same key and payload | `201 Created` | Returns the original IDs without creating duplicates |
| Same key with a different payload | `409 Conflict` | Rejects the request without creating ads |
| Request fails before the transaction commits | Error response | The transaction is rolled back, allowing a retry |
| Request without an idempotency key | `201 Created` | Imports ads atomically, but retries may create duplicates |

Imports are **atomic**: all ads are created within a single database transaction. If any insertion fails, the transaction is rolled back and no ads from that import are persisted.

The idempotency key and the import result are stored in the same transaction as the ads, ensuring retries can recover the original result even if the first HTTP response was lost.

**Note:** The idempotency mechanism compares normalized ad data. For example, `price: 120.00` and `price_cents: 12000` are treated as equivalent for retry detection.

## Publication quota

Users can publish at most **5 ads within a rolling 24-hour window** through `POST /ads`.

- The first 5 publications are accepted (`201 Created`).
- Additional publications within the same window are rejected with `429 Too Many Requests`.
- Deleting an ad does not restore the publication quota.
- Bulk imports through `POST /ads/import` are exempt from the quota and do not count towards it.

### Implementation

Publication events are stored in a dedicated PostgreSQL `ad_publications` table, independently of the ads themselves.

Each single-ad creation runs in a database transaction:

1. Acquire a transaction-scoped advisory lock for the authenticated user.
2. Count publications made within the last 24 hours.
3. Reject the request if the quota is reached.
4. Otherwise, create the ad and record its publication.
5. Commit both operations atomically.

The per-user advisory lock prevents concurrent requests from exceeding the quota. If the transaction fails, neither the ad nor its publication event is persisted.

This approach also ensures that deleting an ad does not reset the quota and that bulk imports remain excluded.

## Database migrations

PostgreSQL migration scripts are located in `ads/internal/cmd/ads-api/dao/postgres/migrations/`, with corresponding rollback scripts in `ads/internal/cmd/ads-api/dao/postgres/rollbacks/`.

For a new database initialized by Docker Compose, migration scripts are executed automatically during PostgreSQL initialization.

For an existing database, migrations must be applied manually, as Docker initialization scripts are not re-executed when an existing database volume is reused.

### Applying migrations to an existing database

The following migrations are required:

- `003_create_import_requests.sql`: creates the `import_requests` table for bulk import idempotency.
- `004_create_ad_publications.sql`: creates the publication history table and its index for enforcing the rolling 24-hour quota.

Start PostgreSQL:

```shell
docker compose up -d ads-database
```

Apply the missing migrations to the application database (`ads`), in order, before deploying the updated API:

```shell
MSYS_NO_PATHCONV=1 docker compose exec -T ads-database \
  psql -v ON_ERROR_STOP=1 -U ads -d ads \
  -f /docker-entrypoint-initdb.d/003_create_import_requests.sql

MSYS_NO_PATHCONV=1 docker compose exec -T ads-database \
  psql -v ON_ERROR_STOP=1 -U ads -d ads \
  -f /docker-entrypoint-initdb.d/004_create_ad_publications.sql
```

These commands are compatible with Git Bash on Windows.

Migrations should only be applied once. Skip migrations that have already been applied.

The existing `001_create_tables.sql` and `002_seed_data.sql` scripts do not need to be re-executed.

### Verifying the migrations

Check that both tables exist:

```shell
docker compose exec -T ads-database \
  psql -U ads -d ads \
  -c "SELECT to_regclass('public.import_requests') AS import_requests,
             to_regclass('public.ad_publications') AS ad_publications;"
```

Both columns should contain their respective table names rather than `NULL`.

### Rolling back migrations

Rollback scripts are provided to reverse database schema changes.

**Warning:** These rollbacks are destructive. They drop the corresponding tables and permanently remove their stored data:

- Dropping `ad_publications` removes the publication history and resets quota tracking.
- Dropping `import_requests` removes stored idempotency keys and import results. Previously imported ads remain in the `ads` table, so retrying an old import may create duplicates.

Before rolling back, stop the updated API or deploy a compatible previous version, and back up the affected data if it must be preserved.

Apply rollbacks in reverse migration order:

```shell
docker compose exec -T ads-database \
  psql -v ON_ERROR_STOP=1 -U ads -d ads \
  < ads/internal/cmd/ads-api/dao/postgres/rollbacks/004_drop_ad_publications.sql

docker compose exec -T ads-database \
  psql -v ON_ERROR_STOP=1 -U ads -d ads \
  < ads/internal/cmd/ads-api/dao/postgres/rollbacks/003_drop_import_requests.sql
```

These commands read the rollback scripts from the repository and send them to PostgreSQL through standard input.

Rollback scripts should only be executed when explicitly reverting the corresponding features. Rolling back an application version does not necessarily require dropping its database tables.

### Publication quota activation

The `ad_publications` table starts empty when migration `004` is applied.

Publication history is recorded only for successful single-ad creations through `POST /ads` after the quota-enabled API is deployed.

Consequently:

- Ads created before quota activation are not counted, even if they were created within the previous 24 hours.
- Existing ads are not automatically added to the publication history.
- Bulk imports through `POST /ads/import` remain exempt from the quota.
- Deleting an ad does not remove its publication history.

Users may therefore temporarily exceed 5 publications across the deployment boundary.

No automatic backfill is performed because existing ads cannot necessarily be distinguished from bulk-imported ads, which are exempt from the quota.

This initial transition should be considered when deploying the feature to an existing production database.

## Tests

To launch all the tests:

```shell
go test ./...
```

### Component tests (PostgreSQL)

The PostgreSQL DAO has component tests that run against a real database to verify transaction handling, idempotency key reservation, publication history, and concurrent requests.

These tests require Docker and a dedicated `ads_test` database.

#### 1. Start PostgreSQL

```shell
docker compose up -d ads-database
```

#### 2. Create the test database

Run this command once:

```shell
docker compose exec -T ads-database \
  psql -U ads -d postgres -c "CREATE DATABASE ads_test;"
```

#### 3. Apply the required migrations

The test database requires the `ads`, `import_requests`, and `ad_publications` tables.

The seed data migration (`002_seed_data.sql`) is not needed.

The following commands are compatible with Git Bash on Windows:

```shell
MSYS_NO_PATHCONV=1 docker compose exec -T ads-database \
  psql -v ON_ERROR_STOP=1 -U ads -d ads_test \
  -f /docker-entrypoint-initdb.d/001_create_tables.sql

MSYS_NO_PATHCONV=1 docker compose exec -T ads-database \
  psql -v ON_ERROR_STOP=1 -U ads -d ads_test \
  -f /docker-entrypoint-initdb.d/003_create_import_requests.sql

MSYS_NO_PATHCONV=1 docker compose exec -T ads-database \
  psql -v ON_ERROR_STOP=1 -U ads -d ads_test \
  -f /docker-entrypoint-initdb.d/004_create_ad_publications.sql
```

`MSYS_NO_PATHCONV=1` prevents Git Bash from converting container paths into Windows paths.

These migrations only need to be applied once to an existing test database.

#### 4. Run the component tests

From the repository root:

```shell
ADS_TEST_DATABASE_URL="postgres://ads:ads@localhost:5432/ads_test?sslmode=disable" \
go test ./ads/internal/cmd/ads-api/dao/postgres \
  -run 'TestImportRequests|TestAdPublications' -v
```

The component tests cover:

- Atomic import transactions and idempotency key reservation.
- Concurrent requests using the same idempotency key.
- Publication counting within a rolling 24-hour window.
- Publication persistence after commit and cancellation after rollback.
- Concurrent publication attempts for the same user, ensuring that the quota cannot be exceeded.

To verify that the concurrent publication test remains stable, run it multiple times:

```shell
ADS_TEST_DATABASE_URL="postgres://ads:ads@localhost:5432/ads_test?sslmode=disable" \
go test ./ads/internal/cmd/ads-api/dao/postgres \
  -run 'TestAdPublications/concurrent_requests' -count=20
```

To run all tests, including the PostgreSQL component tests:

```shell
ADS_TEST_DATABASE_URL="postgres://ads:ads@localhost:5432/ads_test?sslmode=disable" \
go test ./...
```

If `ADS_TEST_DATABASE_URL` is not set, the PostgreSQL component tests are skipped, so the regular test suite can still run without a database.

**Note:** Database creation and migration steps only need to be performed once, unless the test database is recreated. If the database already exists, apply only the missing migrations.
