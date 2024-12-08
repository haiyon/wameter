CREATE TABLE IF NOT EXISTS agents (
  id TEXT PRIMARY KEY,
  hostname TEXT NOT NULL,
  version TEXT NOT NULL,
  status TEXT NOT NULL,
  last_seen DATETIME NOT NULL,
  registered_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_agents_last_seen ON agents(last_seen);

CREATE TABLE IF NOT EXISTS metrics (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  agent_id TEXT NOT NULL,
  timestamp DATETIME NOT NULL,
  collected_at DATETIME NOT NULL,
  reported_at DATETIME NOT NULL,
  data JSON NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (agent_id) REFERENCES agents(id)
);

CREATE INDEX IF NOT EXISTS idx_metrics_agent_time ON metrics(agent_id, timestamp);

CREATE TABLE IF NOT EXISTS ip_changes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  agent_id TEXT NOT NULL,
  interface_name TEXT,
  version TEXT NOT NULL,
  is_external INTEGER NOT NULL,
  old_addrs TEXT,
  new_addrs TEXT,
  action TEXT NOT NULL,
  reason TEXT NOT NULL,
  timestamp DATETIME NOT NULL,
  created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_ip_changes_agent_time ON ip_changes(agent_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_ip_changes_interface ON ip_changes(interface_name);
CREATE INDEX IF NOT EXISTS idx_ip_changes_created_at ON ip_changes(created_at);
