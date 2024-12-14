// Code generated by ent, DO NOT EDIT.

package agent

import (
	"entgo.io/ent/dialect/sql"
)

const (
	// Label holds the string label denoting the agent type in the database.
	Label = "agent"
	// FieldID holds the string denoting the id field in the database.
	FieldID = "id"
	// FieldHostname holds the string denoting the hostname field in the database.
	FieldHostname = "hostname"
	// FieldVersion holds the string denoting the version field in the database.
	FieldVersion = "version"
	// FieldStatus holds the string denoting the status field in the database.
	FieldStatus = "status"
	// FieldLastSeen holds the string denoting the last_seen field in the database.
	FieldLastSeen = "last_seen"
	// FieldRegisteredAt holds the string denoting the registered_at field in the database.
	FieldRegisteredAt = "registered_at"
	// FieldUpdatedAt holds the string denoting the updated_at field in the database.
	FieldUpdatedAt = "updated_at"
	// Table holds the table name of the agent in the database.
	Table = "agents"
)

// Columns holds all SQL columns for agent fields.
var Columns = []string{
	FieldID,
	FieldHostname,
	FieldVersion,
	FieldStatus,
	FieldLastSeen,
	FieldRegisteredAt,
	FieldUpdatedAt,
}

// ValidColumn reports if the column name is valid (part of the table columns).
func ValidColumn(column string) bool {
	for i := range Columns {
		if column == Columns[i] {
			return true
		}
	}
	return false
}

var (
	// HostnameValidator is a validator for the "hostname" field. It is called by the builders before save.
	HostnameValidator func(string) error
	// VersionValidator is a validator for the "version" field. It is called by the builders before save.
	VersionValidator func(string) error
	// StatusValidator is a validator for the "status" field. It is called by the builders before save.
	StatusValidator func(string) error
)

// OrderOption defines the ordering options for the Agent queries.
type OrderOption func(*sql.Selector)

// ByID orders the results by the id field.
func ByID(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldID, opts...).ToFunc()
}

// ByHostname orders the results by the hostname field.
func ByHostname(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldHostname, opts...).ToFunc()
}

// ByVersion orders the results by the version field.
func ByVersion(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldVersion, opts...).ToFunc()
}

// ByStatus orders the results by the status field.
func ByStatus(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldStatus, opts...).ToFunc()
}

// ByLastSeen orders the results by the last_seen field.
func ByLastSeen(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldLastSeen, opts...).ToFunc()
}

// ByRegisteredAt orders the results by the registered_at field.
func ByRegisteredAt(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldRegisteredAt, opts...).ToFunc()
}

// ByUpdatedAt orders the results by the updated_at field.
func ByUpdatedAt(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldUpdatedAt, opts...).ToFunc()
}