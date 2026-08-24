package sqlcommenter

import (
	"context"
	"fmt"
)

type contextKey string

const CorrelationIDKey contextKey = "x-correlation-id"
const TraceIDKey contextKey = "traceparent"

// WithCorrelationID attaches a correlation ID to a Go context.Context.
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, correlationID)
}

// InjectComment appends SQL commenter metadata to a raw SQL query string.
func InjectComment(ctx context.Context, query string) string {
	correlationID, _ := ctx.Value(CorrelationIDKey).(string)
	traceID, _ := ctx.Value(TraceIDKey).(string)

	if correlationID == "" && traceID == "" {
		return query
	}

	comment := "/*"
	if correlationID != "" {
		comment += fmt.Sprintf("x-correlation-id='%s'", correlationID)
	}
	if traceID != "" {
		if correlationID != "" {
			comment += ","
		}
		comment += fmt.Sprintf("traceparent='%s'", traceID)
	}
	comment += "*/"

	return fmt.Sprintf("%s %s;", query, comment)
}
