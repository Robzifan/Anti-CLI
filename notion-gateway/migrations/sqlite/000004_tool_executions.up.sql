PRAGMA foreign_keys = ON;
CREATE TABLE memory_tool_executions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  project_id TEXT NOT NULL DEFAULT '',
  conversation_id TEXT NOT NULL DEFAULT '',
  turn_id TEXT,
  tool_name TEXT NOT NULL,
  tool_version TEXT,
  operation TEXT,
  payload_variant TEXT,
  input_fingerprint TEXT NOT NULL,
  input_summary_redacted TEXT NOT NULL DEFAULT '',
  output_summary_redacted TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  error_category TEXT,
  error_summary_redacted TEXT,
  code_revision TEXT,
  artifact_refs_json TEXT NOT NULL DEFAULT '[]',
  idempotency_key TEXT,
  started_at TEXT NOT NULL,
  finished_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(user_id, project_id, idempotency_key)
);
CREATE INDEX idx_memory_tool_scope ON memory_tool_executions(user_id, project_id, tool_name, status, finished_at DESC);
