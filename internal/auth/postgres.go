package auth

import (
	"context"
	"database/sql"
	"fmt"
)

type Queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type PostgresAPIKeyStore struct {
	db Queryer
}

func NewPostgresAPIKeyStore(db Queryer) *PostgresAPIKeyStore {
	return &PostgresAPIKeyStore{db: db}
}

func (s *PostgresAPIKeyStore) Lookup(key string) (APIKey, bool) {
	if s == nil || s.db == nil || key == "" {
		return APIKey{}, false
	}

	const query = `
SELECT api_keys.id, api_keys.key, tenants.slug
FROM api_keys
JOIN tenants ON tenants.id = api_keys.tenant_id
WHERE api_keys.key = $1 AND api_keys.revoked_at IS NULL
LIMIT 1`

	var found APIKey
	if err := s.db.QueryRowContext(context.Background(), query, key).Scan(&found.ID, &found.Key, &found.Tenant); err != nil {
		return APIKey{}, false
	}

	return found, true
}

func ValidateAPIKeyStore(store APIKeyStore) error {
	if store == nil {
		return fmt.Errorf("api key store is required")
	}
	return nil
}
