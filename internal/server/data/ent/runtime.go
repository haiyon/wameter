// Code generated by ent, DO NOT EDIT.

package ent

import (
	"time"
	"wameter/internal/server/data/ent/agent"
	"wameter/internal/server/data/ent/metric"
	"wameter/internal/server/data/schema"
)

// The init function reads all schema descriptors with runtime code
// (default values, validators, hooks and policies) and stitches it
// to their package variables.
func init() {
	agentFields := schema.Agent{}.Fields()
	_ = agentFields
	// agentDescHostname is the schema descriptor for hostname field.
	agentDescHostname := agentFields[1].Descriptor()
	// agent.HostnameValidator is a validator for the "hostname" field. It is called by the builders before save.
	agent.HostnameValidator = agentDescHostname.Validators[0].(func(string) error)
	// agentDescVersion is the schema descriptor for version field.
	agentDescVersion := agentFields[2].Descriptor()
	// agent.VersionValidator is a validator for the "version" field. It is called by the builders before save.
	agent.VersionValidator = agentDescVersion.Validators[0].(func(string) error)
	// agentDescStatus is the schema descriptor for status field.
	agentDescStatus := agentFields[3].Descriptor()
	// agent.StatusValidator is a validator for the "status" field. It is called by the builders before save.
	agent.StatusValidator = agentDescStatus.Validators[0].(func(string) error)
	metricFields := schema.Metric{}.Fields()
	_ = metricFields
	// metricDescCreatedAt is the schema descriptor for created_at field.
	metricDescCreatedAt := metricFields[6].Descriptor()
	// metric.DefaultCreatedAt holds the default value on creation for the created_at field.
	metric.DefaultCreatedAt = metricDescCreatedAt.Default.(func() time.Time)
}