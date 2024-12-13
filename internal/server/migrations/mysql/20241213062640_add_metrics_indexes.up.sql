-- Add idx_metrics_collected to metrics table
ALTER TABLE metrics ADD INDEX idx_metrics_collected (collected_at);

-- Add idx_metrics_reported to metrics table
ALTER TABLE metrics ADD INDEX idx_metrics_reported (reported_at);

-- Add idx_metrics_network index to metrics table
ALTER TABLE metrics ADD INDEX idx_metrics_network ((CAST(data->>'$.metrics.network' AS CHAR(255))));
