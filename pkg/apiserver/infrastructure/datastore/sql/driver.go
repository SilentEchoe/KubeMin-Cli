package sql

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
)

// Driver is a unified implementation of SQL driver of datastore
type Driver struct {
	Client gorm.DB
}

// Add a data model
func (m *Driver) Add(ctx context.Context, entity datastore.Entity) error {
	if entity.PrimaryKey() == "" {
		return datastore.ErrPrimaryEmpty
	}
	if entity.TableName() == "" {
		return datastore.ErrTableNameEmpty
	}
	entity.SetCreateTime(time.Now())
	entity.SetUpdateTime(time.Now())

	if dbAdd := m.Client.WithContext(ctx).Create(entity); dbAdd.Error != nil {
		if match := errors.Is(dbAdd.Error, gorm.ErrDuplicatedKey); match {
			return datastore.ErrRecordExist
		}
		return datastore.NewDBError(dbAdd.Error)
	}
	return nil
}

// BatchAdd batch adds entity, this operation has some atomicity.
func (m *Driver) BatchAdd(ctx context.Context, entities []datastore.Entity) error {
	notRollback := make(map[string]bool)
	for i, saveEntity := range entities {
		if err := m.Add(ctx, saveEntity); err != nil {
			if errors.Is(err, datastore.ErrRecordExist) {
				notRollback[saveEntity.PrimaryKey()] = true
			}
			for _, deleteEntity := range entities[:i] {
				if _, exit := notRollback[deleteEntity.PrimaryKey()]; !exit {
					if err := m.Delete(ctx, deleteEntity); err != nil {
						if !errors.Is(err, datastore.ErrRecordNotExist) {
							klog.Errorf("rollback delete entity failure %w", err)
						}
					}
				}
			}
			return datastore.NewDBError(fmt.Errorf("save entities occur error, %w", err))
		}
	}
	return nil
}

// Get get data model
func (m *Driver) Get(ctx context.Context, entity datastore.Entity) error {
	if entity.PrimaryKey() == "" {
		return datastore.ErrPrimaryEmpty
	}
	if entity.TableName() == "" {
		return datastore.ErrTableNameEmpty
	}

	if dbGet := m.Client.WithContext(ctx).First(entity); dbGet.Error != nil {
		if errors.Is(dbGet.Error, gorm.ErrRecordNotFound) {
			return datastore.ErrRecordNotExist
		}
		return datastore.NewDBError(dbGet.Error)
	}
	return nil
}

// Put update data model
func (m *Driver) Put(ctx context.Context, entity datastore.Entity) error {
	if entity.PrimaryKey() == "" {
		return datastore.ErrPrimaryEmpty
	}
	if entity.TableName() == "" {
		return datastore.ErrTableNameEmpty
	}
	entity.SetUpdateTime(time.Now())
	if dbPut := m.Client.WithContext(ctx).Model(entity).Updates(entity); dbPut.Error != nil {
		if errors.Is(dbPut.Error, gorm.ErrRecordNotFound) {
			return datastore.ErrRecordNotExist
		}
		return datastore.NewDBError(dbPut.Error)
	}
	return nil
}

// IsExist determine whether data exists.
func (m *Driver) IsExist(ctx context.Context, entity datastore.Entity) (bool, error) {
	if entity.PrimaryKey() == "" {
		return false, datastore.ErrPrimaryEmpty
	}
	if entity.TableName() == "" {
		return false, datastore.ErrTableNameEmpty
	}

	if dbExist := m.Client.WithContext(ctx).First(entity); dbExist.Error != nil {
		if errors.Is(dbExist.Error, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, datastore.NewDBError(dbExist.Error)
	}

	return true, nil
}

// Delete delete data
func (m *Driver) Delete(ctx context.Context, entity datastore.Entity) error {
	if entity.PrimaryKey() == "" {
		return datastore.ErrPrimaryEmpty
	}
	if entity.TableName() == "" {
		return datastore.ErrTableNameEmpty
	}
	// check entity is existed
	if err := m.Get(ctx, entity); err != nil {
		return err
	}

	if dbDelete := m.Client.WithContext(ctx).Model(entity).Delete(entity); dbDelete.Error != nil {
		klog.Errorf("delete document failure %w", dbDelete.Error)
		return datastore.NewDBError(dbDelete.Error)
	}

	return nil
}

// _toColumnName converts keys of the models to lowercase as the column name are in lowercase in the database
func _toColumnName(columnName string) string {
	return strings.ToLower(columnName)
}

func _applyFilterOptions(clauses []clause.Expression, filterOptions datastore.FilterOptions) []clause.Expression {
	for _, queryOp := range filterOptions.Queries {
		clauses = append(clauses, clause.Like{
			Column: _toColumnName(queryOp.Key),
			Value:  fmt.Sprintf("%%%s%%", queryOp.Query),
		})
	}
	for _, queryOp := range filterOptions.In {
		values := make([]interface{}, len(queryOp.Values))
		for i, v := range queryOp.Values {
			values[i] = v
		}
		clauses = append(clauses, clause.IN{
			Column: _toColumnName(queryOp.Key),
			Values: values,
		})
	}
	for _, queryOp := range filterOptions.IsNotExist {
		clauses = append(clauses, clause.Eq{
			Column: _toColumnName(queryOp.Key),
			Value:  "",
		})
	}
	return clauses
}

// List list entity function
func (m *Driver) List(ctx context.Context, entity datastore.Entity, op *datastore.ListOptions) ([]datastore.Entity, error) {
	if entity.TableName() == "" {
		return nil, datastore.ErrTableNameEmpty
	}
	var (
		clauses []clause.Expression
		exprs   []clause.Expression
		limit   int
		offset  int
	)
	if op != nil && op.PageSize > 0 && op.Page > 0 {
		limit = op.PageSize
		offset = op.PageSize * (op.Page - 1)
		clauses = append(clauses, clause.Limit{
			Limit:  &limit,
			Offset: offset,
		})
	}
	for k, v := range entity.Index() {
		exprs = append(exprs, clause.Eq{
			Column: strings.ToLower(k),
			Value:  v,
		})
	}
	if op != nil {
		exprs = _applyFilterOptions(exprs, op.FilterOptions)
	}
	if len(exprs) > 0 {
		clauses = append(clauses, clause.Where{
			Exprs: exprs,
		})
	}
	if op != nil && op.SortBy != nil {
		var sortOption []clause.OrderByColumn
		for _, v := range op.SortBy {
			sortOption = append(sortOption, clause.OrderByColumn{
				Column: clause.Column{
					Name: strings.ToLower(v.Key),
				},
				Desc: v.Order == datastore.SortOrderDescending,
			})
		}
		clauses = append(clauses, clause.OrderBy{
			Columns: sortOption,
		})
	}
	var list []datastore.Entity
	rows, err := m.Client.WithContext(ctx).Model(entity).Clauses(clauses...).Rows()
	if err != nil {
		return nil, datastore.NewDBError(err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			klog.Warningf("close rows failure %s", err.Error())
		}
	}()
	for rows.Next() {
		item, err := datastore.NewEntity(entity)
		if err != nil {
			return nil, datastore.NewDBError(err)
		}
		err = m.Client.WithContext(ctx).ScanRows(rows, &item)
		if err != nil {
			return nil, datastore.NewDBError(fmt.Errorf("row scan failure %w", err))
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		return nil, datastore.NewDBError(err)
	}
	return list, nil
}

// Count counts entities
func (m *Driver) Count(ctx context.Context, entity datastore.Entity, filterOptions *datastore.FilterOptions) (int64, error) {
	if entity.TableName() == "" {
		return 0, datastore.ErrTableNameEmpty
	}
	var (
		count   int64
		exprs   []clause.Expression
		clauses []clause.Expression
	)
	for k, v := range entity.Index() {
		exprs = append(exprs, clause.Eq{
			Column: strings.ToLower(k),
			Value:  v,
		})
	}
	if filterOptions != nil {
		exprs = _applyFilterOptions(exprs, *filterOptions)
	}
	if len(exprs) > 0 {
		clauses = append(clauses, clause.Where{
			Exprs: exprs,
		})
	}
	if dbCount := m.Client.WithContext(ctx).Model(entity).Clauses(clauses...).Count(&count); dbCount.Error != nil {
		return 0, datastore.NewDBError(dbCount.Error)
	}
	return count, nil
}
