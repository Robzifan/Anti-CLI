package memory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"errors"
	"fmt"
	"strings"
	"time"
)

// SQLiteStore implements the durable local-first store. The driver is supplied by main
// so the domain package remains independent of a particular SQLite implementation.
type SQLiteStore struct { db *sql.DB }

func OpenSQLite(ctx context.Context, driver, dsn string) (*SQLiteStore, error) {
	if strings.TrimSpace(driver) == "" { return nil, errors.New("sqlite driver is required") }
	if strings.TrimSpace(dsn) == "" { return nil, errors.New("sqlite data source is required") }
	if dsn != ":memory:" && !strings.HasPrefix(dsn, "file:") {
		dir := filepath.Dir(dsn)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o700); err != nil { return nil, fmt.Errorf("create sqlite directory: %w", err) }
		}
	}
	db, err := sql.Open(driver, dsn)
	if err != nil { return nil, err }
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil { _ = db.Close(); return nil, err }
	s := &SQLiteStore{db: db}
	if err := s.migrate(ctx); err != nil { _ = db.Close(); return nil, err }
	return s, nil
}
func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) migrate(ctx context.Context) error {
	statements := []string{
		"PRAGMA foreign_keys = ON", "PRAGMA journal_mode = WAL", "PRAGMA busy_timeout = 5000",
		`CREATE TABLE IF NOT EXISTS memory_conversations (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, project_id TEXT NOT NULL DEFAULT '', client_key_hash TEXT, last_response_id TEXT, created_at TEXT NOT NULL, last_activity_at TEXT NOT NULL, UNIQUE(user_id, client_key_hash))`,
		`CREATE TABLE IF NOT EXISTS memory_turns (id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES memory_conversations(id), model TEXT NOT NULL, response_id TEXT, status TEXT NOT NULL, created_at TEXT NOT NULL, completed_at TEXT, error_redacted TEXT)`,
		`CREATE TABLE IF NOT EXISTS memory_messages (id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES memory_conversations(id), turn_id TEXT NOT NULL REFERENCES memory_turns(id), ordinal INTEGER NOT NULL, role TEXT NOT NULL, content_redacted TEXT NOT NULL, created_at TEXT NOT NULL, UNIQUE(conversation_id, ordinal))`,
		`CREATE TABLE IF NOT EXISTS memory_summaries (conversation_id TEXT PRIMARY KEY REFERENCES memory_conversations(id), summary_redacted TEXT NOT NULL, covers_turn_count INTEGER NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS memory_records (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, project_id TEXT NOT NULL DEFAULT '', conversation_id TEXT NOT NULL DEFAULT '', type TEXT NOT NULL, subject_key TEXT NOT NULL, content_redacted TEXT NOT NULL, stable_hash TEXT NOT NULL, importance REAL NOT NULL, confidence REAL NOT NULL, state TEXT NOT NULL DEFAULT 'active', created_at TEXT NOT NULL, updated_at TEXT NOT NULL, UNIQUE(user_id, project_id, conversation_id, stable_hash))`,
		`CREATE INDEX IF NOT EXISTS idx_memory_records_scope ON memory_records(user_id, project_id, conversation_id, state, updated_at DESC)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS memory_records_fts USING fts5(memory_id UNINDEXED, user_id UNINDEXED, project_id UNINDEXED, conversation_id UNINDEXED, content)`,
		`CREATE TABLE IF NOT EXISTS memory_outbox (id TEXT PRIMARY KEY, event_type TEXT NOT NULL, payload_json TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'pending', attempts INTEGER NOT NULL DEFAULT 0, available_at TEXT NOT NULL, locked_at TEXT, completed_at TEXT, last_error_redacted TEXT, created_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_outbox_pending ON memory_outbox(status, available_at, created_at)`,
		`CREATE TABLE IF NOT EXISTS memory_tool_executions (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, project_id TEXT NOT NULL DEFAULT '', conversation_id TEXT NOT NULL DEFAULT '', turn_id TEXT, tool_name TEXT NOT NULL, tool_version TEXT, operation TEXT, payload_variant TEXT, input_fingerprint TEXT NOT NULL, input_summary_redacted TEXT NOT NULL DEFAULT '', output_summary_redacted TEXT NOT NULL DEFAULT '', status TEXT NOT NULL, error_category TEXT, error_summary_redacted TEXT, code_revision TEXT, artifact_refs_json TEXT NOT NULL DEFAULT '[]', idempotency_key TEXT, started_at TEXT NOT NULL, finished_at TEXT NOT NULL, created_at TEXT NOT NULL, UNIQUE(user_id, project_id, idempotency_key))`,
		`CREATE INDEX IF NOT EXISTS idx_memory_tool_scope ON memory_tool_executions(user_id, project_id, tool_name, status, finished_at DESC)`,
		`CREATE TABLE IF NOT EXISTS memory_approvals (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, project_id TEXT NOT NULL DEFAULT '', conversation_id TEXT NOT NULL DEFAULT '', server_name TEXT NOT NULL, tool_name TEXT NOT NULL, arguments_json TEXT NOT NULL, status TEXT NOT NULL, expires_at TEXT NOT NULL, created_at TEXT NOT NULL, resolved_at TEXT)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_approvals_pending ON memory_approvals(status, expires_at, created_at)`,
	}
	for _, statement := range statements { if _, err := s.db.ExecContext(ctx, statement); err != nil { return fmt.Errorf("memory migration: %w", err) } }
	return nil
}

func (s *SQLiteStore) ResolveConversation(ctx context.Context, scope Scope, explicit, previousResponseID, clientKey string) (string, error) {
	if explicit != "" {
		var id string
		err := s.db.QueryRowContext(ctx, `SELECT id FROM memory_conversations WHERE id=? AND user_id=? AND project_id=?`, explicit, scope.UserID, scope.ProjectID).Scan(&id)
		if err == nil { return id, nil }
		if !errors.Is(err, sql.ErrNoRows) { return "", err }
	}
	if previousResponseID != "" {
		var id string
		err := s.db.QueryRowContext(ctx, `SELECT conversation_id FROM memory_turns t JOIN memory_conversations c ON c.id=t.conversation_id WHERE t.response_id=? AND c.user_id=? AND c.project_id=?`, previousResponseID, scope.UserID, scope.ProjectID).Scan(&id)
		if err == nil { return id, nil }
		if !errors.Is(err, sql.ErrNoRows) { return "", err }
	}
	clientHash := hash(clientKey)
	if clientKey != "" {
		var id string
		err := s.db.QueryRowContext(ctx, `SELECT id FROM memory_conversations WHERE user_id=? AND client_key_hash=?`, scope.UserID, clientHash).Scan(&id)
		if err == nil { return id, nil }
		if !errors.Is(err, sql.ErrNoRows) { return "", err }
	}
	id, now := newID(), time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT INTO memory_conversations(id,user_id,project_id,client_key_hash,created_at,last_activity_at) VALUES(?,?,?,?,?,?)`, id, scope.UserID, scope.ProjectID, nullable(clientHash), now, now)
	return id, err
}

func (s *SQLiteStore) AppendUser(ctx context.Context, conversationID, content, model string) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil); if err != nil { return "", err }; defer tx.Rollback()
	turnID, now := newID(), time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, `INSERT INTO memory_turns(id,conversation_id,model,status,created_at) VALUES(?,?,?,?,?)`, turnID, conversationID, model, "streaming", now); err != nil { return "", err }
	var ordinal int
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(ordinal),0)+1 FROM memory_messages WHERE conversation_id=?`, conversationID).Scan(&ordinal); err != nil { return "", err }
	if _, err = tx.ExecContext(ctx, `INSERT INTO memory_messages(id,conversation_id,turn_id,ordinal,role,content_redacted,created_at) VALUES(?,?,?,?,?,?,?)`, newID(), conversationID, turnID, ordinal, "user", content, now); err != nil { return "", err }
	if _, err = tx.ExecContext(ctx, `UPDATE memory_conversations SET last_activity_at=? WHERE id=?`, now, conversationID); err != nil { return "", err }
	return turnID, tx.Commit()
}

func (s *SQLiteStore) CompleteTurn(ctx context.Context, turnID, assistant, responseID, status string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil { return err }
	defer tx.Rollback()
	var conversationID string
	if err = tx.QueryRowContext(ctx, `SELECT conversation_id FROM memory_turns WHERE id=?`, turnID).Scan(&conversationID); err != nil { return err }
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var ordinal int
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(ordinal),0)+1 FROM memory_messages WHERE conversation_id=?`, conversationID).Scan(&ordinal); err != nil { return err }
	if _, err = tx.ExecContext(ctx, `INSERT INTO memory_messages(id,conversation_id,turn_id,ordinal,role,content_redacted,created_at) VALUES(?,?,?,?,?,?,?)`, newID(), conversationID, turnID, ordinal, "assistant", assistant, now); err != nil { return err }
	if _, err = tx.ExecContext(ctx, `UPDATE memory_turns SET response_id=?,status=?,completed_at=? WHERE id=?`, nullable(responseID), status, now, turnID); err != nil { return err }
	if _, err = tx.ExecContext(ctx, `UPDATE memory_conversations SET last_response_id=?,last_activity_at=? WHERE id=?`, nullable(responseID), now, conversationID); err != nil { return err }
	payload, err := json.Marshal(map[string]string{"conversation_id": conversationID})
	if err != nil { return err }
	if _, err = tx.ExecContext(ctx, `INSERT INTO memory_outbox(id,event_type,payload_json,status,attempts,available_at,created_at) VALUES(?,?,?,?,?,?,?)`, newID(), "memory.consolidate", string(payload), "pending", 0, now, now); err != nil { return err }
	return tx.Commit()
}

func (s *SQLiteStore) AbortTurn(ctx context.Context, turnID, reason string) error { _, err := s.db.ExecContext(ctx, `UPDATE memory_turns SET status='failed', error_redacted=?, completed_at=? WHERE id=?`, reason, time.Now().UTC().Format(time.RFC3339Nano), turnID); return err }
func (s *SQLiteStore) LatestSummary(ctx context.Context, conversationID string) (string,error) { var v string; err:=s.db.QueryRowContext(ctx,`SELECT summary_redacted FROM memory_summaries WHERE conversation_id=?`,conversationID).Scan(&v); if errors.Is(err,sql.ErrNoRows){return "",nil}; return v,err }
func (s *SQLiteStore) RecentMessages(ctx context.Context, conversationID string, limit int) ([]Message,error) { rows,err:=s.db.QueryContext(ctx,`SELECT role,content_redacted,created_at FROM memory_messages WHERE conversation_id=? ORDER BY ordinal DESC LIMIT ?`,conversationID,limit); if err != nil{return nil,err}; defer rows.Close(); var reversed []Message; for rows.Next(){var m Message;var at string;if err:=rows.Scan(&m.Role,&m.Content,&at);err!=nil{return nil,err};m.CreatedAt,_=time.Parse(time.RFC3339Nano,at);reversed=append(reversed,m)};if err:=rows.Err();err!=nil{return nil,err}; for i,j:=0,len(reversed)-1;i<j;i,j=i+1,j-1{reversed[i],reversed[j]=reversed[j],reversed[i]};return reversed,nil }
func (s *SQLiteStore) SearchMemories(ctx context.Context, scope Scope, query string, limit int) ([]MemoryRecord,error) { terms:=ftsQuery(query);if terms==""{return nil,nil}; rows,err:=s.db.QueryContext(ctx,`SELECT r.id,r.conversation_id,r.type,r.subject_key,r.content_redacted,r.importance,r.confidence,r.created_at,r.updated_at FROM memory_records_fts f JOIN memory_records r ON r.id=f.memory_id WHERE memory_records_fts MATCH ? AND r.user_id=? AND r.project_id=? AND r.state='active' ORDER BY bm25(memory_records_fts),r.importance DESC LIMIT ?`,terms,scope.UserID,scope.ProjectID,limit);if err!=nil{return nil,err};defer rows.Close();var out []MemoryRecord;for rows.Next(){var r MemoryRecord;var created,updated string;if err:=rows.Scan(&r.ID,&r.ConversationID,&r.Type,&r.SubjectKey,&r.Content,&r.Importance,&r.Confidence,&created,&updated);err!=nil{return nil,err};r.Scope=scope;r.CreatedAt,_=time.Parse(time.RFC3339Nano,created);r.UpdatedAt,_=time.Parse(time.RFC3339Nano,updated);out=append(out,r)};return out,rows.Err() }
func (s *SQLiteStore) CountTurns(ctx context.Context, conversationID string)(int,error){var n int;err:=s.db.QueryRowContext(ctx,`SELECT COUNT(*) FROM memory_turns WHERE conversation_id=? AND status='completed'`,conversationID).Scan(&n);return n,err}
func (s *SQLiteStore) UpsertSummary(ctx context.Context, conversationID, summary string, count int)error{_,err:=s.db.ExecContext(ctx,`INSERT INTO memory_summaries(conversation_id,summary_redacted,covers_turn_count,updated_at) VALUES(?,?,?,?) ON CONFLICT(conversation_id) DO UPDATE SET summary_redacted=excluded.summary_redacted,covers_turn_count=excluded.covers_turn_count,updated_at=excluded.updated_at`,conversationID,summary,count,time.Now().UTC().Format(time.RFC3339Nano));return err}
func hash(v string)string{if v==""{return ""};h:=sha256.Sum256([]byte(v));return hex.EncodeToString(h[:])};func nullable(s string)any{if s==""{return nil};return s};func ftsQuery(q string)string{var words []string;for _,w:=range strings.Fields(q){w=strings.Trim(w,"\"'():,.;[]{}");if len([]rune(w))>=3{words=append(words,strings.ReplaceAll(w,"\"",""))}};if len(words)>8{words=words[:8]};return strings.Join(words," OR ")}


func (s *SQLiteStore) claimOutbox(ctx context.Context) (*outboxEvent, error) {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()

    now := time.Now().UTC()
    stale := now.Add(-5 * time.Minute).Format(time.RFC3339Nano)
    row := tx.QueryRowContext(ctx, `SELECT id,event_type,payload_json,attempts FROM memory_outbox WHERE (status='pending' AND available_at<=?) OR (status='processing' AND locked_at<?) ORDER BY created_at LIMIT 1`, now.Format(time.RFC3339Nano), stale)
    var event outboxEvent
    if err := row.Scan(&event.ID, &event.Type, &event.Payload, &event.Attempts); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, nil
        }
        return nil, err
    }
    if _, err := tx.ExecContext(ctx, `UPDATE memory_outbox SET status='processing',locked_at=?,attempts=attempts+1 WHERE id=?`, now.Format(time.RFC3339Nano), event.ID); err != nil {
        return nil, err
    }
    event.Attempts++
    if err := tx.Commit(); err != nil {
        return nil, err
    }
    return &event, nil
}

func (s *SQLiteStore) completeOutbox(ctx context.Context, eventID string) error {
    _, err := s.db.ExecContext(ctx, `UPDATE memory_outbox SET status='completed',completed_at=?,locked_at=NULL WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), eventID)
    return err
}

func (s *SQLiteStore) retryOutbox(ctx context.Context, eventID string, attempts int, diagnostic string) error {
    delay := time.Second * time.Duration(1<<min(attempts, 8))
    if delay > 5*time.Minute {
        delay = 5 * time.Minute
    }
    _, err := s.db.ExecContext(ctx, `UPDATE memory_outbox SET status='pending',available_at=?,locked_at=NULL,last_error_redacted=? WHERE id=?`, time.Now().UTC().Add(delay).Format(time.RFC3339Nano), diagnostic, eventID)
    return err
}

func (s *SQLiteStore) consolidateConversation(ctx context.Context, conversationID string) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    var userID, projectID string
    if err := tx.QueryRowContext(ctx, `SELECT user_id,project_id FROM memory_conversations WHERE id=?`, conversationID).Scan(&userID, &projectID); err != nil {
        return err
    }
    rows, err := tx.QueryContext(ctx, `SELECT content_redacted FROM memory_messages WHERE conversation_id=? AND role='assistant' ORDER BY ordinal`, conversationID)
    if err != nil {
        return err
    }
    var records []MemoryRecord
    for rows.Next() {
        var content string
        if err := rows.Scan(&content); err != nil {
            rows.Close()
            return err
        }
        records = append(records, extractCandidates(conversationID, content)...)
    }
    if err := rows.Close(); err != nil {
        return err
    }
    now := time.Now().UTC().Format(time.RFC3339Nano)
    for _, record := range records {
        stable := hash(record.Type + "\x00" + record.Content)
        if _, err := tx.ExecContext(ctx, `INSERT INTO memory_records(id,user_id,project_id,conversation_id,type,subject_key,content_redacted,stable_hash,importance,confidence,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(user_id,project_id,conversation_id,stable_hash) DO UPDATE SET importance=MAX(importance,excluded.importance),confidence=MAX(confidence,excluded.confidence),updated_at=excluded.updated_at`, record.ID, userID, projectID, conversationID, record.Type, record.SubjectKey, record.Content, stable, record.Importance, record.Confidence, now, now); err != nil {
            return err
        }
        var canonicalID string
        if err := tx.QueryRowContext(ctx, `SELECT id FROM memory_records WHERE user_id=? AND project_id=? AND conversation_id=? AND stable_hash=?`, userID, projectID, conversationID, stable).Scan(&canonicalID); err != nil {
            return err
        }
        if _, err := tx.ExecContext(ctx, `DELETE FROM memory_records_fts WHERE memory_id=?`, canonicalID); err != nil {
            return err
        }
        if _, err := tx.ExecContext(ctx, `INSERT INTO memory_records_fts(memory_id,user_id,project_id,conversation_id,content) VALUES(?,?,?,?,?)`, canonicalID, userID, projectID, conversationID, record.Content); err != nil {
            return err
        }
    }

    messages, err := messagesTx(ctx, tx, conversationID, 32)
    if err != nil {
        return err
    }
    if summary := deterministicSummary(messages); summary != "" {
        var turns int
        if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_turns WHERE conversation_id=? AND status='completed'`, conversationID).Scan(&turns); err != nil {
            return err
        }
        if _, err := tx.ExecContext(ctx, `INSERT INTO memory_summaries(conversation_id,summary_redacted,covers_turn_count,updated_at) VALUES(?,?,?,?) ON CONFLICT(conversation_id) DO UPDATE SET summary_redacted=excluded.summary_redacted,covers_turn_count=excluded.covers_turn_count,updated_at=excluded.updated_at`, conversationID, summary, turns, now); err != nil {
            return err
        }
    }
    return tx.Commit()
}

func messagesTx(ctx context.Context, tx *sql.Tx, conversationID string, limit int) ([]Message, error) {
    rows, err := tx.QueryContext(ctx, `SELECT role,content_redacted,created_at FROM memory_messages WHERE conversation_id=? ORDER BY ordinal DESC LIMIT ?`, conversationID, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var reversed []Message
    for rows.Next() {
        var message Message
        var created string
        if err := rows.Scan(&message.Role, &message.Content, &created); err != nil {
            return nil, err
        }
        message.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
        reversed = append(reversed, message)
    }
    if err := rows.Err(); err != nil {
        return nil, err
    }
    for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
        reversed[i], reversed[j] = reversed[j], reversed[i]
    }
    return reversed, nil
}

func (s *SQLiteStore) RecordToolExecution(ctx context.Context, execution ToolExecution) (ToolExecution, error) {
	if strings.TrimSpace(execution.Scope.UserID) == "" || strings.TrimSpace(execution.ToolName) == "" {
		return ToolExecution{}, errors.New("tool execution user and tool name are required")
	}
	if execution.Status == "" {
		execution.Status = "succeeded"
	}
	if execution.Status != "succeeded" && execution.Status != "failed" && execution.Status != "cancelled" {
		return ToolExecution{}, errors.New("invalid tool execution status")
	}
	now := time.Now().UTC()
	if execution.ID == "" { execution.ID = newID() }
	if execution.StartedAt.IsZero() { execution.StartedAt = now }
	if execution.FinishedAt.IsZero() { execution.FinishedAt = now }
	execution.InputSummary = Sanitize(execution.InputSummary)
	execution.OutputSummary = Sanitize(execution.OutputSummary)
	execution.ErrorSummary = Sanitize(execution.ErrorSummary)
	if execution.InputFingerprint == "" { execution.InputFingerprint = hash(execution.InputSummary) }
	if execution.ArtifactRefsJSON == "" { execution.ArtifactRefsJSON = "[]" }

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil { return ToolExecution{}, err }
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO memory_tool_executions(id,user_id,project_id,conversation_id,turn_id,tool_name,tool_version,operation,payload_variant,input_fingerprint,input_summary_redacted,output_summary_redacted,status,error_category,error_summary_redacted,code_revision,artifact_refs_json,idempotency_key,started_at,finished_at,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(user_id,project_id,idempotency_key) DO UPDATE SET output_summary_redacted=excluded.output_summary_redacted,status=excluded.status,error_category=excluded.error_category,error_summary_redacted=excluded.error_summary_redacted,finished_at=excluded.finished_at`, execution.ID, execution.Scope.UserID, execution.Scope.ProjectID, execution.ConversationID, nullable(execution.TurnID), execution.ToolName, nullable(execution.ToolVersion), nullable(execution.Operation), nullable(execution.PayloadVariant), execution.InputFingerprint, execution.InputSummary, execution.OutputSummary, execution.Status, nullable(execution.ErrorCategory), nullable(execution.ErrorSummary), nullable(execution.CodeRevision), execution.ArtifactRefsJSON, nullable(execution.IdempotencyKey), execution.StartedAt.Format(time.RFC3339Nano), execution.FinishedAt.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil { return ToolExecution{}, err }
	if execution.IdempotencyKey != "" {
		if err = tx.QueryRowContext(ctx, `SELECT id FROM memory_tool_executions WHERE user_id=? AND project_id=? AND idempotency_key=?`, execution.Scope.UserID, execution.Scope.ProjectID, execution.IdempotencyKey).Scan(&execution.ID); err != nil { return ToolExecution{}, err }
	}
	payload, err := json.Marshal(map[string]string{"execution_id": execution.ID})
	if err != nil { return ToolExecution{}, err }
	if _, err = tx.ExecContext(ctx, `INSERT INTO memory_outbox(id,event_type,payload_json,status,attempts,available_at,created_at) VALUES(?,?,?,?,?,?,?)`, newID(), "tool.consolidate", string(payload), "pending", 0, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil { return ToolExecution{}, err }
	if err = tx.Commit(); err != nil { return ToolExecution{}, err }
	return execution, nil
}

func (s *SQLiteStore) SearchToolExecutions(ctx context.Context, scope Scope, query string, limit int) ([]ToolExecution, error) {
	if limit <= 0 { return nil, nil }
	rows, err := s.db.QueryContext(ctx, `SELECT id,conversation_id,COALESCE(turn_id,''),tool_name,COALESCE(tool_version,''),COALESCE(operation,''),COALESCE(payload_variant,''),input_fingerprint,input_summary_redacted,output_summary_redacted,status,COALESCE(error_category,''),COALESCE(error_summary_redacted,''),COALESCE(code_revision,''),artifact_refs_json,COALESCE(idempotency_key,''),started_at,finished_at FROM memory_tool_executions WHERE user_id=? AND project_id=? ORDER BY finished_at DESC LIMIT 50`, scope.UserID, scope.ProjectID)
	if err != nil { return nil, err }
	defer rows.Close()
	words := strings.Fields(strings.ToLower(query))
	var result []ToolExecution
	for rows.Next() {
		var execution ToolExecution
		var started, finished string
		if err := rows.Scan(&execution.ID, &execution.ConversationID, &execution.TurnID, &execution.ToolName, &execution.ToolVersion, &execution.Operation, &execution.PayloadVariant, &execution.InputFingerprint, &execution.InputSummary, &execution.OutputSummary, &execution.Status, &execution.ErrorCategory, &execution.ErrorSummary, &execution.CodeRevision, &execution.ArtifactRefsJSON, &execution.IdempotencyKey, &started, &finished); err != nil { return nil, err }
		execution.Scope = scope
		execution.StartedAt, _ = time.Parse(time.RFC3339Nano, started)
		execution.FinishedAt, _ = time.Parse(time.RFC3339Nano, finished)
		haystack := strings.ToLower(strings.Join([]string{execution.ToolName, execution.Operation, execution.PayloadVariant, execution.InputSummary, execution.OutputSummary, execution.ErrorCategory, execution.ErrorSummary}, " "))
		matched := len(words) == 0
		for _, word := range words { if len([]rune(word)) >= 3 && strings.Contains(haystack, word) { matched = true; break } }
		if matched { result = append(result, execution) }
		if len(result) >= limit { break }
	}
	return result, rows.Err()
}

func (s *SQLiteStore) consolidateToolExecution(ctx context.Context, executionID string) error {
	var execution ToolExecution
	var started, finished string
	err := s.db.QueryRowContext(ctx, `SELECT id,user_id,project_id,conversation_id,COALESCE(turn_id,''),tool_name,COALESCE(tool_version,''),COALESCE(operation,''),COALESCE(payload_variant,''),input_fingerprint,input_summary_redacted,output_summary_redacted,status,COALESCE(error_category,''),COALESCE(error_summary_redacted,''),COALESCE(code_revision,''),artifact_refs_json,COALESCE(idempotency_key,''),started_at,finished_at FROM memory_tool_executions WHERE id=?`, executionID).Scan(&execution.ID, &execution.Scope.UserID, &execution.Scope.ProjectID, &execution.ConversationID, &execution.TurnID, &execution.ToolName, &execution.ToolVersion, &execution.Operation, &execution.PayloadVariant, &execution.InputFingerprint, &execution.InputSummary, &execution.OutputSummary, &execution.Status, &execution.ErrorCategory, &execution.ErrorSummary, &execution.CodeRevision, &execution.ArtifactRefsJSON, &execution.IdempotencyKey, &started, &finished)
	if err != nil { return err }
	execution.StartedAt, _ = time.Parse(time.RFC3339Nano, started)
	execution.FinishedAt, _ = time.Parse(time.RFC3339Nano, finished)
	detail := execution.OutputSummary
	if execution.Status != "succeeded" { detail = execution.ErrorSummary }
	content := fmt.Sprintf("Tool %s (%s) %s: %s", execution.ToolName, execution.PayloadVariant, execution.Status, detail)
	if len([]rune(strings.TrimSpace(detail))) < 4 { return nil }
	record := MemoryRecord{ID: newID(), Scope: execution.Scope, ConversationID: execution.ConversationID, Type: "tool_outcome", SubjectKey: "tool:" + execution.ToolName + ":" + execution.PayloadVariant, Content: Sanitize(content), Importance: 0.75, Confidence: 0.95}
	return s.upsertToolOutcome(ctx, record)
}

func (s *SQLiteStore) upsertToolOutcome(ctx context.Context, record MemoryRecord) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	stable := hash(record.Type + "\x00" + record.Content)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil { return err }
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO memory_records(id,user_id,project_id,conversation_id,type,subject_key,content_redacted,stable_hash,importance,confidence,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(user_id,project_id,conversation_id,stable_hash) DO UPDATE SET importance=MAX(importance,excluded.importance),confidence=MAX(confidence,excluded.confidence),updated_at=excluded.updated_at`, record.ID, record.Scope.UserID, record.Scope.ProjectID, record.ConversationID, record.Type, record.SubjectKey, record.Content, stable, record.Importance, record.Confidence, now, now); err != nil { return err }
	var canonicalID string
	if err = tx.QueryRowContext(ctx, `SELECT id FROM memory_records WHERE user_id=? AND project_id=? AND conversation_id=? AND stable_hash=?`, record.Scope.UserID, record.Scope.ProjectID, record.ConversationID, stable).Scan(&canonicalID); err != nil { return err }
	if _, err = tx.ExecContext(ctx, `DELETE FROM memory_records_fts WHERE memory_id=?`, canonicalID); err != nil { return err }
	if _, err = tx.ExecContext(ctx, `INSERT INTO memory_records_fts(memory_id,user_id,project_id,conversation_id,content) VALUES(?,?,?,?,?)`, canonicalID, record.Scope.UserID, record.Scope.ProjectID, record.ConversationID, record.Content); err != nil { return err }
	return tx.Commit()
}

func (s *SQLiteStore) CreateApproval(ctx context.Context, approval Approval) (Approval, error) {
    if approval.Scope.UserID == "" || approval.Server == "" || approval.Tool == "" { return Approval{}, errors.New("approval user, server, and tool are required") }
    now := time.Now().UTC()
    if approval.ID == "" { approval.ID = newID() }
    if approval.Status == "" { approval.Status = "pending" }
    if approval.ExpiresAt.IsZero() { approval.ExpiresAt = now.Add(10 * time.Minute) }
    approval.CreatedAt = now
    _, err := s.db.ExecContext(ctx, `INSERT INTO memory_approvals(id,user_id,project_id,conversation_id,server_name,tool_name,arguments_json,status,expires_at,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, approval.ID, approval.Scope.UserID, approval.Scope.ProjectID, approval.ConversationID, approval.Server, approval.Tool, approval.ArgumentsJSON, approval.Status, approval.ExpiresAt.Format(time.RFC3339Nano), approval.CreatedAt.Format(time.RFC3339Nano))
    return approval, err
}

func (s *SQLiteStore) ResolveApproval(ctx context.Context, id, action string) (Approval, error) {
    if action != "approved" && action != "denied" { return Approval{}, errors.New("approval action must be approved or denied") }
    tx, err := s.db.BeginTx(ctx, nil); if err != nil { return Approval{}, err }; defer tx.Rollback()
    var approval Approval; var expires, created string; var resolved sql.NullString
    err = tx.QueryRowContext(ctx, `SELECT id,user_id,project_id,conversation_id,server_name,tool_name,arguments_json,status,expires_at,created_at,resolved_at FROM memory_approvals WHERE id=?`, id).Scan(&approval.ID, &approval.Scope.UserID, &approval.Scope.ProjectID, &approval.ConversationID, &approval.Server, &approval.Tool, &approval.ArgumentsJSON, &approval.Status, &expires, &created, &resolved)
    if err != nil { return Approval{}, err }
    approval.ExpiresAt, _ = time.Parse(time.RFC3339Nano, expires); approval.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
    now := time.Now().UTC()
    if approval.Status != "pending" { return Approval{}, errors.New("approval is not pending") }
    if now.After(approval.ExpiresAt) {
        if _, err = tx.ExecContext(ctx, `UPDATE memory_approvals SET status='expired',resolved_at=? WHERE id=?`, now.Format(time.RFC3339Nano), id); err != nil { return Approval{}, err }
        if err = tx.Commit(); err != nil { return Approval{}, err }; approval.Status="expired"; approval.ResolvedAt=now; return approval, errors.New("approval expired")
    }
    if _, err = tx.ExecContext(ctx, `UPDATE memory_approvals SET status=?,resolved_at=? WHERE id=?`, action, now.Format(time.RFC3339Nano), id); err != nil { return Approval{}, err }
    if err = tx.Commit(); err != nil { return Approval{}, err }; approval.Status=action; approval.ResolvedAt=now; return approval, nil
}
