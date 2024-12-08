CREATE TABLE IF NOT EXISTS agents (
  id VARCHAR(64) PRIMARY KEY,
  hostname VARCHAR(255) NOT NULL,
  version VARCHAR(32) NOT NULL,
  status VARCHAR(16) NOT NULL,
  last_seen TIMESTAMP NOT NULL,
  registered_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_agents_last_seen ON agents(last_seen);

CREATE TABLE IF NOT EXISTS metrics (
  id BIGSERIAL PRIMARY KEY,
  agent_id VARCHAR(64) NOT NULL,
  timestamp TIMESTAMP NOT NULL,
  collected_at TIMESTAMP NOT NULL,
  reported_at TIMESTAMP NOT NULL,
  data JSONB NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_metrics_agent_time ON metrics(agent_id, timestamp);

CREATE TABLE IF NOT EXISTS ip_changes (
  id BIGSERIAL PRIMARY KEY,
  agent_id VARCHAR(64) NOT NULL,
  interface_name VARCHAR(64),
  version VARCHAR(10) NOT NULL,
  is_external BOOLEAN NOT NULL,
  old_addrs JSONB,
  new_addrs JSONB,
  action VARCHAR(20) NOT NULL,
  reason VARCHAR(50) NOT NULL,
  timestamp TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_ip_changes_agent_time ON ip_changes(agent_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_ip_changes_interface ON ip_changes(interface_name);
CREATE INDEX IF NOT EXISTS idx_ip_changes_created_at ON ip_changes(created_at);
