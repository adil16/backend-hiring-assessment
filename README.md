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
  -d '{"title":"Mountain bike","price_cents":12000,"photo_url":"https://example.com/photos/bike.jpg"}'

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

# Delete a single ad
curl -i -X DELETE http://localhost:8080/ads/1  \
  -H 'Authorization: 00000000-0000-0000-0000-000000000001'
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

## Tests

To launch all the tests:

```shell
go test ./...
```

### Component tests (PostgreSQL)

The PostgreSQL DAO has component tests that run against a real database to verify transaction handling, idempotency key reservation, and concurrent requests.

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

The test database requires the `ads` and `import_requests` tables. The seed data migration (`002_seed_data.sql`) is not needed.

The following commands are compatible with Git Bash on Windows:

```shell
MSYS_NO_PATHCONV=1 docker compose exec -T ads-database \
  psql -v ON_ERROR_STOP=1 -U ads -d ads_test \
  -f /docker-entrypoint-initdb.d/001_create_tables.sql

MSYS_NO_PATHCONV=1 docker compose exec -T ads-database \
  psql -v ON_ERROR_STOP=1 -U ads -d ads_test \
  -f /docker-entrypoint-initdb.d/003_create_import_requests.sql
```

`MSYS_NO_PATHCONV=1` prevents Git Bash from converting container paths into Windows paths.

#### 4. Run the component tests

From the repository root:

```shell
ADS_TEST_DATABASE_URL="postgres://ads:ads@localhost:5432/ads_test?sslmode=disable" \
go test ./ads/internal/cmd/ads-api/dao/postgres -run TestImportRequests -v
```

To run all tests, including the PostgreSQL component tests:

```shell
ADS_TEST_DATABASE_URL="postgres://ads:ads@localhost:5432/ads_test?sslmode=disable" \
go test ./...
```

If `ADS_TEST_DATABASE_URL` is not set, the PostgreSQL component tests are skipped, so the regular test suite can still run without a database.

**Note:** The database creation and migration steps only need to be performed once, unless the test database is recreated.