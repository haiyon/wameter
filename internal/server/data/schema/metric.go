package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Metric holds the schema definition for the Metric entity.
type Metric struct {
	ent.Schema
}

// Fields of the Metric.
func (Metric) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").StorageKey("id"),
		field.String("agent_id"),
		field.Time("timestamp"),
		field.Time("collected_at"),
		field.Time("reported_at"),
		field.JSON("data", map[string]any{}),
		field.Time("created_at").Default(time.Now),
	}
}

// Indexes of the Metric.
func (Metric) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("agent_id", "timestamp"),
		index.Fields("collected_at"),
		index.Fields("reported_at"),
	}
}

// Edges of the Metric.
func (Metric) Edges() []ent.Edge {
	return nil
}
