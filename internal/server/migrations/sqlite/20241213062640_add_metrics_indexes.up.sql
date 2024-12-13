-- Add idx_metrics_collected to metrics table
CREATE INDEX IF NOT EXISTS idx_metrics_collected ON metrics (collected_at);

-- Add idx_metrics_reported to metrics table
CREATE INDEX IF NOT EXISTS idx_metrics_reported ON metrics (reported_at);

-- Add idx_metrics_network index to metrics table
CREATE INDEX IF NOT EXISTS idx_metrics_network ON metrics ((json_extract(data, '$.metrics.network')));
