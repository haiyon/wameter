package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Agent holds the schema definition for the Agent entity.
type Agent struct {
	ent.Schema
}

// Fields of the Agent.
func (Agent) Fields() []ent.Field {
	return []ent.Field{
		field.String("id"),
		field.String("hostname").NotEmpty(),
		field.String("version").NotEmpty(),
		field.String("status").NotEmpty(),
		field.Time("last_seen"),
		field.Time("registered_at"),
		field.Time("updated_at"),
	}
}

// Indexes of the Agent.
func (Agent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
		index.Fields("last_seen"),
	}
}

// Edges of the Agent.
func (Agent) Edges() []ent.Edge {
	return nil
}
