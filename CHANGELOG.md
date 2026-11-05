# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- `DELETE /ads/{adID}` with ownership checks.
- A limit of five ad publications per rolling 24-hour window through `POST /ads`, with a `429 Too Many Requests` response when exceeded.
- Optional, user-scoped `Idempotency-Key` support for `POST /ads/import`, with a `409 Conflict` response when a key is reused with a different payload.
- Added database tables to enforce publication quotas and prevent duplicate bulk imports including migrations and rollback scripts.
- HTTP input validation for ad titles, photo URIs, user UUIDs, and price filters, with whitespace normalization for titles and photo URIs.
- Structured JSON logging with configurable levels through `LOG_LEVEL`.
- Project versioning through a `VERSION` file and build-time metadata injection using Go linker flags.
- Docker Compose end-to-end tests for authorization, ad lifecycle, ownership, import idempotency, and publication quotas, with a dedicated CI job.
- PostgreSQL component tests for import idempotency, transaction rollback, and concurrent requests.

### Changed

- Included the username in the `GET /ads/{adID}` response.
- Moved bulk imports to a dedicated transactional `ImportAds` use case.
- Updated Docker builds to support version and Git commit injection.
- Updated the OpenAPI specification, README, and related tests to reflect the new API behavior.

### Fixed

- Corrected ad retrieval by ID.
- Made bulk imports atomic to prevent partial imports.
- Prevented duplicate ads when retrying an import with the same idempotency key.

## [0.1.0] - 2026-10-30

### Added

- Initial release of the project.