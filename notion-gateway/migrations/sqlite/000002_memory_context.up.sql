PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

-- Durable, scope-isolated memory/context subsystem. No credential or raw secret column exists.
CREATE TABLE memory_conversations (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  project_id TEXT NOT NULL DEFAULT '',
  client_key_hash TEXT,
  last_response_id TEXT,
  created_at TEXT NOT NULL,
  last_activity_at TEXT NOT NULL,
  UNIQUE(user_id, client_key_hash)
);
CREATE TABLE memory_turns (
  id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL REFERENCES memory_conversations(id),
  model TEXT NOT NULL,
  response_id TEXT,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  completed_at TEXT,
  error_redacted TEXT
);
CREATE TABLE memory_messages (
  id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL REFERENCES memory_conversations(id),
  turn_id TEXT NOT NULL REFERENCES memory_turns(id),
  ordinal INTEGER NOT NULL,
  role TEXT NOT NULL,
  content_redacted TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(conversation_id, ordinal)
);
CREATE TABLE memory_summaries (
  conversation_id TEXT PRIMARY KEY REFERENCES memory_conversations(id),
  summary_redacted TEXT NOT NULL,
  covers_turn_count INTEGER NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE memory_records (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  project_id TEXT NOT NULL DEFAULT '',
  conversation_id TEXT NOT NULL DEFAULT '',
  type TEXT NOT NULL,
  subject_key TEXT NOT NULL,
  content_redacted TEXT NOT NULL,
  stable_hash TEXT NOT NULL,
  importance REAL NOT NULL,
  confidence REAL NOT NULL,
  state TEXT NOT NULL DEFAULT 'active',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(user_id, project_id, conversation_id, stable_hash)
);
CREATE INDEX idx_memory_records_scope ON memory_records(user_id, project_id, conversation_id, state, updated_at DESC);
CREATE VIRTUAL TABLE memory_records_fts USING fts5(memory_id UNINDEXED, user_id UNINDEXED, project_id UNINDEXED, conversation_id UNINDEXED, content);
