// Code generated by ent, DO NOT EDIT.

package ent

import (
	"context"
	"errors"
	"fmt"
	"time"
	"wameter/internal/server/data/ent/metric"

	"entgo.io/ent/dialect/sql/sqlgraph"
	"entgo.io/ent/schema/field"
)

// MetricCreate is the builder for creating a Metric entity.
type MetricCreate struct {
	config
	mutation *MetricMutation
	hooks    []Hook
}

// SetAgentID sets the "agent_id" field.
func (mc *MetricCreate) SetAgentID(s string) *MetricCreate {
	mc.mutation.SetAgentID(s)
	return mc
}

// SetTimestamp sets the "timestamp" field.
func (mc *MetricCreate) SetTimestamp(t time.Time) *MetricCreate {
	mc.mutation.SetTimestamp(t)
	return mc
}

// SetCollectedAt sets the "collected_at" field.
func (mc *MetricCreate) SetCollectedAt(t time.Time) *MetricCreate {
	mc.mutation.SetCollectedAt(t)
	return mc
}

// SetReportedAt sets the "reported_at" field.
func (mc *MetricCreate) SetReportedAt(t time.Time) *MetricCreate {
	mc.mutation.SetReportedAt(t)
	return mc
}

// SetData sets the "data" field.
func (mc *MetricCreate) SetData(m map[string]interface{}) *MetricCreate {
	mc.mutation.SetData(m)
	return mc
}

// SetCreatedAt sets the "created_at" field.
func (mc *MetricCreate) SetCreatedAt(t time.Time) *MetricCreate {
	mc.mutation.SetCreatedAt(t)
	return mc
}

// SetNillableCreatedAt sets the "created_at" field if the given value is not nil.
func (mc *MetricCreate) SetNillableCreatedAt(t *time.Time) *MetricCreate {
	if t != nil {
		mc.SetCreatedAt(*t)
	}
	return mc
}

// SetID sets the "id" field.
func (mc *MetricCreate) SetID(i int64) *MetricCreate {
	mc.mutation.SetID(i)
	return mc
}

// Mutation returns the MetricMutation object of the builder.
func (mc *MetricCreate) Mutation() *MetricMutation {
	return mc.mutation
}

// Save creates the Metric in the database.
func (mc *MetricCreate) Save(ctx context.Context) (*Metric, error) {
	mc.defaults()
	return withHooks(ctx, mc.sqlSave, mc.mutation, mc.hooks)
}

// SaveX calls Save and panics if Save returns an error.
func (mc *MetricCreate) SaveX(ctx context.Context) *Metric {
	v, err := mc.Save(ctx)
	if err != nil {
		panic(err)
	}
	return v
}

// Exec executes the query.
func (mc *MetricCreate) Exec(ctx context.Context) error {
	_, err := mc.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (mc *MetricCreate) ExecX(ctx context.Context) {
	if err := mc.Exec(ctx); err != nil {
		panic(err)
	}
}

// defaults sets the default values of the builder before save.
func (mc *MetricCreate) defaults() {
	if _, ok := mc.mutation.CreatedAt(); !ok {
		v := metric.DefaultCreatedAt()
		mc.mutation.SetCreatedAt(v)
	}
}

// check runs all checks and user-defined validators on the builder.
func (mc *MetricCreate) check() error {
	if _, ok := mc.mutation.AgentID(); !ok {
		return &ValidationError{Name: "agent_id", err: errors.New(`ent: missing required field "Metric.agent_id"`)}
	}
	if _, ok := mc.mutation.Timestamp(); !ok {
		return &ValidationError{Name: "timestamp", err: errors.New(`ent: missing required field "Metric.timestamp"`)}
	}
	if _, ok := mc.mutation.CollectedAt(); !ok {
		return &ValidationError{Name: "collected_at", err: errors.New(`ent: missing required field "Metric.collected_at"`)}
	}
	if _, ok := mc.mutation.ReportedAt(); !ok {
		return &ValidationError{Name: "reported_at", err: errors.New(`ent: missing required field "Metric.reported_at"`)}
	}
	if _, ok := mc.mutation.Data(); !ok {
		return &ValidationError{Name: "data", err: errors.New(`ent: missing required field "Metric.data"`)}
	}
	if _, ok := mc.mutation.CreatedAt(); !ok {
		return &ValidationError{Name: "created_at", err: errors.New(`ent: missing required field "Metric.created_at"`)}
	}
	return nil
}

func (mc *MetricCreate) sqlSave(ctx context.Context) (*Metric, error) {
	if err := mc.check(); err != nil {
		return nil, err
	}
	_node, _spec := mc.createSpec()
	if err := sqlgraph.CreateNode(ctx, mc.driver, _spec); err != nil {
		if sqlgraph.IsConstraintError(err) {
			err = &ConstraintError{msg: err.Error(), wrap: err}
		}
		return nil, err
	}
	if _spec.ID.Value != _node.ID {
		id := _spec.ID.Value.(int64)
		_node.ID = int64(id)
	}
	mc.mutation.id = &_node.ID
	mc.mutation.done = true
	return _node, nil
}

func (mc *MetricCreate) createSpec() (*Metric, *sqlgraph.CreateSpec) {
	var (
		_node = &Metric{config: mc.config}
		_spec = sqlgraph.NewCreateSpec(metric.Table, sqlgraph.NewFieldSpec(metric.FieldID, field.TypeInt64))
	)
	if id, ok := mc.mutation.ID(); ok {
		_node.ID = id
		_spec.ID.Value = id
	}
	if value, ok := mc.mutation.AgentID(); ok {
		_spec.SetField(metric.FieldAgentID, field.TypeString, value)
		_node.AgentID = value
	}
	if value, ok := mc.mutation.Timestamp(); ok {
		_spec.SetField(metric.FieldTimestamp, field.TypeTime, value)
		_node.Timestamp = value
	}
	if value, ok := mc.mutation.CollectedAt(); ok {
		_spec.SetField(metric.FieldCollectedAt, field.TypeTime, value)
		_node.CollectedAt = value
	}
	if value, ok := mc.mutation.ReportedAt(); ok {
		_spec.SetField(metric.FieldReportedAt, field.TypeTime, value)
		_node.ReportedAt = value
	}
	if value, ok := mc.mutation.Data(); ok {
		_spec.SetField(metric.FieldData, field.TypeJSON, value)
		_node.Data = value
	}
	if value, ok := mc.mutation.CreatedAt(); ok {
		_spec.SetField(metric.FieldCreatedAt, field.TypeTime, value)
		_node.CreatedAt = value
	}
	return _node, _spec
}

// MetricCreateBulk is the builder for creating many Metric entities in bulk.
type MetricCreateBulk struct {
	config
	err      error
	builders []*MetricCreate
}

// Save creates the Metric entities in the database.
func (mcb *MetricCreateBulk) Save(ctx context.Context) ([]*Metric, error) {
	if mcb.err != nil {
		return nil, mcb.err
	}
	specs := make([]*sqlgraph.CreateSpec, len(mcb.builders))
	nodes := make([]*Metric, len(mcb.builders))
	mutators := make([]Mutator, len(mcb.builders))
	for i := range mcb.builders {
		func(i int, root context.Context) {
			builder := mcb.builders[i]
			builder.defaults()
			var mut Mutator = MutateFunc(func(ctx context.Context, m Mutation) (Value, error) {
				mutation, ok := m.(*MetricMutation)
				if !ok {
					return nil, fmt.Errorf("unexpected mutation type %T", m)
				}
				if err := builder.check(); err != nil {
					return nil, err
				}
				builder.mutation = mutation
				var err error
				nodes[i], specs[i] = builder.createSpec()
				if i < len(mutators)-1 {
					_, err = mutators[i+1].Mutate(root, mcb.builders[i+1].mutation)
				} else {
					spec := &sqlgraph.BatchCreateSpec{Nodes: specs}
					// Invoke the actual operation on the latest mutation in the chain.
					if err = sqlgraph.BatchCreate(ctx, mcb.driver, spec); err != nil {
						if sqlgraph.IsConstraintError(err) {
							err = &ConstraintError{msg: err.Error(), wrap: err}
						}
					}
				}
				if err != nil {
					return nil, err
				}
				mutation.id = &nodes[i].ID
				if specs[i].ID.Value != nil && nodes[i].ID == 0 {
					id := specs[i].ID.Value.(int64)
					nodes[i].ID = int64(id)
				}
				mutation.done = true
				return nodes[i], nil
			})
			for i := len(builder.hooks) - 1; i >= 0; i-- {
				mut = builder.hooks[i](mut)
			}
			mutators[i] = mut
		}(i, ctx)
	}
	if len(mutators) > 0 {
		if _, err := mutators[0].Mutate(ctx, mcb.builders[0].mutation); err != nil {
			return nil, err
		}
	}
	return nodes, nil
}

// SaveX is like Save, but panics if an error occurs.
func (mcb *MetricCreateBulk) SaveX(ctx context.Context) []*Metric {
	v, err := mcb.Save(ctx)
	if err != nil {
		panic(err)
	}
	return v
}

// Exec executes the query.
func (mcb *MetricCreateBulk) Exec(ctx context.Context) error {
	_, err := mcb.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (mcb *MetricCreateBulk) ExecX(ctx context.Context) {
	if err := mcb.Exec(ctx); err != nil {
		panic(err)
	}
}
