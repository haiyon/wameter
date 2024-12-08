-- Remove network_metrics column from metrics table
ALTER TABLE metrics DROP COLUMN network_metrics;

-- Drop idx_metrics_network index from metrics table
DROP INDEX IF EXISTS idx_metrics_network;

-- Drop idx_metrics_reported index from metrics table
DROP INDEX IF EXISTS idx_metrics_reported;

-- Drop idx_metrics_collected index from metrics table
DROP INDEX IF EXISTS idx_metrics_collected;
