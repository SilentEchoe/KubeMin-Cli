package mysql

import (
	datastore "KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

type mysqldb struct {
	client   *sql.DB
	database string
}

func New(ctx context.Context, cfg datastore.Config) (datastore.DataStore, error) {

	db, err := sql.Open("mysql", cfg.URL)
	if err != nil {
		return nil, err
	}

	mysql := &mysqldb{
		client:   db,
		database: cfg.Database,
	}
	return mysql, nil
}

// Add add data model
func (m *mysqldb) Add(ctx context.Context, entity datastore.Entity) error {
	return nil
}

// BatchAdd batch add entity, this operation has some atomicity.
func (m *mysqldb) BatchAdd(ctx context.Context, entities []datastore.Entity) error {
	return nil
}

// Get get data model
func (m *mysqldb) Get(ctx context.Context, entity datastore.Entity) error {
	return nil
}

// Put update data model
func (m *mysqldb) Put(ctx context.Context, entity datastore.Entity) error {
	return nil
}

// IsExist determine whether data exists.
func (m *mysqldb) IsExist(ctx context.Context, entity datastore.Entity) (bool, error) {
	return true, nil
}

// Delete delete data
func (m *mysqldb) Delete(ctx context.Context, entity datastore.Entity) error {
	return nil
}

// List list entity function
func (m *mysqldb) List(ctx context.Context, entity datastore.Entity, op *datastore.ListOptions) ([]datastore.Entity, error) {
	return nil, nil
}

// Count counts entities

func (m *mysqldb) Count(ctx context.Context, entity datastore.Entity, filterOptions *datastore.FilterOptions) (int64, error) {
	return 0, nil
}
