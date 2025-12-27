package mysql

import (
	"kubemin-cli/pkg/apiserver/domain/model"
	"context"

	mysqlgorm "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"kubemin-cli/pkg/apiserver/infrastructure/datastore"
	"kubemin-cli/pkg/apiserver/infrastructure/datastore/sql"
	"kubemin-cli/pkg/apiserver/infrastructure/datastore/sqlnamer"
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

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
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
