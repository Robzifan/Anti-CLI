package api

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net"
	"strings"
	"time"

	"github.com/notioncodex/gateway/internal/api/middleware"
	"github.com/notioncodex/gateway/internal/domain"
	"github.com/notioncodex/gateway/internal/models"
	"github.com/notioncodex/gateway/internal/memory"
	"github.com/notioncodex/gateway/internal/mcp"
	"github.com/notioncodex/gateway/internal/ports"
)

type Server struct {
	logger         *slog.Logger
	registry       *models.Registry
	transport      ports.InferenceTransport
	memory         *memory.Engine
	mcp            *mcp.Manager
	mcpAPIKey      string
	started        time.Time
	requestTimeout time.Duration
}

func New(logger *slog.Logger, r *models.Registry, t ports.InferenceTransport, timeout time.Duration, engines ...*memory.Engine) http.Handler {
	var engine *memory.Engine
	if len(engines) > 0 { engine = engines[0] }
	return NewWithMCPAndKey(logger, r, t, timeout, engine, nil, "")
}

func NewWithMCP(logger *slog.Logger, r *models.Registry, t ports.InferenceTransport, timeout time.Duration, engine *memory.Engine, manager *mcp.Manager) http.Handler {
	return NewWithMCPAndKey(logger, r, t, timeout, engine, manager, "")
}

func NewWithMCPAndKey(logger *slog.Logger, r *models.Registry, t ports.InferenceTransport, timeout time.Duration, engine *memory.Engine, manager *mcp.Manager, apiKey string) http.Handler {
	s := &Server{logger: logger, registry: r, transport: t, memory: engine, mcp: manager, mcpAPIKey: apiKey, started: time.Now(), requestTimeout: timeout}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /v1/models", s.listModels)
	mux.HandleFunc("GET /v1/mcp/tools", s.listMCPTools)
	mux.HandleFunc("POST /v1/mcp/call", s.callMCPTool)
	mux.HandleFunc("POST /v1/mcp/approvals", s.resolveMCPApproval)
	mux.HandleFunc("POST /v1/responses", s.responses)
	return middleware.RequestID(s.log(mux))
}
func (s *Server) log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("http_request", "request_id", middleware.ID(r.Context()), "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "uptime_seconds": int(time.Since(s.started).Seconds())})
}
func (s *Server) ready(w http.ResponseWriter, _ *http.Request) {
	if !s.registry.Ready() {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "model registry is empty", "")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
type mcpCallRequest struct {
	Server       string          `json:"server"`
	Name         string          `json:"name"`
	Arguments    json.RawMessage `json:"arguments"`
	Conversation string          `json:"conversation"`
	Metadata     map[string]string `json:"metadata"`
}

func (s *Server) callMCPTool(w http.ResponseWriter, r *http.Request) {
	if !s.requireMCPAuth(w, r) { return }
	if s.mcp == nil {
		writeError(w, http.StatusNotFound, "not_found", "MCP is not configured", middleware.ID(r.Context()))
		return
	}
	if !isLoopbackRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden", "MCP call endpoint is loopback-only", middleware.ID(r.Context()))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req mcpCallRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil || req.Server == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "server and name are required", middleware.ID(r.Context()))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if s.mcp.RequiresApproval(req.Server, req.Name) {
		if s.memory == nil { writeError(w, http.StatusServiceUnavailable, "approval_unavailable", "memory is required for approvals", middleware.ID(r.Context())); return }
		projectID := ""; if req.Metadata != nil { projectID = req.Metadata["ncg_project_id"] }
		approval, approvalErr := s.memory.CreateApproval(ctx, memory.Approval{Scope:memory.Scope{UserID:"local",ProjectID:projectID}, ConversationID:req.Conversation, Server:req.Server, Tool:req.Name, ArgumentsJSON:string(req.Arguments)})
		if approvalErr != nil { writeError(w, http.StatusBadRequest, "approval_error", safeErr(approvalErr), middleware.ID(r.Context())); return }
		writeJSON(w, http.StatusAccepted, map[string]any{"id":approval.ID,"status":approval.Status,"expires_at":approval.ExpiresAt})
		return
	}
	result, err := s.mcp.Call(ctx, req.Server, req.Name, req.Arguments)
	if err != nil {
		writeError(w, http.StatusBadGateway, "mcp_error", safeErr(err), middleware.ID(r.Context()))
		return
	}
	if s.memory != nil {
		projectID := ""
		if req.Metadata != nil { projectID = req.Metadata["ncg_project_id"] }
		status := "succeeded"
		if result.IsError { status = "failed" }
		_, recordErr := s.memory.RecordToolExecution(ctx, memory.ToolExecution{
			Scope: memory.Scope{UserID:"local", ProjectID:projectID}, ConversationID:req.Conversation,
			ToolName: result.Server+"."+result.Name, Operation:"tools/call", Status:status,
			OutputSummary: truncateMCPContent(result.Content), IdempotencyKey: middleware.ID(r.Context()),
		})
		if recordErr != nil { s.logger.Warn("memory_tool_record_failed", "request_id", middleware.ID(r.Context()), "error", safeErr(recordErr)) }
	}
	writeJSON(w, http.StatusOK, result)
}

type mcpApprovalRequest struct { ID string `json:"id"`; Action string `json:"action"` }

func (s *Server) resolveMCPApproval(w http.ResponseWriter, r *http.Request) {
	if !s.requireMCPAuth(w, r) { return }
	if s.mcp == nil || s.memory == nil { writeError(w, http.StatusServiceUnavailable, "approval_unavailable", "MCP and memory are required", middleware.ID(r.Context())); return }
	if !isLoopbackRequest(r) { writeError(w, http.StatusForbidden, "forbidden", "MCP approval endpoint is loopback-only", middleware.ID(r.Context())); return }
	var req mcpApprovalRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil || req.ID == "" { writeError(w, http.StatusBadRequest, "invalid_request_error", "approval id is required", middleware.ID(r.Context())); return }
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second); defer cancel()
	approval, err := s.memory.ResolveApproval(ctx, req.ID, req.Action)
	if err != nil { writeError(w, http.StatusConflict, "approval_error", safeErr(err), middleware.ID(r.Context())); return }
	if approval.Status == "denied" { writeJSON(w, http.StatusOK, approval); return }
	result, err := s.mcp.Call(ctx, approval.Server, approval.Tool, json.RawMessage(approval.ArgumentsJSON))
	if err != nil { writeError(w, http.StatusBadGateway, "mcp_error", safeErr(err), middleware.ID(r.Context())); return }
	status := "succeeded"; if result.IsError { status="failed" }
	_, _ = s.memory.RecordToolExecution(ctx, memory.ToolExecution{Scope:approval.Scope,ConversationID:approval.ConversationID,ToolName:result.Server+"."+result.Name,Operation:"tools/call",Status:status,OutputSummary:truncateMCPContent(result.Content),IdempotencyKey:approval.ID})
	writeJSON(w, http.StatusOK, map[string]any{"approval":approval,"result":result})
}

func (s *Server) requireMCPAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.mcpAPIKey == "" { return true }
	provided := r.Header.Get("X-NotionCodex-API-Key")
	if provided == "" { provided = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") }
	if subtle.ConstantTimeCompare([]byte(provided), []byte(s.mcpAPIKey)) == 1 { return true }
	writeError(w, http.StatusUnauthorized, "authentication_error", "valid MCP API key is required", middleware.ID(r.Context()))
	return false
}

func isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil { host = r.RemoteAddr }
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func truncateMCPContent(content json.RawMessage) string {
	value := strings.TrimSpace(string(content))
	if len(value) > 4000 { return value[:4000] + "…" }
	return value
}

func (s *Server) listMCPTools(w http.ResponseWriter, r *http.Request) {
	if !s.requireMCPAuth(w, r) { return }
	if s.mcp == nil {
		writeError(w, http.StatusNotFound, "not_found", "MCP is not configured", middleware.ID(r.Context()))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	tools, err := s.mcp.Tools(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "mcp_error", safeErr(err), middleware.ID(r.Context()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"object":"list", "data":tools})
}

func (s *Server) listModels(w http.ResponseWriter, _ *http.Request) {
	items := make([]map[string]any, 0)
	for _, m := range s.registry.List() {
		items = append(items, map[string]any{"id": m.PublicID, "object": "model", "owned_by": "notioncodex-gateway"})
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": items})
}

type responseRequest struct {
	Model              string            `json:"model"`
	Input              json.RawMessage   `json:"input"`
	Stream             bool              `json:"stream"`
	PreviousResponseID string            `json:"previous_response_id"`
	Conversation       string            `json:"conversation"`
	Metadata           map[string]string `json:"metadata"`
	Tools              json.RawMessage   `json:"tools"`
}

func (s *Server) responses(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	dec := json.NewDecoder(r.Body)
	var req responseRequest
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "invalid JSON request", middleware.ID(r.Context()))
		return
	}
	if req.Model == "" {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "model is required", middleware.ID(r.Context()))
		return
	}
	input, err := parseInput(req.Input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error(), middleware.ID(r.Context()))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
	defer cancel()
	memTurn := memory.Turn{Prompt: input}
	if s.memory != nil {
		clientKey := ""
		projectID := ""
		if req.Metadata != nil { clientKey, projectID = req.Metadata["ncg_conversation_id"], req.Metadata["ncg_project_id"] }
		if clientKey == "" { clientKey = r.Header.Get("X-NotionCodex-Conversation-ID") }
		prepared, memoryErr := s.memory.Start(ctx, memory.Request{Scope: memory.Scope{UserID: "local", ProjectID: projectID}, ConversationID: req.Conversation, PreviousResponseID: req.PreviousResponseID, ClientKey: clientKey, Input: input, Model: req.Model})
		if memoryErr != nil { writeError(w, http.StatusInternalServerError, "memory_error", "unable to prepare conversation context", middleware.ID(r.Context())); return }
		memTurn = prepared
		w.Header().Set("X-NotionCodex-Conversation-ID", memTurn.ConversationID)
	}
	if s.mcp != nil && len(req.Tools) > 0 {
		text, loopErr := s.runGatewayToolLoop(ctx, req.Model, memTurn.Prompt, memTurn)
		if loopErr != nil {
			if s.memory != nil { s.memory.Abort(ctx, memTurn, loopErr.Error()) }
			writeError(w, http.StatusBadGateway, "mcp_tool_loop_error", safeErr(loopErr), middleware.ID(r.Context()))
			return
		}
		static := &staticEventStream{events: []domain.InferenceEvent{{Kind: domain.EventDelta, Text: text}, {Kind: domain.EventDone}}}
		if req.Stream { s.streamResponse(ctx, w, static, req.Model, middleware.ID(r.Context()), memTurn); return }
		s.nonStream(ctx, w, static, req.Model, middleware.ID(r.Context()), memTurn)
		return
	}
	stream, err := s.transport.Infer(ctx, domain.InferenceRequest{Model: req.Model, Input: memTurn.Prompt})
	if err != nil {
		if s.memory != nil { s.memory.Abort(ctx, memTurn, err.Error()) }
		writeError(w, http.StatusBadGateway, "upstream_error", err.Error(), middleware.ID(r.Context()))
		return
	}
	defer stream.Close()
	if req.Stream {
		s.streamResponse(ctx, w, stream, req.Model, middleware.ID(r.Context()), memTurn)
		return
	}
	s.nonStream(ctx, w, stream, req.Model, middleware.ID(r.Context()), memTurn)
}
func parseInput(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", errors.New("input is required")
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", errors.New("input must be valid JSON")
	}
	var userMessages []string
	collectUserMessages(value, &userMessages)
	if len(userMessages) > 0 {
		return userMessages[len(userMessages)-1], nil
	}
	var parts []string
	collectInputText(value, &parts)
	if len(parts) == 0 {
		return "", errors.New("input contains no supported text content")
	}
	return strings.Join(parts, "\n"), nil
}

func collectUserMessages(value any, messages *[]string) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed { collectUserMessages(item, messages) }
	case map[string]any:
		if role, _ := typed["role"].(string); role == "user" {
			var parts []string
			if content, ok := typed["content"]; ok { collectInputText(content, &parts) }
			if text, ok := typed["text"].(string); ok && text != "" { parts = append(parts, text) }
			if len(parts) > 0 { *messages = append(*messages, strings.Join(parts, "\n")) }
			return
		}
		for _, child := range typed { collectUserMessages(child, messages) }
	}
}

func collectInputText(value any, parts *[]string) {
	switch typed := value.(type) {
	case string:
		if typed != "" {
			*parts = append(*parts, typed)
		}
	case []any:
		for _, item := range typed {
			collectInputText(item, parts)
		}
	case map[string]any:
		if text, ok := typed["text"].(string); ok && text != "" {
			kind, _ := typed["type"].(string)
			if kind == "" || kind == "input_text" || kind == "text" {
				*parts = append(*parts, text)
				return
			}
		}
		if content, ok := typed["content"]; ok {
			collectInputText(content, parts)
		}
	}
}
func (s *Server) nonStream(ctx context.Context, w http.ResponseWriter, st ports.EventStream, model, id string, memTurn memory.Turn) {
	var b strings.Builder
	for {
		e, err := st.Recv(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeError(w, http.StatusBadGateway, "upstream_error", safeErr(err), id)
			return
		}
		switch e.Kind {
		case domain.EventDelta:
			b.WriteString(e.Text)
		case domain.EventTool:
			s.recordToolEvent(ctx, memTurn, e.Tool, id)
		case domain.EventError:
			if s.memory != nil { s.memory.Abort(ctx, memTurn, safeErr(e.Err)) }
			writeError(w, http.StatusBadGateway, "upstream_error", safeErr(e.Err), id)
			return
		case domain.EventDone:
			if s.memory != nil { if err := s.memory.Complete(ctx, memTurn, b.String(), "resp_"+id); err != nil { s.logger.Warn("memory_complete_failed", "request_id", id, "error", safeErr(err)) } }
			writeJSON(w, http.StatusOK, responseObject(id, model, b.String()))
			return
		}
	}
	writeError(w, http.StatusBadGateway, "upstream_error", "stream ended without terminal event", id)
}
func (s *Server) streamResponse(ctx context.Context, w http.ResponseWriter, st ports.EventStream, model, id string, memTurn memory.Turn) {
	f, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "server_error", "streaming unsupported", id)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	rid := "resp_" + id
	seq := 0
	emit := func(kind string, data any) {
		payload, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", kind, payload)
		f.Flush()
	}
	emit("response.created", map[string]any{"type": "response.created", "sequence_number": seq, "response": map[string]any{"id": rid, "object": "response", "status": "in_progress", "model": model}})
	seq++
	emit("response.output_item.added", map[string]any{"type": "response.output_item.added", "sequence_number": seq, "output_index": 0, "item": map[string]any{"id": "msg_" + id, "type": "message", "role": "assistant", "status": "in_progress", "content": []any{}}})
	seq++
	emit("response.content_part.added", map[string]any{"type": "response.content_part.added", "sequence_number": seq, "output_index": 0, "content_index": 0, "part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}}})
	seq++
	var b strings.Builder
	terminal := false
	for !terminal {
		e, err := st.Recv(ctx)
		if errors.Is(err, io.EOF) {
			terminal = true
			break
		}
		if err != nil {
			if s.memory != nil { s.memory.Abort(ctx, memTurn, safeErr(err)) }
			emit("error", map[string]any{"type": "error", "sequence_number": seq, "error": map[string]any{"type": "upstream_error", "message": safeErr(err)}})
			return
		}
		switch e.Kind {
		case domain.EventDelta:
			if e.Text == "" {
				continue
			}
			b.WriteString(e.Text)
			emit("response.output_text.delta", map[string]any{"type": "response.output_text.delta", "sequence_number": seq, "output_index": 0, "content_index": 0, "delta": e.Text})
			seq++
		case domain.EventTool:
			s.recordToolEvent(ctx, memTurn, e.Tool, id)
		case domain.EventError:
			if s.memory != nil { s.memory.Abort(ctx, memTurn, safeErr(e.Err)) }
			emit("error", map[string]any{"type": "error", "sequence_number": seq, "error": map[string]any{"type": "upstream_error", "message": safeErr(e.Err)}})
			return
		case domain.EventDone:
			terminal = true
		}
	}
	text := b.String()
	if s.memory != nil { if err := s.memory.Complete(ctx, memTurn, text, "resp_"+id); err != nil { s.logger.Warn("memory_complete_failed", "request_id", id, "error", safeErr(err)) } }
	emit("response.output_text.done", map[string]any{"type": "response.output_text.done", "sequence_number": seq, "output_index": 0, "content_index": 0, "text": text})
	seq++
	emit("response.output_item.done", map[string]any{"type": "response.output_item.done", "sequence_number": seq, "output_index": 0, "item": map[string]any{"id": "msg_" + id, "type": "message", "role": "assistant", "status": "completed", "content": []any{map[string]any{"type": "output_text", "text": text, "annotations": []any{}}}}})
	seq++
	emit("response.completed", map[string]any{"type": "response.completed", "sequence_number": seq, "response": responseObject(id, model, text)})
}

func (s *Server) recordToolEvent(ctx context.Context, turn memory.Turn, event *domain.ToolEvent, requestID string) {
	if s.memory == nil || event == nil || turn.ID == "" {
		return
	}
	_, err := s.memory.RecordToolExecution(ctx, memory.ToolExecution{
		Scope: turn.Scope, ConversationID: turn.ConversationID, TurnID: turn.ID,
		ToolName: event.Name, ToolVersion: event.Version, Operation: event.Operation,
		PayloadVariant: event.PayloadVariant, InputSummary: event.InputSummary,
		OutputSummary: event.OutputSummary, Status: event.Status, ErrorCategory: event.ErrorCategory,
		ErrorSummary: event.ErrorSummary, CodeRevision: event.CodeRevision,
		ArtifactRefsJSON: event.ArtifactRefsJSON, IdempotencyKey: event.IdempotencyKey,
	})
	if err != nil {
		s.logger.Warn("memory_tool_record_failed", "request_id", requestID, "error", safeErr(err))
	}
}

func responseObject(id, model, text string) map[string]any {
	return map[string]any{"id": "resp_" + id, "object": "response", "created_at": time.Now().Unix(), "status": "completed", "model": model, "output": []any{map[string]any{"id": "msg_" + id, "type": "message", "role": "assistant", "status": "completed", "content": []any{map[string]any{"type": "output_text", "text": text, "annotations": []any{}}}}}, "error": nil}
}
func safeErr(err error) string {
	if err == nil {
		return "upstream error"
	}
	if errors.Is(err, context.Canceled) {
		return "request canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "request timed out"
	}
	return err.Error()
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, typ, msg, id string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"message": msg, "type": typ, "param": nil, "code": nil, "request_id": id}})
}
