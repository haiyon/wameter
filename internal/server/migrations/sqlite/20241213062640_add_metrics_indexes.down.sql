-- Drop idx_metrics_network index from metrics table
DROP INDEX IF EXISTS idx_metrics_collected;

-- Drop idx_metrics_reported index from metrics table
DROP INDEX IF EXISTS idx_metrics_reported;

-- Drop idx_metrics_collected index from metrics table
DROP INDEX IF EXISTS idx_ip_changes_interface;
