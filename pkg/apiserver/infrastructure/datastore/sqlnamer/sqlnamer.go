package sqlnamer

import (
	"fmt"
	"strings"

	"gorm.io/gorm/schema"
)

// SQLNamer overrides the default naming conventions used by gorm
type SQLNamer struct {
	// DefaultNamingStrategy to use the methods of the default NamingStrategy of gorm if modification not needed
	DefaultNamingStrategy schema.NamingStrategy
}

func (namer SQLNamer) UniqueName(table, column string) string {
	// 生成一个唯一约束名称，格式为：uk_{表名}_{列名}
	// 由于MySQL中标识符长度限制，需要确保名称不超过64个字符
	const maxLength = 64
	const prefix = "uk_"

	// 计算表名和列名可用的最大长度
	availableLength := maxLength - len(prefix) - 1 // 减1是为了下划线

	// 如果表名和列名的总长度超过可用长度，则需要截断
	if len(table)+len(column) > availableLength {
		// 平均分配可用长度给表名和列名
		tableMaxLen := availableLength / 2
		columnMaxLen := availableLength - tableMaxLen

		if len(table) > tableMaxLen {
			table = table[:tableMaxLen]
		}

		if len(column) > columnMaxLen {
			column = column[:columnMaxLen]
		}
	}
	return fmt.Sprintf("%s%s_%s", prefix, table, column)
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
