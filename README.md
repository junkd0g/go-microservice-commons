# Go Microservice Commons

A reusable Go library providing common microservice infrastructure components including context management, structured logging, and JWT authentication.

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

### Auth Package
- JWT token generation and validation using HMAC-SHA256
- Configurable expiration, issuer, and secret key
- Algorithm-confusion attack prevention
- Custom claims with user ID and email

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
    ctx = goctx.AddLoggerToContext(ctx, log)

    // Use the logger
    log.Info(ctx, "Application started", map[string]interface{}{
        "service": "my-service",
        "version": 1,
    })
}
```

### Context with Fields

```go
// Add fields to context — the logger picks them up automatically
ctx = goctx.AddFieldsToContext(ctx, []map[string]interface{}{
    {"request_id": "abc-123"},
})

log.Info(ctx, "Processing request")
// Output: {"level":"info","msg":"Processing request","request_id":"abc-123"}
```

### HTTP Middleware Example

```go
func LoggerMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log, _ := logger.NewLogger()
        ctx := goctx.AddLoggerToContext(r.Context(), log)

        // Add request metadata
        ctx = goctx.AddFieldsToContext(ctx, []map[string]interface{}{
            {
                "path":   r.URL.Path,
                "method": r.Method,
            },
        })

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### JWT Authentication

```go
package main

import (
    "context"
    "fmt"

    "github.com/junkd0g/go-microservice-commons/auth"
)

func main() {
    // Create a JWT wrapper
    jwtWrapper, err := auth.NewJwtWrapper("my-secret-key", "my-service", 24)
    if err != nil {
        panic(err)
    }

    // Generate a token
    ctx := context.Background()
    token, err := jwtWrapper.GenerateToken(ctx, "user-uuid", "user@example.com")
    if err != nil {
        panic(err)
    }
    fmt.Println("Token:", token)

    // Validate the token
    claims, err := jwtWrapper.ValidateToken(ctx, token)
    if err != nil {
        panic(err)
    }
    fmt.Println("User ID:", claims.ID)
    fmt.Println("Email:", claims.Email)
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