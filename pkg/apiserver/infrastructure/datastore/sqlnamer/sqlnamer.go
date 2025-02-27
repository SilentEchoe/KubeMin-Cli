package sqlnamer

import (
	"strings"

	"gorm.io/gorm/schema"
)

// SQLNamer overrides the default naming conventions used by gorm
type SQLNamer struct {
	// DefaultNamingStrategy to use the methods of the default NamingStrategy of gorm if modification not needed
	DefaultNamingStrategy schema.NamingStrategy
}

func (namer SQLNamer) UniqueName(table, column string) string {
	//TODO implement me
	panic("implement me")
}

// TableName convert string to table name
func (namer SQLNamer) TableName(str string) string {
	return namer.DefaultNamingStrategy.TableName(str)
}

// SchemaName generate schema name from table name, don't guarantee it is the reverse value of TableName
func (namer SQLNamer) SchemaName(table string) string {
	return namer.DefaultNamingStrategy.SchemaName(table)
}

// ColumnName overrides the DefaultNamingStrategy.ColumnName as it converts the column names to snake-case, but we need them in lowercase
func (namer SQLNamer) ColumnName(_, column string) string {
	return strings.ToLower(column)
}

// JoinTableName convert string to join table name
func (namer SQLNamer) JoinTableName(str string) string {
	return namer.DefaultNamingStrategy.JoinTableName(str)
}

// RelationshipFKName generate fk name for relation
func (namer SQLNamer) RelationshipFKName(rel schema.Relationship) string {
	return namer.DefaultNamingStrategy.RelationshipFKName(rel)
}

// CheckerName generate checker name
func (namer SQLNamer) CheckerName(table, column string) string {
	return namer.DefaultNamingStrategy.CheckerName(table, column)
}

// IndexName generate index name
func (namer SQLNamer) IndexName(table, column string) string {
	return namer.DefaultNamingStrategy.IndexName(table, column)
}
