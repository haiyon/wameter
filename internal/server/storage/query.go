package storage

import (
	"fmt"
	"strings"
	"time"
)

// QueryOptions defines query options
type QueryOptions struct {
	BatchSize int           `json:"batch_size,omitempty"`
	Timeout   time.Duration `json:"timeout,omitempty"`
	ReadOnly  bool          `json:"read_only,omitempty"`
}

// MetricsQuery represents a query for metrics data
type MetricsQuery struct {
	AgentIDs    []string  `json:"agent_ids,omitempty"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	MetricTypes []string  `json:"metric_types,omitempty"`
	Limit       int       `json:"limit,omitempty"`
	OrderBy     string    `json:"order_by,omitempty"`
	Order       string    `json:"order,omitempty"`
	Interval    string    `json:"interval,omitempty"`
	Function    string    `json:"function,omitempty"`
}

// QueryBuilder represents query builder
type QueryBuilder struct {
	sql      strings.Builder
	args     []interface{}
	argIndex int
}

// Reset clears the builder
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
func (qb *QueryBuilder) Args() []interface{} {
	return qb.args
}

// Select starts SELECT query
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

// Where adds WHERE clause with placeholder
func (qb *QueryBuilder) Where(cond string, args ...interface{}) *QueryBuilder {
	if !strings.Contains(qb.sql.String(), "WHERE") {
		qb.sql.WriteString(" WHERE ")
	} else {
		qb.sql.WriteString(" AND ")
	}

	// Replace ? with $N for postgres
	if strings.Contains(cond, "?") {
		for range args {
			qb.argIndex++
			cond = strings.Replace(cond, "?", fmt.Sprintf("$%d", qb.argIndex), 1)
		}
	}

	qb.sql.WriteString(cond)
	qb.args = append(qb.args, args...)
	return qb
}

// OrderBy adds ORDER BY
func (qb *QueryBuilder) OrderBy(col string, order string) *QueryBuilder {
	qb.sql.WriteString(" ORDER BY ")
	qb.sql.WriteString(col)
	if order != "" {
		qb.sql.WriteString(" " + order)
	}
	return qb
}

// Limit adds LIMIT
func (qb *QueryBuilder) Limit(n int) *QueryBuilder {
	qb.sql.WriteString(fmt.Sprintf(" LIMIT %d", n))
	return qb
}

// GroupBy adds GROUP BY
func (qb *QueryBuilder) GroupBy(cols ...string) *QueryBuilder {
	qb.sql.WriteString(" GROUP BY ")
	qb.sql.WriteString(strings.Join(cols, ", "))
	return qb
}

// String returns the built query string
func (qb *QueryBuilder) String() string {
	return qb.sql.String()
}

// Having adds HAVING
func (qb *QueryBuilder) Having(cond string, args ...interface{}) *QueryBuilder {
	qb.sql.WriteString(" HAVING ")
	qb.sql.WriteString(cond)
	qb.args = append(qb.args, args...)
	return qb
}

// Join adds JOIN clause
func (qb *QueryBuilder) Join(joinType, table, cond string) *QueryBuilder {
	qb.sql.WriteString(" " + joinType + " JOIN " + table + " ON " + cond)
	return qb
}

// SubQuery adds a subquery
func (qb *QueryBuilder) SubQuery(subQuery string) *QueryBuilder {
	qb.sql.WriteString("(" + subQuery + ")")
	return qb
}

// Union adds UNION
func (qb *QueryBuilder) Union(unionType string, subQuery string) *QueryBuilder {
	qb.sql.WriteString(" " + unionType + " " + subQuery)
	return qb
}

// Raw adds raw SQL
func (qb *QueryBuilder) Raw(sql string, args ...interface{}) *QueryBuilder {
	qb.sql.WriteString(sql)
	qb.args = append(qb.args, args...)
	return qb
}

// Count adds COUNT
func (qb *QueryBuilder) Count(col string) *QueryBuilder {
	qb.sql.WriteString(fmt.Sprintf("COUNT(%s)", col))
	return qb
}

// DateTrunc adds date_trunc function for time bucket
func (qb *QueryBuilder) DateTrunc(interval string, col string) *QueryBuilder {
	qb.sql.WriteString(fmt.Sprintf("date_trunc('%s', %s)", interval, col))
	return qb
}

// MetricsQueryBuilder provides metrics-specific query building
type MetricsQueryBuilder struct {
	QueryBuilder
}

// NewMetricsQueryBuilder creates new metrics query builder
func NewMetricsQueryBuilder() *MetricsQueryBuilder {
	return &MetricsQueryBuilder{}
}

// TimeRange adds time range
func (mqb *MetricsQueryBuilder) TimeRange(start, end time.Time) *MetricsQueryBuilder {
	mqb.Where("timestamp BETWEEN ? AND ?", start, end)
	return mqb
}

// ForAgents adds agent filter
func (mqb *MetricsQueryBuilder) ForAgents(agentIDs []string) *MetricsQueryBuilder {
	if len(agentIDs) > 0 {
		mqb.Where("agent_id IN (?)", agentIDs)
	}
	return mqb
}

// WithMetricTypes adds metric type filter
func (mqb *MetricsQueryBuilder) WithMetricTypes(types []string) *MetricsQueryBuilder {
	if len(types) > 0 {
		mqb.Where("data->>'type' IN (?)", types)
	}
	return mqb
}

// WithAggregation adds aggregation
func (mqb *MetricsQueryBuilder) WithAggregation(interval string) *MetricsQueryBuilder {
	mqb.GroupBy(fmt.Sprintf("date_trunc('%s', timestamp)", interval))
	return mqb
}
