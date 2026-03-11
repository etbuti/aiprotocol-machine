CREATE TABLE IF NOT EXISTS tokens (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  token TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  credits INTEGER NOT NULL DEFAULT 0,
  rate_limit_per_hour INTEGER NOT NULL DEFAULT 60,
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS jobs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  job_id TEXT NOT NULL UNIQUE,
  token_name TEXT NOT NULL,
  target TEXT NOT NULL,
  status TEXT NOT NULL,
  visa_id TEXT,
  result TEXT,
  credits_used INTEGER NOT NULL DEFAULT 0,
  evidence_url TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS ledger (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  token_name TEXT NOT NULL,
  delta INTEGER NOT NULL,
  reason TEXT NOT NULL,
  ref_id TEXT,
  created_at TEXT NOT NULL
);

INSERT INTO tokens (token, name, credits, created_at)
VALUES ('eb_agent_demo_20260311', 'agent-default', 390, datetime('now'));
