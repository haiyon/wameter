package service

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"
	"wameter/internal/server/repository"
	"wameter/internal/types"

	"go.uber.org/zap"
)

// MetricsService represents metrics service interface
type MetricsService interface {
	SaveMetrics(ctx context.Context, data *types.MetricsData) error
	BatchSave(ctx context.Context, metrics []*types.MetricsData) error
	GetMetrics(ctx context.Context, query MetricsQuery) ([]*types.MetricsData, error)
	GetLatestMetrics(ctx context.Context, agentID string) (*types.MetricsData, error)
	GetMetricsSummary(ctx context.Context, agentID string) (*types.MetricsSummary, error)
	ExportMetrics(ctx context.Context, format string, filter types.MetricsFilter) (io.Reader, error)
	ArchiveMetrics(ctx context.Context, opts types.MetricsArchiveOptions) error
	DeleteMetrics(ctx context.Context, before time.Time) error
}

// _ implements MetricsService
var _ MetricsService = (*Service)(nil)

// MetricsQuery represents a query for metrics
type MetricsQuery struct {
	AgentIDs  []string  `json:"agent_ids,omitempty"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Limit     int       `json:"limit,omitempty"`
}

// SaveMetrics saves metrics data
func (s *Service) SaveMetrics(ctx context.Context, data *types.MetricsData) error {
	// Update agent status
	if err := s.UpdateAgentStatus(ctx, data.AgentID, types.AgentStatusOnline); err != nil {
		s.logger.Error("Failed to update agent status",
			zap.Error(err),
			zap.String("agent_id", data.AgentID))
	}

	// Save metrics
	if err := s.metricsRepo.Save(ctx, data); err != nil {
		return fmt.Errorf("failed to save metrics: %w", err)
	}

	if data.Metrics.Network != nil {
		s.processNetworkMetrics(ctx, data)
	}

	s.recordMetric(func(m *types.ServiceMetrics) {
		m.MetricsProcessed++
	})

	// Process metrics for notifications
	go s.processMetricsAlerts(data)

	return nil
}

// BatchSave saves multiple metrics entries
func (s *Service) BatchSave(ctx context.Context, metrics []*types.MetricsData) error {
	// First validate all metrics
	for _, m := range metrics {
		if m.AgentID == "" || m.Timestamp.IsZero() {
			return fmt.Errorf("invalid metrics data: missing required fields")
		}
	}

	// Save metrics in transaction
	if err := s.metricsRepo.BatchSave(ctx, metrics); err != nil {
		return fmt.Errorf("failed to save metrics batch: %w", err)
	}

	// Process metrics in background
	go func() {
		for _, m := range metrics {
			s.processMetricsAlerts(m)
		}
	}()

	return nil
}

// GetMetrics retrieves metrics based on query parameters
func (s *Service) GetMetrics(ctx context.Context, query MetricsQuery) ([]*types.MetricsData, error) {
	// Validate time range
	if query.StartTime.After(query.EndTime) {
		return nil, fmt.Errorf("start time must be before end time")
	}

	// Set reasonable limits
	if query.Limit <= 0 {
		query.Limit = 1000
	} else if query.Limit > 10000 {
		query.Limit = 10000
	}

	return s.metricsRepo.Query(ctx, repository.QueryParams{
		AgentIDs:  query.AgentIDs,
		StartTime: query.StartTime,
		EndTime:   query.EndTime,
		Limit:     query.Limit,
	})
}

// GetLatestMetrics returns the latest metrics for an agent
func (s *Service) GetLatestMetrics(ctx context.Context, agentID string) (*types.MetricsData, error) {
	// Verify agent exists
	agent, err := s.agentRepo.FindByID(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to find agent: %w", err)
	}

	// Get latest metrics
	metrics, err := s.metricsRepo.GetLatest(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest metrics: %w", err)
	}

	// Update with current agent status
	if metrics.Metrics.Network != nil {
		metrics.Metrics.Network.AgentID = agent.ID
		metrics.Metrics.Network.Hostname = agent.Hostname
	}

	return metrics, nil
}

// ExportMetrics exports metrics in specified format
func (s *Service) ExportMetrics(ctx context.Context, format string, filter types.MetricsFilter) (io.Reader, error) {
	// Get metrics based on filter
	metrics, err := s.metricsRepo.Query(ctx, repository.QueryParams{
		AgentIDs:  filter.AgentIDs,
		StartTime: filter.StartTime,
		EndTime:   filter.EndTime,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}

	switch format {
	case "json":
		return s.exportMetricsJSON(metrics)
	case "csv":
		return s.exportMetricsCSV(metrics)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// exportMetricsJSON exports metrics as JSON
func (s *Service) exportMetricsJSON(metrics []*types.MetricsData) (io.Reader, error) {
	pr, pw := io.Pipe()

	go func() {
		encoder := json.NewEncoder(pw)
		err := encoder.Encode(metrics)
		_ = pw.CloseWithError(err)
	}()

	return pr, nil
}

// exportMetricsCSV exports metrics as CSV
func (s *Service) exportMetricsCSV(metrics []*types.MetricsData) (io.Reader, error) {
	pr, pw := io.Pipe()

	go func() {
		writer := csv.NewWriter(pw)
		defer writer.Flush()

		// Write header
		header := []string{
			"AgentID",
			"Timestamp",
			"CollectedAt",
			"ReportedAt",
			"MetricType",
			"Value",
		}
		if err := writer.Write(header); err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		// Write metrics data
		for _, m := range metrics {
			// Write network metrics
			if m.Metrics.Network != nil {
				for name, iface := range m.Metrics.Network.Interfaces {
					row := []string{
						m.AgentID,
						m.Timestamp.Format(time.RFC3339),
						m.CollectedAt.Format(time.RFC3339),
						m.ReportedAt.Format(time.RFC3339),
						"network_interface",
						fmt.Sprintf("%s:%s", name, iface.Status),
					}
					if err := writer.Write(row); err != nil {
						_ = pw.CloseWithError(err)
						return
					}
				}
			}
		}
		_ = pw.Close()
	}()

	return pr, nil
}

// GetMetricsSummary returns a metrics summary for an agent
func (s *Service) GetMetricsSummary(ctx context.Context, agentID string) (*types.MetricsSummary, error) {
	// Verify agent exists
	if _, err := s.agentRepo.FindByID(ctx, agentID); err != nil {
		return nil, fmt.Errorf("failed to find agent: %w", err)
	}

	// Get metrics summary from repository
	summary, err := s.metricsRepo.GetMetricsSummary(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics summary: %w", err)
	}

	// Get current agent status
	s.agentsMu.RLock()
	if agent, exists := s.agents[agentID]; exists {
		summary.LastSeen = agent.LastSeen
		summary.CurrentStatus = string(agent.Status)
	}
	s.agentsMu.RUnlock()

	return summary, nil
}

// ArchiveMetrics archives old metrics
func (s *Service) ArchiveMetrics(ctx context.Context, opts types.MetricsArchiveOptions) error {
	// Get metrics to archive
	metrics, err := s.metricsRepo.Query(ctx, repository.QueryParams{
		EndTime: opts.Before,
	})
	if err != nil {
		return fmt.Errorf("failed to get metrics for archival: %w", err)
	}

	// Archive metrics based on storage type
	switch opts.StorageType {
	case "s3":
		if err := s.archiveToS3(ctx, metrics, opts); err != nil {
			return fmt.Errorf("failed to archive to S3: %w", err)
		}
	case "file":
		if err := s.archiveToFile(ctx, metrics, opts); err != nil {
			return fmt.Errorf("failed to archive to file: %w", err)
		}
	default:
		return fmt.Errorf("unsupported storage type: %s", opts.StorageType)
	}

	// Delete archived metrics if requested
	if opts.DeleteAfter {
		if err := s.DeleteMetrics(ctx, opts.Before); err != nil {
			return fmt.Errorf("failed to delete archived metrics: %w", err)
		}
	}

	return nil
}

// archiveToS3 archives metrics to S3
func (s *Service) archiveToS3(ctx context.Context, metrics []*types.MetricsData, opts types.MetricsArchiveOptions) error {
	if len(metrics) == 0 {
		return nil
	}

	// Prepare archive file
	archiveData, err := s.prepareArchiveData(metrics, opts.Compress)
	if err != nil {
		return fmt.Errorf("failed to prepare archive data: %w", err)
	}

	// Generate archive key
	timeStr := time.Now().Format("2006-01-02")
	archiveKey := fmt.Sprintf("metrics/%s/metrics-%s.json", timeStr,
		opts.Before.Format("2006-01-02"))
	if opts.Compress {
		archiveKey += ".gz"
	}

	// Upload to S3
	if err := s.uploadToS3(ctx, archiveKey, archiveData); err != nil {
		return fmt.Errorf("failed to upload archive to S3: %w", err)
	}

	s.logger.Info("Archived metrics to S3",
		zap.Int("metrics_count", len(metrics)),
		zap.String("archive_key", archiveKey))

	return nil
}

// archiveToFile archives metrics to local file
func (s *Service) archiveToFile(_ context.Context, metrics []*types.MetricsData, opts types.MetricsArchiveOptions) error {
	if len(metrics) == 0 {
		return nil
	}

	// Prepare archive data
	archiveData, err := s.prepareArchiveData(metrics, opts.Compress)
	if err != nil {
		return fmt.Errorf("failed to prepare archive data: %w", err)
	}

	// Generate archive path
	timeStr := time.Now().Format("2006-01-02")
	archivePath := fmt.Sprintf("/var/lib/wameter/archives/metrics-%s.json", timeStr)
	if opts.Compress {
		archivePath += ".gz"
	}

	// Write to file
	if err := s.writeArchiveFile(archivePath, archiveData); err != nil {
		return fmt.Errorf("failed to write archive file: %w", err)
	}

	s.logger.Info("Archived metrics to file",
		zap.Int("metrics_count", len(metrics)),
		zap.String("archive_path", archivePath))

	return nil
}

// prepareArchiveData prepares metrics data for archiving
func (s *Service) prepareArchiveData(metrics []*types.MetricsData, compress bool) ([]byte, error) {
	// Marshal metrics to JSON
	data, err := json.Marshal(metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metrics: %w", err)
	}

	// Compress if requested
	if compress {
		compressed, err := s.compressData(data)
		if err != nil {
			return nil, fmt.Errorf("failed to compress data: %w", err)
		}
		return compressed, nil
	}

	return data, nil
}

// uploadToS3 uploads data to S3
func (s *Service) uploadToS3(ctx context.Context, key string, data []byte) error {
	// TODO: Implement S3 upload
	return fmt.Errorf("S3 upload not implemented")
}

// writeArchiveFile writes archive data to file
func (s *Service) writeArchiveFile(path string, data []byte) error {
	// TODO: Implement file writing with proper permissions and error handling
	return fmt.Errorf("file archive not implemented")
}

// compressData compresses byte data
func (s *Service) compressData(data []byte) ([]byte, error) {
	// TODO: Implement data compression
	return nil, fmt.Errorf("data compression not implemented")
}

// DeleteMetrics deletes metrics before specified time
func (s *Service) DeleteMetrics(ctx context.Context, before time.Time) error {
	return s.metricsRepo.DeleteBefore(ctx, before)
}

// // archiveMetricsData handles actual archiving of metrics data
// func (s *Service) archiveMetricsData(metrics []*types.MetricsData) error {
// 	// Implement archiving strategy (e.g., to S3, local file, etc.)
// 	return fmt.Errorf("metrics archiving not implemented")
// }

// processNetworkMetrics processes network metrics
func (s *Service) processNetworkMetrics(ctx context.Context, data *types.MetricsData) {
	network := data.Metrics.Network

	// Handle IP changes
	if len(network.IPChanges) > 0 {
		for _, change := range network.IPChanges {
			if err := s.ipChangeRepo.Save(ctx, data.AgentID, &change); err != nil {
				s.logger.Error("Failed to save IP change",
					zap.Error(err),
					zap.String("agent_id", data.AgentID),
					zap.String("interface", change.InterfaceName))
				continue
			}

			// Send notification
			if s.notifier != nil {
				agent := &types.AgentInfo{
					ID:       data.AgentID,
					Hostname: data.Hostname,
					Status:   types.AgentStatusOnline,
				}
				s.notifier.NotifyIPChange(agent, &change)
			}
		}
	}

	// Check interface statistics
	for _, iface := range network.Interfaces {
		if iface.Statistics == nil {
			continue
		}

		// Error rates
		totalErrors := iface.Statistics.RxErrors + iface.Statistics.TxErrors
		if totalErrors > 100 && s.notifier != nil {
			s.notifier.NotifyNetworkErrors(data.AgentID, iface)
		}

		// High utilization
		if (iface.Statistics.RxBytesRate+iface.Statistics.TxBytesRate) > 100*1024*1024 && s.notifier != nil {
			s.notifier.NotifyHighNetworkUtilization(data.AgentID, iface)
		}
	}
}

// processMetricsAlerts processes metrics for alerts
func (s *Service) processMetricsAlerts(data *types.MetricsData) {
	if data.Metrics.Network == nil {
		return
	}
	// Process network metrics
	for _, iface := range data.Metrics.Network.Interfaces {
		if iface.Statistics == nil {
			continue
		}

		// Check for high error rates
		totalErrors := iface.Statistics.RxErrors + iface.Statistics.TxErrors
		if totalErrors > 100 {
			s.notifier.NotifyNetworkErrors(data.AgentID, iface)
		}

		// Check for high utilization
		if iface.Statistics.RxBytesRate > 100*1024*1024 || // 100 MB/s
			iface.Statistics.TxBytesRate > 100*1024*1024 {
			s.notifier.NotifyHighNetworkUtilization(data.AgentID, iface)
		}
	}
}
