package mongo

import (
	"context"
	goctx "github.com/junkd0g/go-microservice-commons/context"
	"github.com/junkd0g/go-microservice-commons/httpx"
	"go.mongodb.org/mongo-driver/event"
	"time"
)

// MongoMonitor correlates slow and failed database commands without logging documents or server addresses.
// MonitorOptions controls logging and expected-failure filtering.
type MonitorOptions struct {
	Logger        goctx.Logger
	IgnoreFailure func(context.Context, *event.CommandFailedEvent) bool
}

// Monitor records redacted slow and failed database operations.
func Monitor(options MonitorOptions) *event.CommandMonitor {
	fallback := options.Logger
	record := func(ctx context.Context, e event.CommandFinishedEvent, failed bool) {
		if !failed && e.Duration < 500*time.Millisecond {
			return
		}
		logger, err := goctx.GetLoggerFromContext(ctx)
		if err != nil {
			logger = fallback
		}
		if logger == nil {
			return
		}
		fields := map[string]interface{}{"request_id": httpx.RequestID(ctx), "database": e.DatabaseName, "operation": e.CommandName, "duration_ms": int(e.Duration.Milliseconds())}
		if failed {
			fields["error_type"] = "command_failed"
			logger.Error(ctx, "Database command failed", fields)
		} else {
			logger.Info(ctx, "Slow database command", fields)
		}
	}
	return &event.CommandMonitor{
		Succeeded: func(ctx context.Context, e *event.CommandSucceededEvent) { record(ctx, e.CommandFinishedEvent, false) },
		Failed: func(ctx context.Context, e *event.CommandFailedEvent) {
			if options.IgnoreFailure != nil && options.IgnoreFailure(ctx, e) {
				return
			}
			record(ctx, e.CommandFinishedEvent, true)
		},
	}
}
