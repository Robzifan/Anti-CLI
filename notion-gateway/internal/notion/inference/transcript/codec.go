// Package transcript implements the observed runInferenceTranscript request and JSON stream codec.
package transcript

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/notioncodex/gateway/internal/domain"
)

const ObservedPath = "/api/v3/runInferenceTranscript"

func debugEnabled() bool {
	return strings.EqualFold(os.Getenv("NOTION_CODEX_DEBUG"), "true") || os.Getenv("NOTION_CODEX_DEBUG") == "1"
}

func debugf(format string, args ...any) {
	if debugEnabled() {
		log.Printf("[transcript] "+format, args...)
	}
}

func traceContentEnabled() bool {
	return debugEnabled() || strings.EqualFold(os.Getenv("NOTION_CODEX_TRACE_CONTENT"), "true") || os.Getenv("NOTION_CODEX_TRACE_CONTENT") == "1"
}

func traceContentf(format string, args ...any) {
	if traceContentEnabled() {
		log.Printf("[transcript-trace] "+format, args...)
	}
}

func previewValue(value any, limit int) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("<marshal error: %v>", err)
	}
	text := strings.TrimSpace(string(payload))
	if len(text) > limit {
		return text[:limit] + "…"
	}
	if text == "" {
		return "<empty>"
	}
	return text
}

type BuildOptions struct {
	SpaceID  string
	Model    string
	Input    string
	Timezone string
}

type BuildVariant struct {
	Name    string
	Payload []byte
}

func Build(options BuildOptions) ([]byte, error) {
	variants, err := BuildVariants(options)
	if err != nil {
		return nil, err
	}
	return variants[0].Payload, nil
}

func BuildVariants(options BuildOptions) ([]BuildVariant, error) {
	if options.SpaceID == "" || options.Model == "" || options.Input == "" {
		return nil, errors.New("space, model, and input are required")
	}

	traceID, err := randomID()
	if err != nil {
		return nil, err
	}
	threadID, err := randomID()
	if err != nil {
		return nil, err
	}
	configID, err := randomID()
	if err != nil {
		return nil, err
	}
	contextID, err := randomID()
	if err != nil {
		return nil, err
	}
	userStepID, err := randomID()
	if err != nil {
		return nil, err
	}

	now := time.Now().In(loadLocation(options.Timezone))
	currentDatetime := now.Format("2006-01-02T15:04:05.000Z07:00")
	createdAt := currentDatetime

	userName := getenvDefault("NOTION_USER_NAME", "yarik")
	userID := getenvDefault("NOTION_USER_ID", "397d872b-594c-811c-b2f0-00029adf0292")
	userEmail := getenvDefault("NOTION_USER_EMAIL", "vncresolver@gmail.com")
	spaceName := getenvDefault("NOTION_SPACE_NAME", "yarik’s Space")
	spaceViewID := getenvDefault("NOTION_SPACE_VIEW_ID", "397edeb0-7557-806b-8f0b-0006d376e3d0")

	configValue := map[string]any{
		"type": "workflow",
		"enableAgentAutomations": true,
		"enableAgentIntegrations": true,
		"enableCustomAgents": true,
		"enableExperimentalIntegrations": false,
		"enableScriptAgent": true,
		"enableScriptAgentAdvanced": false,
		"enableScriptAgentSearchConnectorsInCustomAgent": false,
		"enableScriptAgentGoogleDriveInCustomAgent": false,
		"enableScriptAgentGoogleDriveOAuthInCustomAgent": false,
		"enableScriptAgentSlack": true,
		"enableScriptAgentMcpServers": false,
		"enableAgentDiffs": true,
		"enableCsvAttachmentSupport": true,
		"showDatabaseAgentsDiscoverability": true,
		"enableAgentThreadTools": false,
		"enableCrdtOperations": false,
		"enableAgentCardCustomization": true,
		"enableSystemPromptAsPage": false,
		"enableUserSessionContext": false,
		"enableLargeToolResultComputerOffload": false,
		"enableScriptAgentGtm": false,
		"enablePitCrewTableViewTool": false,
		"enableComputer": true,
		"enableCreateAndRunThread": true,
		"enableCustomAgentCreateGuidanceV2": false,
		"enableSoftwareFactoryPage": false,
		"enableAgentGenerateImage": true,
		"enableQueryCalendar": false,
		"enableQueryMail": false,
		"enableMailExplicitToolCalls": true,
		"enableMailNotificationPreferences": false,
		"enableMailAgentMultiProviderSupport": true,
		"enableWebResearch": false,
		"useRulePrioritization": true,
		"availableConnectors": []any{},
		"searchScopes": []any{map[string]any{"type": "everything"}},
		"useWebSearch": true,
		"isHipaa": false,
		"internetAccess": false,
		"manageWorkers": false,
		"useReadOnlyMode": false,
		"writerMode": false,
		"modelFromUser": true,
		"isCustomAgent": false,
		"isCustomAgentBuilder": false,
		"isAgentResearchRequest": false,
		"useCustomAgentDraft": false,
		"enableUpdatePageAutofixer": true,
		"enableMarkdownVNext": false,
		"enableAgentSkillsV2": true,
		"updatePageStaleViewGuardEnabled": false,
		"enableUpdatePageOrderUpdates": true,
		"enableAgentSupportPropertyReorder": true,
		"enableAgentAskSurvey": true,
		"databaseAgentConfigMode": false,
		"isOnboardingAgent": false,
		"isMobile": false,
		"useContextualCoreDocsAutoLoad": false,
		"useDocPreviewsForCoreAutoLoad": true,
		"isThreadStartedByAdmin": true,
		"model": options.Model,
	}

	actualObserved := map[string]any{
		"traceId": traceID,
		"spaceId": options.SpaceID,
		"transcript": []any{
			map[string]any{"id": configID, "type": "config", "value": configValue},
			map[string]any{"id": contextID, "type": "context", "value": map[string]any{
				"timezone": options.Timezone,
				"userName": userName,
				"userId": userID,
				"userEmail": userEmail,
				"spaceName": spaceName,
				"spaceId": options.SpaceID,
				"spaceViewId": spaceViewID,
				"currentDatetime": currentDatetime,
				"surface": "ai_module",
			}},
			map[string]any{"id": userStepID, "type": "user", "userId": userID, "value": []any{[]any{options.Input}}, "createdAt": createdAt},
		},
		"threadId": threadID,
		"threadParentPointer": map[string]any{"table": "space", "id": options.SpaceID, "spaceId": options.SpaceID},
		"createThread": true,
		"debugOverrides": map[string]any{
			"emitAgentSearchExtractedResults": true,
			"cachedInferences": map[string]any{},
			"annotationInferences": map[string]any{},
			"emitInferences": false,
		},
		"generateTitle": true,
		"saveAllThreadOperations": true,
		"setUnreadState": true,
		"createdSource": "ai_module",
		"threadType": "workflow",
		"isPartialTranscript": false,
		"asPatchResponse": true,
		"patchResponseVersion": 2,
		"isUserInAnySalesAssistedSpace": false,
		"isSpaceSalesAssisted": false,
		"supportsCustomAgentNudgeTranscriptStep": true,
		"customAgentNudgePrecheckContext": map[string]any{
			"threadId": threadID,
			"experimentId": "nosey_to_custom_agent_nudge",
			"isEligible": true,
			"isTreatmentVariant": false,
		},
	}

	userObject := map[string]any{"type": "text", "content": options.Input}

	variants := []struct {
		name    string
		request map[string]any
	}{
		{name: "actualObserved", request: actualObserved},
		{
			name: "minimalObjectUser",
			request: map[string]any{
				"traceId": traceID,
				"spaceId": options.SpaceID,
				"threadId": threadID,
				"createThread": true,
				"transcript": []any{
					map[string]any{"id": configID, "type": "config", "value": map[string]any{"type": "workflow", "model": options.Model, "modelFromUser": true, "useWebSearch": true}},
					map[string]any{"id": contextID, "type": "context", "value": map[string]any{"timezone": options.Timezone, "spaceId": options.SpaceID, "surface": "ai_module", "userName": userName, "userId": userID, "spaceName": spaceName, "spaceViewId": spaceViewID, "currentDatetime": currentDatetime}},
					map[string]any{"id": userStepID, "type": "user", "userId": userID, "value": userObject, "createdAt": createdAt},
				},
				"threadParentPointer": map[string]any{"table": "space", "id": options.SpaceID, "spaceId": options.SpaceID},
				"generateTitle": true,
				"saveAllThreadOperations": true,
				"setUnreadState": true,
				"createdSource": "ai_module",
				"threadType": "workflow",
				"asPatchResponse": true,
				"patchResponseVersion": 2,
			},
		},
		{
			name: "minimalArrayUser",
			request: map[string]any{
				"traceId": traceID,
				"spaceId": options.SpaceID,
				"threadId": threadID,
				"createThread": true,
				"transcript": []any{
					map[string]any{"id": configID, "type": "config", "value": map[string]any{"type": "workflow", "model": options.Model, "modelFromUser": true, "useWebSearch": true}},
					map[string]any{"id": contextID, "type": "context", "value": map[string]any{"timezone": options.Timezone, "spaceId": options.SpaceID, "surface": "ai_module", "userName": userName, "userId": userID, "spaceName": spaceName, "spaceViewId": spaceViewID, "currentDatetime": currentDatetime}},
					map[string]any{"id": userStepID, "type": "user", "userId": userID, "value": []any{[]any{options.Input}}, "createdAt": createdAt},
				},
				"threadParentPointer": map[string]any{"table": "space", "id": options.SpaceID, "spaceId": options.SpaceID},
				"generateTitle": true,
				"saveAllThreadOperations": true,
				"setUnreadState": true,
				"createdSource": "ai_module",
				"threadType": "workflow",
				"asPatchResponse": true,
				"patchResponseVersion": 2,
			},
		},
	}

	result := make([]BuildVariant, 0, len(variants))
	for _, variant := range variants {
		payload, err := json.Marshal(variant.request)
		if err != nil {
			return nil, err
		}
		debugf("build request variant=%s model=%s input_len=%d payload=%s", variant.name, options.Model, len(options.Input), previewValue(variant.request, 2400))
		result = append(result, BuildVariant{Name: variant.name, Payload: payload})
	}
	return result, nil
}

func loadLocation(name string) *time.Location {
	if strings.TrimSpace(name) == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone(name, 0)
	}
	return loc
}

func getenvDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func randomID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate request id: %w", err)
	}
	return hex.EncodeToString(value[0:4])+"-"+hex.EncodeToString(value[4:6])+"-"+hex.EncodeToString(value[6:8])+"-"+hex.EncodeToString(value[8:10])+"-"+hex.EncodeToString(value[10:16]), nil
}

type tailBuffer struct {
	limit int
	data  []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	if t.limit <= 0 {
		return len(p), nil
	}
	if len(p) >= t.limit {
		t.data = append(t.data[:0], p[len(p)-t.limit:]...)
		return len(p), nil
	}
	if overflow := len(t.data)+len(p)-t.limit; overflow > 0 {
		t.data = append(t.data[overflow:], p...)
		return len(p), nil
	}
	t.data = append(t.data, p...)
	return len(p), nil
}

func (t *tailBuffer) String() string {
	if len(t.data) == 0 {
		return "<empty>"
	}
	s := strings.TrimSpace(string(t.data))
	if s == "" {
		return "<whitespace>"
	}
	return s
}

type Decoder struct {
	reader           *bufio.Reader
	decoder          *json.Decoder
	rawTail          *tailBuffer
	arrayMode        bool
	initialized      bool
	previous         string
	pendingDone      bool
	done             bool
	arrayLengths     map[string]int
	nodeKinds        map[string]string
	textContentPaths map[string]string
	textStepPaths    map[string]struct{}
}

func NewDecoder(reader io.Reader) *Decoder {
	tail := &tailBuffer{limit: 4096}
	buffered := bufio.NewReader(io.TeeReader(reader, tail))
	return &Decoder{
		reader:           buffered,
		rawTail:          tail,
		arrayLengths:     map[string]int{},
		nodeKinds:        map[string]string{},
		textContentPaths: map[string]string{},
		textStepPaths:    map[string]struct{}{},
	}
}

func (d *Decoder) Next(ctx context.Context) (domain.InferenceEvent, error) {
	if err := ctx.Err(); err != nil {
		return domain.InferenceEvent{}, err
	}
	if d.pendingDone {
		d.pendingDone = false
		d.done = true
		return domain.InferenceEvent{Kind: domain.EventDone}, nil
	}
	if d.done {
		return domain.InferenceEvent{}, io.EOF
	}
	if err := d.initStream(); err != nil {
		if errors.Is(err, io.EOF) {
			return domain.InferenceEvent{}, io.EOF
		}
		return domain.InferenceEvent{}, fmt.Errorf("decode notion stream frame: %w; raw_tail=%q", err, d.rawTail.String())
	}

	for {
		if err := ctx.Err(); err != nil {
			return domain.InferenceEvent{}, err
		}

		if d.arrayMode && !d.decoder.More() {
			debugf("array stream reports no more elements")
			token, err := d.decoder.Token()
			if err != nil {
				return domain.InferenceEvent{}, fmt.Errorf("decode notion stream frame: %w; raw_tail=%q", err, d.rawTail.String())
			}
			delim, ok := token.(json.Delim)
			if !ok || delim != ']' {
				return domain.InferenceEvent{}, fmt.Errorf("decode notion stream frame: unexpected closing token %v; raw_tail=%q", token, d.rawTail.String())
			}
			d.done = true
			return domain.InferenceEvent{}, io.EOF
		}

		var frame any
		if err := d.decoder.Decode(&frame); err != nil {
			if errors.Is(err, io.EOF) {
				return domain.InferenceEvent{}, io.EOF
			}
			return domain.InferenceEvent{}, fmt.Errorf("decode notion stream frame: %w; raw_tail=%q", err, d.rawTail.String())
		}

		debugf("decoded frame preview=%s", previewValue(frame, 1600))
		text, terminal, handled := d.inspectPatchFrame(frame)
		if !handled {
			text, terminal = inspect(frame)
		}
		debugf("frame result terminal=%t text_len=%d text_preview=%q", terminal, len(text), text)
		if terminal && text == "" {
			traceContentf("terminal frame without visible text text_paths=%d node_kinds=%d raw_tail=%q", len(d.textContentPaths), len(d.nodeKinds), d.rawTail.String())
		}
		if text != "" {
			delta := text
			if strings.HasPrefix(text, d.previous) {
				delta = strings.TrimPrefix(text, d.previous)
			}
			d.previous = text
			if delta != "" {
				if terminal {
					d.pendingDone = true
				}
				return domain.InferenceEvent{Kind: domain.EventDelta, Text: delta}, nil
			}
		}
		if terminal {
			d.done = true
			return domain.InferenceEvent{Kind: domain.EventDone}, nil
		}
	}
}

func (d *Decoder) initStream() error {
	if d.initialized {
		return nil
	}
	for {
		b, err := d.reader.ReadByte()
		if err != nil {
			return err
		}
		if isSpace(b) {
			continue
		}
		if err := d.reader.UnreadByte(); err != nil {
			return err
		}

		d.decoder = json.NewDecoder(d.reader)
		if b == '[' {
			token, err := d.decoder.Token()
			if err != nil {
				return err
			}
			delim, ok := token.(json.Delim)
			if !ok || delim != '[' {
				return fmt.Errorf("unexpected opening token %v", token)
			}
			d.arrayMode = true
			debugf("stream initialized in array mode")
		} else {
			debugf("stream initialized in object mode first_byte=%q", b)
		}

		d.initialized = true
		return nil
	}
}

func isSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

type walkState struct {
	ignored       bool
	assistantLike bool
}

func (d *Decoder) inspectPatchFrame(value any) (string, bool, bool) {
	frame, ok := value.(map[string]any)
	if !ok {
		return "", false, false
	}
	frameType, _ := frame["type"].(string)
	if frameType == "patch-start" {
		data, _ := frame["data"].(map[string]any)
		items, _ := data["s"].([]any)
		d.arrayLengths["/s"] = len(items)
		for index, item := range items {
			d.registerValue(fmt.Sprintf("/s/%d", index), item)
		}
		return "", false, true
	}
	if frameType == "patch-sync" {
		data, _ := frame["data"].(map[string]any)
		items, _ := data["s"].([]any)
		if len(items) == 0 { return "", false, true }
		d.arrayLengths["/s"] = len(items)
		var builder strings.Builder
		for index, item := range items {
			builder.WriteString(d.registerValue(fmt.Sprintf("/s/%d", index), item))
		}
		terminal := false
		for index := len(items) - 1; index >= 0; index-- {
			item, ok := items[index].(map[string]any)
			if !ok { continue }
			if kind, _ := item["type"].(string); kind == "agent-inference" {
				terminal = containsFinishedMarker(item) && containsVisibleText(item)
				break
			}
		}
		return builder.String(), terminal, true
	}
	if frameType != "patch" {
		return "", false, false
	}
	ops, ok := frame["v"].([]any)
	if !ok {
		return "", false, true
	}
	var builder strings.Builder
	terminal := false
	for _, rawOp := range ops {
		op, ok := rawOp.(map[string]any)
		if !ok {
			continue
		}
		verb, _ := op["o"].(string)
		path, _ := op["p"].(string)
		basePath := strings.TrimSuffix(path, "/content")
		stepPath := stepPathFromBase(basePath)
		switch verb {
		case "a":
			if strings.HasSuffix(path, "/-") {
				parent := strings.TrimSuffix(path, "/-")
				idx := d.arrayLengths[parent]
				d.arrayLengths[parent] = idx + 1
				basePath := fmt.Sprintf("%s/%d", parent, idx)
				builder.WriteString(d.registerValue(basePath, op["v"]))
				continue
			}
			if strings.HasSuffix(path, "/finishedAt") {
				stepPath := strings.TrimSuffix(path, "/finishedAt")
				if _, ok := d.textStepPaths[stepPath]; ok {
					terminal = true
					traceContentf("step finished step=%s text_paths=%d", stepPath, len(d.textContentPaths))
				}
				continue
			}
			if isTextContentPath(d.nodeKinds, d.textStepPaths, path) {
				current := d.textContentPaths[path]
				if replacement, ok := op["v"].(string); ok {
					delta := replacement
					if strings.HasPrefix(replacement, current) {
						delta = strings.TrimPrefix(replacement, current)
					}
					d.textContentPaths[path] = replacement
					traceContentf("accepted content op=%s path=%s base=%s step=%s kind=%q delta=%q", verb, path, basePath, stepPath, d.nodeKinds[basePath], delta)
					builder.WriteString(delta)
				}
			} else if strings.HasSuffix(path, "/content") {
				traceContentf("ignored content op=%s path=%s base=%s step=%s kind=%q value=%s", verb, path, basePath, stepPath, d.nodeKinds[basePath], previewValue(op["v"], 120))
			}
		case "x", "p":
			if isTextContentPath(d.nodeKinds, d.textStepPaths, path) {
				current := d.textContentPaths[path]
				if chunk, ok := op["v"].(string); ok {
					if verb == "x" {
						d.textContentPaths[path] = current + chunk
						traceContentf("accepted content op=%s path=%s base=%s step=%s kind=%q delta=%q", verb, path, basePath, stepPath, d.nodeKinds[basePath], chunk)
						builder.WriteString(chunk)
					} else {
						delta := chunk
						if strings.HasPrefix(chunk, current) {
							delta = strings.TrimPrefix(chunk, current)
						}
						d.textContentPaths[path] = chunk
						traceContentf("accepted content op=%s path=%s base=%s step=%s kind=%q delta=%q", verb, path, basePath, stepPath, d.nodeKinds[basePath], delta)
						builder.WriteString(delta)
					}
				}
			} else if strings.HasSuffix(path, "/content") {
				traceContentf("ignored content op=%s path=%s base=%s step=%s kind=%q value=%s", verb, path, basePath, stepPath, d.nodeKinds[basePath], previewValue(op["v"], 120))
			}
		}
	}
	return builder.String(), terminal, true
}

func (d *Decoder) registerValue(basePath string, raw any) string {
	item, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	itemType, _ := item["type"].(string)
	if itemType != "" {
		d.nodeKinds[basePath] = itemType
		traceContentf("registered node base=%s step=%s kind=%q", basePath, stepPathFromBase(basePath), itemType)
	}
	var builder strings.Builder
	if itemType == "text" || itemType == "output_text" {
		content, _ := item["content"].(string)
		contentPath := basePath + "/content"
		d.textContentPaths[contentPath] = content
		d.textStepPaths[stepPathFromBase(basePath)] = struct{}{}
		traceContentf("registered visible text base=%s path=%s step=%s content=%q", basePath, contentPath, stepPathFromBase(basePath), content)
		builder.WriteString(content)
	}
	if valueItems, ok := item["value"].([]any); ok {
		parent := basePath + "/value"
		d.arrayLengths[parent] = len(valueItems)
		for i, child := range valueItems {
			builder.WriteString(d.registerValue(fmt.Sprintf("%s/%d", parent, i), child))
		}
	}
	return builder.String()
}

func stepPathFromBase(basePath string) string {
	if idx := strings.Index(basePath, "/value/"); idx >= 0 {
		return basePath[:idx]
	}
	return basePath
}

func isTextContentPath(nodeKinds map[string]string, textStepPaths map[string]struct{}, path string) bool {
	if !strings.HasSuffix(path, "/content") {
		return false
	}
	basePath := strings.TrimSuffix(path, "/content")
	kind := nodeKinds[basePath]
	if kind == "text" || kind == "output_text" {
		return true
	}
	if kind == "thinking" || kind == "tool_use" {
		return false
	}
	_, stepHasVisibleText := textStepPaths[stepPathFromBase(basePath)]
	return stepHasVisibleText && strings.Contains(basePath, "/value/")
}

func inspect(value any) (string, bool) {
	var preferred []string
	var fallback []string
	seenPreferred := map[string]struct{}{}
	seenFallback := map[string]struct{}{}
	terminal := false

	var walk func(any, walkState)
	walk = func(current any, state walkState) {
		switch typed := current.(type) {
		case map[string]any:
			kind, _ := typed["type"].(string)
			next := walkState{
				ignored:       state.ignored || isIgnoredKind(kind),
				assistantLike: state.assistantLike || isAssistantKind(kind),
			}
			if kind == "agent-turn-full-record-map" {
				next.assistantLike = true
			}
			if isTerminalMap(typed, kind) {
				terminal = true
			}
			appendPreferred(&preferred, seenPreferred, typed, kind, next.ignored)
			if next.assistantLike && !next.ignored {
				appendFallbackStrings(&fallback, seenFallback, typed)
			}
			for _, child := range typed {
				walk(child, next)
			}
		case []any:
			for _, child := range typed {
				walk(child, state)
			}
		}
	}

	walk(value, walkState{})
	text := strings.Join(preferred, "")
	if text == "" {
		text = strings.Join(fallback, "")
	}
	return dedupeAdjacent(text), terminal
}

func appendPreferred(dst *[]string, seen map[string]struct{}, current map[string]any, kind string, ignored bool) {
	if ignored {
		return
	}
	appendUniqueString(dst, seen, kind, current["content"])
	appendUniqueString(dst, seen, kind, current["text"])
	appendUniqueString(dst, seen, kind, current["plainText"])
	appendUniqueString(dst, seen, kind, current["plain_text"])
	appendUniqueString(dst, seen, kind, current["markdown"])
	appendUniqueString(dst, seen, kind, current["displayText"])
	if textMap, ok := current["text"].(map[string]any); ok {
		appendUniqueString(dst, seen, kind, textMap["content"])
		appendUniqueString(dst, seen, kind, textMap["text"])
	}
	if valueMap, ok := current["value"].(map[string]any); ok {
		appendUniqueString(dst, seen, kind, valueMap["content"])
		appendUniqueString(dst, seen, kind, valueMap["text"])
		appendUniqueString(dst, seen, kind, valueMap["plainText"])
	}
}

func appendFallbackStrings(dst *[]string, seen map[string]struct{}, current map[string]any) {
	for key, raw := range current {
		if isIgnoredLeafKey(key) {
			continue
		}
		value, ok := raw.(string)
		if !ok {
			continue
		}
		appendUniqueString(dst, seen, "", value)
	}
}

func appendUniqueString(dst *[]string, seen map[string]struct{}, kind string, raw any) {
	value, ok := raw.(string)
	if !ok {
		return
	}
	value = strings.TrimSpace(value)
	if !shouldKeepText(kind, value) {
		return
	}
	if _, exists := seen[value]; exists {
		return
	}
	seen[value] = struct{}{}
	*dst = append(*dst, value)
}

func isTerminalMap(current map[string]any, kind string) bool {
	// Full record maps can be intermediate snapshots in live streams. A map
	// carrying visible text is terminal only with an explicit completion marker;
	// a textless full map remains the legacy terminal marker.
	if kind == "agent-turn-full-record-map" {
		return !containsVisibleText(current) || containsFinishedMarker(current)
	}
	if kind == "agent-inference" {
		if _, ok := current["finishedAt"]; ok {
			// Some terminal snapshots expose assistant text only in `message`,
			// rather than as a nested text/output_text node.
			if message, ok := current["message"].(string); ok && strings.TrimSpace(message) != "" {
				return true
			}
			return containsVisibleText(current)
		}
	}
	if completed, _ := current["completed"].(bool); completed {
		return true
	}
	if status, _ := current["status"].(string); status == "completed" {
		if _, ok := current["final_step_id"]; ok {
			return true
		}
		if _, ok := current["completed_time"]; ok {
			return true
		}
	}
	if _, ok := current["finishedAt"]; ok {
		if kind == "" || kind == "text" || kind == "output_text" {
			return true
		}
	}
	return false
}


func containsFinishedMarker(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		if _, ok := typed["finishedAt"]; ok { return true }
		if completed, _ := typed["completed"].(bool); completed { return true }
		if status, _ := typed["status"].(string); status == "completed" { return true }
		for _, child := range typed { if containsFinishedMarker(child) { return true } }
	case []any:
		for _, child := range typed { if containsFinishedMarker(child) { return true } }
	}
	return false
}

func isIgnoredKind(kind string) bool {
	switch kind {
	case "config", "updated-config", "context", "user", "thinking", "title", "tool_use", "agent-tool-result", "agent-instruction-state":
		return true
	default:
		return false
	}
}

func isAssistantKind(kind string) bool {
	switch kind {
	case "assistant", "message", "agent-inference", "agent-turn-full-record-map", "text", "output_text":
		return true
	default:
		return false
	}
}

func isIgnoredLeafKey(key string) bool {
	switch key {
	case "id", "type", "role", "status", "model", "spaceId", "threadId", "traceId", "surface", "timezone", "createdSource", "object", "name":
		return true
	default:
		return false
	}
}

func shouldKeepText(kind, text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if isIgnoredKind(kind) {
		return false
	}
	if strings.HasPrefix(text, `{"function":"connections.`) {
		return false
	}
	switch text {
	case "workflow", "gateway", "ai_module", "assistant", "message", "text", "output_text", "thinking", "tool_use":
		return false
	default:
		return true
	}
}

func containsVisibleText(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		kind, _ := typed["type"].(string)
		if kind == "text" || kind == "output_text" {
			if content, _ := typed["content"].(string); shouldKeepText(kind, content) {
				return true
			}
			if text, _ := typed["text"].(string); shouldKeepText(kind, text) {
				return true
			}
		}
		for _, child := range typed {
			if containsVisibleText(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsVisibleText(child) {
				return true
			}
		}
	}
	return false
}

func dedupeAdjacent(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	for i := len(text) / 2; i > 0; i-- {
		if len(text)%i != 0 {
			continue
		}
		segment := text[:i]
		if strings.Repeat(segment, len(text)/i) == text {
			return segment
		}
	}
	return text
}
