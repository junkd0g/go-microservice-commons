# Go Microservice Commons

A reusable Go library providing common microservice infrastructure components including context management and structured logging.

## Features

### Context Package
- Thread-safe mutable fields with `sync.RWMutex`
- Logger interface abstraction
- Context key management for logger and fields
- Zero external dependencies (stdlib only)

### Logger Package
- Zap-based structured logging
- Automatic field extraction from context
- Support for custom log fields
- Context-aware logging with Info and Error levels

## Installation

```bash
go get github.com/junkd0g/go-microservice-commons
```

## Usage

### Basic Logging

```go
package main

import (
    "context"

    goctx "github.com/junkd0g/go-microservice-commons/context"
    "github.com/junkd0g/go-microservice-commons/logger"
)

func main() {
    // Create a new logger
    log, err := logger.NewLogger()
    if err != nil {
        panic(err)
    }

    // Add logger to context
    ctx := context.Background()
    ctx = goctx.AddLoggerToContex(ctx, log)

    // Use the logger
    log.Info(ctx, "Application started", map[string]interface{}{
        "service": "my-service",
        "version": 1,
    })
}
```

### Context with Mutable Fields

```go
// Create mutable fields
mutableFields := goctx.NewMutableFields()
mutableFields.AddField(map[string]interface{}{"request_id": "abc-123"})

// Add to context
ctx = context.WithValue(ctx, goctx.ContextKeyLoggerFields, mutableFields)

// Logger will automatically include these fields
log.Info(ctx, "Processing request")
// Output: {"level":"info","msg":"Processing request","request_id":"abc-123"}
```

### HTTP Middleware Example

```go
func LoggerMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log, _ := logger.NewLogger()
        ctx := goctx.AddLoggerToContex(r.Context(), log)

        // Add request metadata
        mutableFields := goctx.NewMutableFields()
        mutableFields.AddField(map[string]interface{}{
            "path": r.URL.Path,
            "method": r.Method,
        })
        ctx = context.WithValue(ctx, goctx.ContextKeyLoggerFields, mutableFields)

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

## Testing

```bash
go test ./...
```

## License

MIT License - See [LICENSE](LICENSE) file for details

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Author

Iordanis Paschalidis ([@junkd0g](https://github.com/junkd0g))