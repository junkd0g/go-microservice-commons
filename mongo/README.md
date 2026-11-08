# Optional MongoDB integration

This module provides `NewJobStore` for the shared finite-job runner and
`Monitor` for redacted slow/failed-command diagnostics. It is versioned
separately from commons so newer MongoDB dependencies do not raise the Go
minimum of the base logging, context, HTTP and jobrunner packages.

The module requires Go 1.25 or newer. Use a supported, security-patched Go
release in production. Compression is pinned at the version fixing
GO-2026-6865. Application-specific filtering is supplied through MonitorOptions.

For source development before releasing commons, run `go mod edit
-replace=github.com/junkd0g/go-microservice-commons=..`, test, then drop that
replacement before committing. Published consumers use the version in go.mod.
