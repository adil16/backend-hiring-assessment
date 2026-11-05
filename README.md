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

### Build metadata

The Ads API logs its version, Git commit and build date at startup.

To inject the current project version and Git commit into the Docker image, run the following command from the repository root:

```bash
VERSION="$(cat VERSION)" \
GIT_COMMIT="$(git rev-parse --short=12 HEAD)" \
docker compose up --build --wait --wait-timeout 180
```

The build date is generated automatically in UTC by the Dockerfile.

If `VERSION` or `GIT_COMMIT` is not provided, the build uses the default values `dev` and `unknown`, respectively.

The metadata is included in the structured JSON startup log and can be inspected with:

```bash
docker compose logs ads-api
```

### Logging

The ads API uses structured JSON logging through Go's standard `log/slog` package.

The log level is configurable via the `LOG_LEVEL` environment variable. Supported values are `debug`, `info`, `warn`, and `error`. The default is `info`.

To enable debug logging with Docker Compose:

```bash
LOG_LEVEL=debug docker compose up -d
```

Unexpected application errors are logged at `ERROR` level with the operation name and relevant diagnostic context. Expected business errors are handled through their HTTP status codes without being logged as application errors.

## Model

There are 2 entities in this sample:
- ad (title, price, etc...)
- user (id and nickname only)

## API

For the complete API contract, see the
[OpenAPI specification](ads/internal/cmd/ads-api/specs/openapi.yaml).


| Method | Path          | Auth | Description                                                                       |
|--------|---------------|------|-----------------------------------------------------------------------------------|
| `POST` | `/ads`        | yes | Create an ad                                                                      |
| `GET` | `/ads/{id}`   | no | Get a single ad                                                                   |
| `GET` | `/ads`        | no | List ads (filterable by `title`, `user_id`, `min_price_cents`, `max_price_cents`) |
| `POST` | `/ads/import` | yes | Bulk ad import                                                                    |
| `DELETE` | `/ads/{id}`   | yes | Delete a single ad                                                              |


### Get ad by ID

`GET /ads/{id}` retrieves the ad from PostgreSQL and enriches the response with the owner's `username` from the users service.

| Scenario | HTTP status | Behavior |
|---|---|---|
| Ad found and user found | `200 OK` | Returns the ad with its username |
| Ad not found | `404 Not Found` | Returns no ad |
| Owner not found in the users service | `200 OK` | Returns the ad without the username field |
| Users service failure | `500 Internal Server Error` | Returns an error instead of an incomplete response |
| PostgreSQL failure | `500 Internal Server Error` | Returns an error |

Technical failures are propagated and logged. The API does not silently return partial responses when the users service is unavailable.

**Read performance:** This endpoint is expected to handle high read traffic. Fetching the username for every request introduces additional latency and load on the users service. A bounded cache indexed by user ID, with a configurable TTL, could be considered as a future optimization. Such a cache would require explicit trade-offs around data freshness and invalidation.

### Price representation

The API exposes `price` in euros in request and response bodies. The existing `price_cents` field remains supported but is deprecated for backward compatibility.

Prices are stored internally as integer cents (`int64`). Incoming euro amounts are converted using exact decimal arithmetic to avoid floating-point precision issues. Values with fractional cents or exceeding the supported integer range are rejected.

Search filters (`min_price_cents` and `max_price_cents`) remain expressed in cents and are not deprecated.

### Examples

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

**Historical data retention:** Publication records older than 24 hours are excluded from quota calculations but are not automatically deleted. In production, a scheduled cleanup job could periodically remove expired records to prevent unbounded table growth.

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

### End-to-end tests (Docker Compose)

End-to-end tests exercise the complete running stack: the Ads API, the users service and PostgreSQL, initialized with the database migrations and seed data.

Unlike unit and component tests, they send real HTTP requests to the running services without mocking application dependencies.

The scenarios cover authentication, ad creation and deletion, ownership checks, username enrichment, bulk import idempotency and publication quotas.

Start the stack from the repository root:

```bash
docker compose up --build --wait --wait-timeout 180
```

Then run the tests:

```bash
go test -tags=e2e -count=1 -timeout=2m -v ./ads/e2e
```

The tests use `http://localhost:8080` for the Ads API and `http://localhost:8081` for the users service. These addresses can be overridden using `ADS_E2E_BASE_URL` and `USERS_E2E_BASE_URL`.

**Run the E2E tests against a fresh database.** Some scenarios create and delete ads or consume publication quotas. To reset the local Docker Compose environment:

```bash
docker compose down -v
docker compose up --build --wait --wait-timeout 180
```

**Warning:** `docker compose down -v` removes the stack's Docker volumes and their stored data.

E2E tests use the `e2e` build tag, so they are excluded from the standard `go test ./...` suite. They run separately in the CI `e2e-tests` job, which starts the full stack and collects container logs on failure.

### Coverage

Code coverage is measured across the Ads API, including unit and PostgreSQL component tests.

With the test database running and initialized as described above, generate a coverage report from the repository root:

```bash
ADS_TEST_DATABASE_URL="postgres://ads:ads@localhost:5432/ads_test?sslmode=disable" \
go test -count=1 \
  -covermode=atomic \
  -coverpkg=./ads/internal/cmd/ads-api/... \
  -coverprofile=coverage.out \
  ./...
```

Display coverage by function and the overall percentage:

```bash
go tool cover -func=coverage.out

# Overall coverage only
go tool cover -func=coverage.out | tail -n 1
```

Visualize covered and uncovered code in a browser:

```bash
go tool cover -html=coverage.out
```

Alternatively, generate a standalone HTML report:

```bash
go tool cover -html=coverage.out -o coverage.html
```

The `component-tests` CI job also generates coverage reports with the Go race detector enabled. The overall coverage is displayed in the GitHub Actions job summary, while `coverage.out` and `coverage.html` are available as downloadable artifacts.

Coverage includes the DAO mocks and should be treated as an indicator of test execution, not a guarantee of test quality. No minimum coverage threshold is enforced.