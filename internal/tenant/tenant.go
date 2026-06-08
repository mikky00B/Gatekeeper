package tenant

import "context"

type contextKey string

const contextKeyTenant contextKey = "tenant"

type Tenant struct {
	ID   string
	Slug string
}

func WithTenant(ctx context.Context, t Tenant) context.Context {
	return context.WithValue(ctx, contextKeyTenant, t)
}

func FromContext(ctx context.Context) (Tenant, bool) {
	t, ok := ctx.Value(contextKeyTenant).(Tenant)
	return t, ok
}
