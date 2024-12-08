package database

import (
	"fmt"
	"strings"
)

// QueryBuilder provides SQL query building functionality
type QueryBuilder struct {
	sql      strings.Builder
	args     []any
	argIndex int
	driver   string
}

// NewQueryBuilder creates new query builder
func NewQueryBuilder(driver string) *QueryBuilder {
	return &QueryBuilder{
		driver: driver,
		args:   make([]any, 0),
	}
}

// Reset resets the builder state
func (qb *QueryBuilder) Reset() {
	qb.sql.Reset()
	qb.args = qb.args[:0]
	qb.argIndex = 0
}

// SQL returns the built query string
func (qb *QueryBuilder) SQL() string {
	return qb.sql.String()
}

// Args returns query arguments
func (qb *QueryBuilder) Args() []any {
	return qb.args
}

// Select adds SELECT clause
func (qb *QueryBuilder) Select(cols ...string) *QueryBuilder {
	qb.sql.WriteString("SELECT ")
	qb.sql.WriteString(strings.Join(cols, ", "))
	return qb
}

// From adds FROM clause
func (qb *QueryBuilder) From(table string) *QueryBuilder {
	qb.sql.WriteString(" FROM ")
	qb.sql.WriteString(table)
	return qb
}

// Where adds WHERE condition
func (qb *QueryBuilder) Where(cond string, args ...any) *QueryBuilder {
	if !strings.Contains(qb.sql.String(), "WHERE") {
		qb.sql.WriteString(" WHERE ")
	} else {
		qb.sql.WriteString(" AND ")
	}

	// Convert placeholders based on driver
	switch qb.driver {
	case "postgres":
		for range args {
			qb.argIndex++
			cond = strings.Replace(cond, "?", fmt.Sprintf("$%d", qb.argIndex), 1)
		}
	}

	qb.sql.WriteString(cond)
	qb.args = append(qb.args, args...)
	return qb
}

// OrderBy adds ORDER BY clause
func (qb *QueryBuilder) OrderBy(cols ...string) *QueryBuilder {
	qb.sql.WriteString(" ORDER BY ")
	qb.sql.WriteString(strings.Join(cols, ", "))
	return qb
}

// GroupBy adds GROUP BY clause
func (qb *QueryBuilder) GroupBy(cols ...string) *QueryBuilder {
	qb.sql.WriteString(" GROUP BY ")
	qb.sql.WriteString(strings.Join(cols, ", "))
	return qb
}

// Having adds HAVING clause
func (qb *QueryBuilder) Having(cond string, args ...any) *QueryBuilder {
	qb.sql.WriteString(" HAVING ")

	// Convert placeholders for postgres
	if qb.driver == "postgres" {
		for range args {
			qb.argIndex++
			cond = strings.Replace(cond, "?", fmt.Sprintf("$%d", qb.argIndex), 1)
		}
	}

	qb.sql.WriteString(cond)
	qb.args = append(qb.args, args...)
	return qb
}

// Limit adds LIMIT clause
func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	if limit > 0 {
		qb.sql.WriteString(fmt.Sprintf(" LIMIT %d", limit))
	}
	return qb
}

// Offset adds OFFSET clause
func (qb *QueryBuilder) Offset(offset int) *QueryBuilder {
	if offset > 0 {
		qb.sql.WriteString(fmt.Sprintf(" OFFSET %d", offset))
	}
	return qb
}

// Join adds JOIN clause
func (qb *QueryBuilder) Join(joinType, table, cond string) *QueryBuilder {
	qb.sql.WriteString(fmt.Sprintf(" %s JOIN %s ON %s", joinType, table, cond))
	return qb
}

// SubQuery adds a subquery
func (qb *QueryBuilder) SubQuery(subQuery string) *QueryBuilder {
	qb.sql.WriteString("(")
	qb.sql.WriteString(subQuery)
	qb.sql.WriteString(")")
	return qb
}

// Union adds UNION clause
func (qb *QueryBuilder) Union(all bool, subQuery string) *QueryBuilder {
	if all {
		qb.sql.WriteString(" UNION ALL ")
	} else {
		qb.sql.WriteString(" UNION ")
	}
	qb.sql.WriteString(subQuery)
	return qb
}

// Raw adds raw SQL
func (qb *QueryBuilder) Raw(sql string, args ...any) *QueryBuilder {
	if qb.driver == "postgres" {
		for range args {
			qb.argIndex++
			sql = strings.Replace(sql, "?", fmt.Sprintf("$%d", qb.argIndex), 1)
		}
	}
	qb.sql.WriteString(sql)
	qb.args = append(qb.args, args...)
	return qb
}
