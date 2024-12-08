-- Remove network_metrics column from metrics table
ALTER TABLE metrics DROP COLUMN network_metrics;

-- Drop idx_metrics_network index from metrics table
DROP INDEX idx_metrics_network ON metrics;

-- Drop idx_metrics_reported index from metrics table
DROP INDEX idx_metrics_reported ON metrics;

-- Drop idx_metrics_collected index from metrics table
DROP INDEX idx_metrics_collected ON metrics;
