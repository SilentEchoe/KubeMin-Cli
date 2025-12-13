package sql

import (
	"context"

	"gorm.io/gorm"

	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
)

// WithTransaction runs fn within a database transaction.
// The provided tx datastore must be used for all operations that should be atomic.
func (m *Driver) WithTransaction(ctx context.Context, fn func(tx datastore.DataStore) error) error {
	return m.Client.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Driver{Client: *tx})
	})
}
