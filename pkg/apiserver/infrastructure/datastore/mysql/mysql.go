package mysql

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"context"

	mysqlgorm "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore/sql"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore/sqlnamer"
)

type mysql struct {
	sql.Driver
}

// New mysql datastore instance
func New(ctx context.Context, cfg datastore.Config) (datastore.DataStore, error) {
	db, err := gorm.Open(mysqlgorm.Open(cfg.URL), &gorm.Config{
		NamingStrategy: sqlnamer.SQLNamer{},
		Logger:         logger.Default.LogMode(logger.Silent),
		TranslateError: true,
	})
	if err != nil {
		return nil, err
	}

	for _, v := range model.GetRegisterModels() {
		if err := db.WithContext(ctx).AutoMigrate(v); err != nil {
			return nil, err
		}
	}

	m := &mysql{
		Driver: sql.Driver{
			Client: *db.WithContext(ctx),
		},
	}
	return m, nil
}
