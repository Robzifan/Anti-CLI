PRAGMA foreign_keys = ON;
CREATE TABLE memory_outbox (
  id TEXT PRIMARY KEY,
  event_type TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  attempts INTEGER NOT NULL DEFAULT 0,
  available_at TEXT NOT NULL,
  locked_at TEXT,
  completed_at TEXT,
  last_error_redacted TEXT,
  created_at TEXT NOT NULL
);
CREATE INDEX idx_memory_outbox_pending ON memory_outbox(status, available_at, created_at);
