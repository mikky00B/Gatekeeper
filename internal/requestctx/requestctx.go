package requestctx

import "context"

type contextKey string

const logFieldsContextKey contextKey = "log_fields"

type LogFields struct {
	RouteID string
}

func WithLogFields(ctx context.Context, fields *LogFields) context.Context {
	return context.WithValue(ctx, logFieldsContextKey, fields)
}

func LogFieldsFromContext(ctx context.Context) (*LogFields, bool) {
	fields, ok := ctx.Value(logFieldsContextKey).(*LogFields)
	return fields, ok
}
