/*
Copyright 2021 The KubeVela Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package datastore

import (
	"context"
	"fmt"
	"reflect"
	"time"
)

var (

	// ErrPrimaryEmpty Error that primary key is empty.
	ErrPrimaryEmpty = NewDBError(fmt.Errorf("entity primary is empty"))

	// ErrTableNameEmpty Error that table name is empty.
	ErrTableNameEmpty = NewDBError(fmt.Errorf("entity table name is empty"))

	// ErrNilEntity Error that entity is nil
	ErrNilEntity = NewDBError(fmt.Errorf("entity is nil"))

	// ErrRecordExist Error that entity primary key is exist
	ErrRecordExist = NewDBError(fmt.Errorf("data record is exist"))

	// ErrRecordNotExist Error that entity primary key is not exist
	ErrRecordNotExist = NewDBError(fmt.Errorf("data record is not exist"))

	// ErrIndexInvalid Error that entity index is invalid
	ErrIndexInvalid = NewDBError(fmt.Errorf("entity index is invalid"))

	// ErrEntityInvalid Error that entity is invalid
	ErrEntityInvalid = NewDBError(fmt.Errorf("entity is invalid"))
)

// PrimaryKeyMaxLength The primary key length should be limited when the datastore is kube-api
var PrimaryKeyMaxLength = 31

// DBError datastore error
type DBError struct {
	err error
}

func (d *DBError) Error() string {
	return d.err.Error()
}

// NewDBError new datastore error
func NewDBError(err error) error {
	return &DBError{err: err}
}

// Config datastore config
type Config struct {
	Type     string
	URL      string
	Database string
	// MaxIdleConns defines the maximum number of idle connections kept in the pool.
	MaxIdleConns int
	// MaxOpenConns limits the total number of open connections to the database.
	MaxOpenConns int
	// ConnMaxLifetime bounds the lifetime of a connection before it's recycled.
	ConnMaxLifetime time.Duration
	// ConnMaxIdleTime bounds how long an idle connection is kept in the pool.
	ConnMaxIdleTime time.Duration
}

// Entity database data model
type Entity interface {
	SetCreateTime(time time.Time)
	SetUpdateTime(time time.Time)
	PrimaryKey() string
	TableName() string
	ShortTableName() string
	Index() map[string]interface{}
}

// NewEntity Create a new object based on the input type
func NewEntity(in Entity) (Entity, error) {
	if in == nil {
		return nil, ErrNilEntity
	}
	t := reflect.TypeOf(in)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	newEntity := reflect.New(t)
	return newEntity.Interface().(Entity), nil
}

// SortOrder is the order of sort
type SortOrder int

const (
	// SortOrderAscending defines the order of ascending for sorting
	SortOrderAscending = SortOrder(1)
	// SortOrderDescending defines the order of descending for sorting
	SortOrderDescending = SortOrder(-1)
)

// SortOption describes the sorting parameters for list
type SortOption struct {
	Key   string
	Order SortOrder
}

// FuzzyQueryOption defines the fuzzy query search filter option
type FuzzyQueryOption struct {
	Key   string
	Query string
}

// InQueryOption defines the include search filter option
type InQueryOption struct {
	Key    string
	Values []string
}

// IsNotExistQueryOption means the value is empty
type IsNotExistQueryOption struct {
	Key string
}

// FilterOptions filter query returned items
type FilterOptions struct {
	Queries    []FuzzyQueryOption
	In         []InQueryOption
	IsNotExist []IsNotExistQueryOption
}

// ListOptions list api options
type ListOptions struct {
	FilterOptions
	Page     int
	PageSize int
	SortBy   []SortOption
}

// DataStore datastore interface
type DataStore interface {
	// Add adds entity to database, Name() and TableName() can't return zero value.
	Add(ctx context.Context, entity Entity) error

	// BatchAdd will adds batched entities to database, Name() and TableName() can't return zero value.
	BatchAdd(ctx context.Context, entities []Entity) error

	// Put will update entity to database, Name() and TableName() can't return zero value.
	Put(ctx context.Context, entity Entity) error

	// Delete entity from database, Name() and TableName() can't return zero value.
	Delete(ctx context.Context, entity Entity) error

	// DeleteByFilter deletes entities matching the provided index fields and optional filters.
	DeleteByFilter(ctx context.Context, entity Entity, options *FilterOptions) error

	// Get entity from database, Name() and TableName() can't return zero value.
	Get(ctx context.Context, entity Entity) error

	// List entities from database, TableName() can't return zero value, if no matches, it will return a zero list without error.
	List(ctx context.Context, query Entity, options *ListOptions) ([]Entity, error)

	// Count entities from database, TableName() can't return zero value.
	Count(ctx context.Context, entity Entity, options *FilterOptions) (int64, error)

	// IsExist Name() and TableName() can't return zero value.
	IsExist(ctx context.Context, entity Entity) (bool, error)

	//IsExistByCondition 多条件判断实体是否存在
	IsExistByCondition(ctx context.Context, table string, cond map[string]interface{}, dest interface{}) (bool, error)
}
