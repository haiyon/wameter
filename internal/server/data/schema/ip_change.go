package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// IPChange holds the schema definition for the IPChange entity.
type IPChange struct {
	ent.Schema
}

// Fields of the IPChange.
func (IPChange) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").StorageKey("id"),
		field.String("agent_id"),
		field.String("interface_name").Optional(),
		field.String("version"),
		field.Bool("is_external"),
		field.JSON("old_addrs", map[string]any{}).Optional(),
		field.JSON("new_addrs", map[string]any{}).Optional(),
		field.String("action"),
		field.String("reason"),
		field.Time("timestamp"),
		field.Time("created_at"),
	}
}

// Indexes of the IPChange.
func (IPChange) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("agent_id", "timestamp"),
		index.Fields("interface_name"),
		index.Fields("created_at"),
	}
}

// Edges of the IPChange.
func (IPChange) Edges() []ent.Edge {
	return nil
}
