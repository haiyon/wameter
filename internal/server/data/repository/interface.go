package repository

import (
	"context"
	"time"
	"wameter/internal/types"
)

// AgentRepository defines agent storage operations
type AgentRepository interface {
	Save(ctx context.Context, agent *types.AgentInfo) error
	FindByID(ctx context.Context, id string) (*types.AgentInfo, error)
	UpdateAgent(ctx context.Context, agent *types.AgentInfo) error
	UpdateStatus(ctx context.Context, id string, status types.AgentStatus) error
	List(ctx context.Context) ([]*types.AgentInfo, error)
	ListWithPagination(ctx context.Context, limit, offset int) ([]*types.AgentInfo, error)
	Delete(ctx context.Context, id string) error
	GetAgentMetrics(ctx context.Context, id string) (*types.AgentMetrics, error)
}

// IPChangeRepository defines IP change storage operations
type IPChangeRepository interface {
	Save(ctx context.Context, agentID string, change *types.IPChange) error
	GetRecentChanges(ctx context.Context, agentID string, since time.Time) ([]*types.IPChange, error)
	DeleteBefore(ctx context.Context, before time.Time) error
	GetChangeSummary(ctx context.Context, agentID string) (*types.IPChangeSummary, error)
	GetInterfaceChanges(ctx context.Context, agentID, interfaceName string, since time.Time) ([]*types.IPChange, error)
}

// MetricsRepository defines metrics storage operations
type MetricsRepository interface {
	Save(ctx context.Context, data *types.MetricsData) error
	BatchSave(ctx context.Context, metrics []*types.MetricsData) error
	Query(ctx context.Context, params QueryParams) ([]*types.MetricsData, error)
	GetLatest(ctx context.Context, agentID string) (*types.MetricsData, error)
	DeleteBefore(ctx context.Context, before time.Time) error
	GetMetricsByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*types.MetricsData, error)
	GetMetricsSummary(ctx context.Context, agentID string) (*types.MetricsSummary, error)
	PruneMetrics(ctx context.Context, before time.Time) error
}

// QueryParams represents common query parameters
type QueryParams struct {
	AgentIDs  []string  `json:"agent_ids,omitempty"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Limit     int       `json:"limit,omitempty"`
	Offset    int       `json:"offset,omitempty"`
	OrderBy   string    `json:"order_by,omitempty"`
	Order     string    `json:"order,omitempty"`
}
