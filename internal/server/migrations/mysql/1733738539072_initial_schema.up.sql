-- agents table
CREATE TABLE IF NOT EXISTS agents (
  id VARCHAR(64) PRIMARY KEY,
  hostname VARCHAR(255) NOT NULL,
  version VARCHAR(32) NOT NULL,
  status VARCHAR(16) NOT NULL,
  last_seen DATETIME NOT NULL,
  registered_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  INDEX idx_agents_status (status),
  INDEX idx_agents_last_seen (last_seen)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- metrics table
CREATE TABLE IF NOT EXISTS metrics (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  agent_id VARCHAR(64) NOT NULL,
  timestamp DATETIME NOT NULL,
  collected_at DATETIME NOT NULL,
  reported_at DATETIME NOT NULL,
  data JSON NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_metrics_agent_time (agent_id, timestamp)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ip_changes table
CREATE TABLE IF NOT EXISTS ip_changes (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  agent_id VARCHAR(64) NOT NULL,
  interface_name VARCHAR(64),
  version VARCHAR(10) NOT NULL,
  is_external BOOLEAN NOT NULL,
  old_addrs JSON,
  new_addrs JSON,
  action VARCHAR(20) NOT NULL,
  reason VARCHAR(50) NOT NULL,
  timestamp DATETIME NOT NULL,
  created_at DATETIME NOT NULL,
  INDEX idx_agent_time (agent_id, timestamp),
  INDEX idx_interface (interface_name),
  INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
