# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [v1.1.0] - 2026-06-04

### Breaking Changes

- **auth**: JWT claim JSON tags changed from `"ID"`/`"Email"` to `"id"`/`"email"` to follow JWT conventions. Tokens issued before this change will still pass signature validation but their `ID` and `Email` fields will deserialize as empty strings. Consuming services that match on the old uppercase keys will need to be updated.
- **context**: Removed `ErrLoggerFieldsNotFound` — it was never returned by any function and was dead code. External code referencing this sentinel will fail to compile.

### Bug Fixes

- **context/logger**: `AddFieldsToContext` now stores a `*MutableFields` (instead of `[]map[string]interface{}`), matching what the logger reads. Previously, fields added via `AddFieldsToContext` were silently dropped because the logger's type assertion expected `*MutableFields`.
- **context**: `GetFieldsFromContext` now reads `*MutableFields` to match `AddFieldsToContext`, making the round-trip consistent.
- **context**: `MutableFields.GetFields` now returns a copy of the internal slice instead of the backing array, preventing data races when a caller iterates the result while another goroutine calls `AddField`.
- **auth**: Removed unnecessary `time.Now().Local()` in token generation — `NumericDate` serializes as a Unix timestamp (UTC), so the `.Local()` call was a no-op.
- **logger**: `convertToZapFields` now falls back to `zap.Any` for types other than `string` and `int`, instead of silently dropping them.

### Added

- **context**: `AddLoggerToContext` — correctly spelled replacement for `AddLoggerToContex`.
- **CI**: GitHub Actions workflow (`.github/workflows/ci.yml`) running `go test -race`, `go vet`, `gofmt -l`, and `golangci-lint` across Go 1.21–1.23.
- **tests**: End-to-end test proving `AddFieldsToContext` fields reach the logger.
- **tests**: Context package coverage for `AddFieldsToContext`, `GetFieldsFromContext`, `MutableFields`, and `contextKey.String()` (coverage: 27.8% → 100%).
- **tests**: Logger tests for context field extraction path and mixed field types (`bool`, `float64`, etc.).
- **README**: Documented the auth/JWT package with usage examples.

### Deprecated

- **context**: `AddLoggerToContex` — use `AddLoggerToContext` instead.

### Fixed (formatting)

- **auth**: Added missing trailing newlines in `auth/jwt.go` and `auth/jwt_test.go` (`gofmt -l` now clean).
- **README**: Examples updated to use `AddLoggerToContext` and `AddFieldsToContext` instead of the typo'd function and raw `context.WithValue`.
