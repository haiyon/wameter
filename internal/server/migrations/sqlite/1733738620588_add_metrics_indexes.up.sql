CREATE INDEX IF NOT EXISTS idx_metrics_collected ON metrics(collected_at);
CREATE INDEX IF NOT EXISTS idx_metrics_reported ON metrics(reported_at);
CREATE INDEX IF NOT EXISTS idx_metrics_network ON metrics((json_extract(data, '$.metrics.network')));
