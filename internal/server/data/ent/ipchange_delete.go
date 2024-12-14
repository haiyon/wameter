// Code generated by ent, DO NOT EDIT.

package ent

import (
	"context"
	"wameter/internal/server/data/ent/ipchange"
	"wameter/internal/server/data/ent/predicate"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"entgo.io/ent/schema/field"
)

// IPChangeDelete is the builder for deleting a IPChange entity.
type IPChangeDelete struct {
	config
	hooks    []Hook
	mutation *IPChangeMutation
}

// Where appends a list predicates to the IPChangeDelete builder.
func (icd *IPChangeDelete) Where(ps ...predicate.IPChange) *IPChangeDelete {
	icd.mutation.Where(ps...)
	return icd
}

// Exec executes the deletion query and returns how many vertices were deleted.
func (icd *IPChangeDelete) Exec(ctx context.Context) (int, error) {
	return withHooks(ctx, icd.sqlExec, icd.mutation, icd.hooks)
}

// ExecX is like Exec, but panics if an error occurs.
func (icd *IPChangeDelete) ExecX(ctx context.Context) int {
	n, err := icd.Exec(ctx)
	if err != nil {
		panic(err)
	}
	return n
}

func (icd *IPChangeDelete) sqlExec(ctx context.Context) (int, error) {
	_spec := sqlgraph.NewDeleteSpec(ipchange.Table, sqlgraph.NewFieldSpec(ipchange.FieldID, field.TypeInt64))
	if ps := icd.mutation.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	affected, err := sqlgraph.DeleteNodes(ctx, icd.driver, _spec)
	if err != nil && sqlgraph.IsConstraintError(err) {
		err = &ConstraintError{msg: err.Error(), wrap: err}
	}
	icd.mutation.done = true
	return affected, err
}

// IPChangeDeleteOne is the builder for deleting a single IPChange entity.
type IPChangeDeleteOne struct {
	icd *IPChangeDelete
}

// Where appends a list predicates to the IPChangeDelete builder.
func (icdo *IPChangeDeleteOne) Where(ps ...predicate.IPChange) *IPChangeDeleteOne {
	icdo.icd.mutation.Where(ps...)
	return icdo
}

// Exec executes the deletion query.
func (icdo *IPChangeDeleteOne) Exec(ctx context.Context) error {
	n, err := icdo.icd.Exec(ctx)
	switch {
	case err != nil:
		return err
	case n == 0:
		return &NotFoundError{ipchange.Label}
	default:
		return nil
	}
}

// ExecX is like Exec, but panics if an error occurs.
func (icdo *IPChangeDeleteOne) ExecX(ctx context.Context) {
	if err := icdo.Exec(ctx); err != nil {
		panic(err)
	}
}
