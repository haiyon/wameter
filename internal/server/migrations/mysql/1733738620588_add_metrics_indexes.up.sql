ALTER TABLE metrics
  ADD INDEX idx_metrics_collected (collected_at);
ALTER TABLE metrics
  ADD INDEX idx_metrics_reported (reported_at);
ALTER TABLE metrics
  ADD COLUMN network_metrics VARCHAR(255) AS (JSON_UNQUOTE(JSON_EXTRACT(data, '$.metrics.network'))) VIRTUAL;
CREATE INDEX idx_metrics_network ON metrics (network_metrics);
