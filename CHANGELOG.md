# Changelog

All notable changes to this project will be documented in this file.

This project follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

| Type | Description |
|---|---|
| Fixed | Made bulk ad imports atomic using a single PostgreSQL transaction to prevent partial imports. |
| Fixed | Prevented duplicate ads when retrying bulk imports with the same idempotency key. |
| Added | Added optional `Idempotency-Key` header to `POST /ads/import`, scoped to the authenticated user. |
| Added | Added persistent import tracking with request hashes and generated ad IDs. |
| Added | Added `409 Conflict` response when an idempotency key is reused with a different payload. |
| Added | Added database migration and rollback script for the `import_requests` table. |
| Added | Added PostgreSQL component tests for idempotency, rollback, and concurrent requests. |
| Changed | Introduced a dedicated transactional `ImportAds` use case. |
| Changed | Updated HTTP handlers, unit tests, OpenAPI specification, and README for bulk import idempotency. |
| Changed | Added field username to get ad by ID response. |
| Changed | Added field username to get ad by ID response. |
| Added | Added delete ad by ID feature. |
| Fixed | Fixed GetAdByID usecase. |
| Added | Introduced project versioning using a dedicated VERSION file. |
| Added | Added build-time version information injection using Go linker flags. |
| Added | Introduced a CHANGELOG.md to document project changes and releases. |
| Changed | Updated Docker build configuration to support version and Git commit injection. |

## [0.1.0] - 2026-10-30

Initial release of the project.

| Type | Description |
|---|---|
| Added | Initial commit |


---

### Change Types

- **Added**: New features.
- **Changed**: Changes to existing functionality.
- **Deprecated**: Features that will be removed in future releases.
- **Removed**: Removed features.
- **Fixed**: Bug fixes.
- **Security**: Security-related improvements or fixes.
