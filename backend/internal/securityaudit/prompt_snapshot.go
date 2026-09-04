package securityaudit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	ErrNoPromptText = errors.New("prompt audit request contains no user text")

	bearerPattern = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+\-/]+=*`)
	apiKeyPattern = regexp.MustCompile(`(?i)\b(sk|rk|pk|api[_-]?key|token|secret|password)[-_:=\s]+[A-Za-z0-9._~+\-/]{8,}`)
	canaryPattern = regexp.MustCompile(`(?i)([A-Z]+_CANARY_)[A-Za-z0-9_-]+`)
	emailPattern  = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	phonePattern  = regexp.MustCompile(`(?:\+?\d[\d\s().-]{8,}\d)`)
)

const promptAuditPrioritySeparator = "\x00SUB2API_PROMPT_AUDIT_PRIORITY_END\x00"

type promptSegment struct {
	text string
	user bool
	role string
}

// ConversationEvidence is a bounded, role-preserving snapshot used for
// post-upstream forensic records. InputHash and InputLength describe the full
// normalized transcript before Snapshot is truncated.
type ConversationEvidence struct {
	Snapshot     string
	Excerpt      string
	InputHash    string
	InputLength  int
	MessageCount int
	Truncated    bool
}

const DefaultConversationEvidenceMaxRunes = 12000

const (
	conversationLatestUserRunes = 4000
	conversationAdjacentRunes   = 5000
	conversationSystemRunes     = 2000
	conversationMetadataRunes   = 1000
	// DefaultConversationEvidenceExcerptMaxRunes is shared by Cyber evidence
	// extraction and the handler so offset-based excerpts are bounded exactly
	// like the legacy fallback excerpt.
	DefaultConversationEvidenceExcerptMaxRunes = 240
	conversationExcerptRunes                   = DefaultConversationEvidenceExcerptMaxRunes
)

// ExtractConversationEvidence reuses the prompt-audit protocol parser while
// preserving source order and role labels. It intentionally excludes binary
// media and unrelated request JSON so the stored evidence remains reviewable.
func ExtractConversationEvidence(protocol string, body []byte, maxRunes int) (ConversationEvidence, error) {
	segments, err := extractConversationEvidenceSegments(protocol, body)
	if err != nil {
		return ConversationEvidence{}, err
	}
	if maxRunes <= 0 {
		maxRunes = DefaultConversationEvidenceMaxRunes
	}
	fullText := formatConversationSegments(segments)
	digest := sha256.Sum256([]byte(fullText))
	inputLength := utf8.RuneCountInString(fullText)
	snapshot := fullText
	truncated := false
	if inputLength > maxRunes {
		snapshot = buildPrioritizedConversationEvidence(segments, maxRunes)
		truncated = true
	}
	latestUser := latestConversationUserText(segments)
	return ConversationEvidence{
		Snapshot:     snapshot,
		Excerpt:      TrimRunes(latestConversationEvidenceText(segments, latestUser), conversationExcerptRunes),
		InputHash:    hex.EncodeToString(digest[:]),
		InputLength:  inputLength,
		MessageCount: len(segments),
		Truncated:    truncated,
	}, nil
}

// ExtractConversationEvidenceExcerptAtOffset returns a bounded excerpt around
// an upstream-reported match range. Offsets are rune positions in the
// normalized, role-free conversation text (segments joined in source order),
// which is the closest representation to the text evaluated upstream. The
// caller can fall back to ConversationEvidence.Excerpt when an older upstream
// response does not provide a valid range.
func ExtractConversationEvidenceExcerptAtOffset(protocol string, body []byte, start, end, maxRunes int) (string, error) {
	segments, err := extractConversationEvidenceSegments(protocol, body)
	if err != nil {
		return "", err
	}
	if maxRunes <= 0 {
		maxRunes = conversationExcerptRunes
	}
	text := make([]string, 0, len(segments))
	for _, segment := range segments {
		text = append(text, segment.text)
	}
	return excerptAroundConversationOffset(strings.Join(text, "\n\n"), start, end, maxRunes), nil
}

func extractConversationEvidenceSegments(protocol string, body []byte) ([]promptSegment, error) {
	var document any
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, errors.New("conversation evidence request JSON is invalid")
	}
	segments := normalizedPromptSegments(extractProtocolSegments(protocol, document))
	if len(segments) == 0 {
		return nil, ErrNoPromptText
	}
	return segments, nil
}

func excerptAroundConversationOffset(text string, start, end, maxRunes int) string {
	if text == "" || start < 0 || end <= start || maxRunes <= 0 {
		return ""
	}
	runes := []rune(text)
	if start >= len(runes) {
		return ""
	}
	if end > len(runes) {
		end = len(runes)
	}
	if end <= start {
		return ""
	}
	if len(runes) <= maxRunes {
		return text
	}
	matchRunes := end - start
	if matchRunes >= maxRunes {
		center := start + matchRunes/2
		windowStart := max(0, min(len(runes)-maxRunes, center-maxRunes/2))
		return string(runes[windowStart : windowStart+maxRunes])
	}
	leftBudget := max(0, (maxRunes-matchRunes)/2)
	windowStart := max(0, start-leftBudget)
	windowEnd := min(len(runes), windowStart+maxRunes)
	if windowEnd-windowStart < maxRunes {
		windowStart = max(0, windowEnd-maxRunes)
	}
	return string(runes[windowStart:windowEnd])
}

// latestConversationEvidenceText prefers the last client-controlled non-system
// segment. A Cyber rejection may be caused by a tool result or assistant turn
// rather than the latest user text; using the final relevant segment gives the
// operator a useful local excerpt when the upstream response has no offsets.
func latestConversationEvidenceText(segments []promptSegment, fallback string) string {
	for index := len(segments) - 1; index >= 0; index-- {
		role := strings.ToLower(strings.TrimSpace(segments[index].role))
		if role == "system" || role == "developer" {
			continue
		}
		if text := strings.TrimSpace(segments[index].text); text != "" {
			return text
		}
	}
	return fallback
}

func formatConversationSegments(segments []promptSegment) string {
	var builder strings.Builder
	for index, segment := range segments {
		if index > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(formatConversationSegment(segment))
	}
	return strings.ReplaceAll(strings.TrimSpace(builder.String()), "\x00", "")
}

func formatConversationSegment(segment promptSegment) string {
	role := strings.ToLower(strings.TrimSpace(segment.role))
	if role == "" {
		if segment.user {
			role = "user"
		} else {
			role = "unknown"
		}
	}
	return "[" + role + "]\n" + strings.ReplaceAll(strings.TrimSpace(segment.text), "\x00", "")
}

func latestConversationUserText(segments []promptSegment) string {
	for index := len(segments) - 1; index >= 0; index-- {
		role := strings.ToLower(strings.TrimSpace(segments[index].role))
		if segments[index].user || role == "user" {
			return strings.TrimSpace(segments[index].text)
		}
	}
	return ""
}

func buildPrioritizedConversationEvidence(segments []promptSegment, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	latestUserIndex := -1
	for index := len(segments) - 1; index >= 0; index-- {
		if segments[index].user || strings.EqualFold(strings.TrimSpace(segments[index].role), "user") {
			latestUserIndex = index
			break
		}
	}
	selected := make(map[int]struct{}, len(segments))
	parts := make([]string, 0, len(segments))
	remaining := maxRunes
	appendSegment := func(index int, categoryBudget *int) {
		if index < 0 || index >= len(segments) || remaining <= 0 || *categoryBudget <= 0 {
			return
		}
		if _, exists := selected[index]; exists {
			return
		}
		formatted := formatConversationSegment(segments[index])
		limit := min(remaining, *categoryBudget)
		formatted = TrimRunes(formatted, limit)
		if strings.TrimSpace(formatted) == "" {
			return
		}
		parts = append(parts, formatted)
		used := utf8.RuneCountInString(formatted)
		remaining -= used
		*categoryBudget -= used
		selected[index] = struct{}{}
	}
	userBudget := conversationLatestUserRunes
	appendSegment(latestUserIndex, &userBudget)
	adjacentBudget := conversationAdjacentRunes
	for distance := 1; distance < len(segments) && adjacentBudget > 0 && remaining > 0; distance++ {
		for _, index := range []int{latestUserIndex - distance, latestUserIndex + distance} {
			if index < 0 || index >= len(segments) || !isAdjacentConversationRole(segments[index].role) {
				continue
			}
			appendSegment(index, &adjacentBudget)
		}
	}
	systemBudget := conversationSystemRunes
	for index, segment := range segments {
		role := strings.ToLower(strings.TrimSpace(segment.role))
		if role == "system" || role == "developer" {
			appendSegment(index, &systemBudget)
		}
	}
	metadataBudget := conversationMetadataRunes
	for index := len(segments) - 1; index >= 0; index-- {
		appendSegment(index, &metadataBudget)
	}
	return TrimRunes(strings.Join(parts, "\n\n"), maxRunes)
}

func isAdjacentConversationRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant", "model", "tool", "function":
		return true
	default:
		return false
	}
}

func ExtractPromptSnapshot(req Request) (PromptSnapshot, error) {
	return extractPromptSnapshot(req, false)
}

// ExtractBlockingPromptSnapshot builds the narrow, low-latency blocking input
// when configured. Asynchronous auditing always uses ExtractPromptSnapshot so
// the complete client-controlled transcript is retained for review.
func ExtractBlockingPromptSnapshot(req Request, latestTurnOnly bool) (PromptSnapshot, error) {
	return extractPromptSnapshot(req, latestTurnOnly)
}

func extractPromptSnapshot(req Request, latestTurnOnly bool) (PromptSnapshot, error) {
	var document any
	if err := json.Unmarshal(req.Body, &document); err != nil {
		return PromptSnapshot{}, errors.New("prompt audit request JSON is invalid")
	}
	extracted := extractProtocolSegments(req.Protocol, document)
	segments := normalizeSegmentsLatestUserFirst(extracted)
	if latestTurnOnly {
		segments = blockingSegmentsLatestUserAndPreviousOutput(extracted)
	}
	if len(segments) == 0 {
		return PromptSnapshot{}, ErrNoPromptText
	}
	scanText, metadataText := buildPrioritizedScanText(segments)
	digest := sha256.Sum256([]byte(metadataText))
	stage := strings.TrimSpace(req.Stage)
	if stage == "" {
		stage = "http"
	}
	return PromptSnapshot{
		RequestID: req.RequestID, UserID: req.UserID, UsernameSnapshot: req.Username,
		UserEmailSnapshot: req.UserEmail, APIKeyID: req.APIKeyID, APIKeyNameSnapshot: req.APIKeyName,
		GroupID: cloneInt64Ptr(req.GroupID), GroupName: req.GroupName, Provider: req.Provider,
		Endpoint: req.Endpoint, Protocol: req.Protocol, Model: req.Model,
		PromptHash: hex.EncodeToString(digest[:]), RedactedPreview: BuildPromptPreview(metadataText, DefaultPromptPreviewMaxRunes),
		FullPrompt:   BuildFullPrompt(metadataText, DefaultFullPromptMaxRunes),
		PromptLength: utf8.RuneCountInString(metadataText), MessageCount: len(segments), Stage: stage,
		ScanText: scanText,
	}, nil
}

// DefaultPromptPreviewMaxRunes caps how much sanitized prompt text may be
// considered before BuildPromptPreview withholds the majority for storage/UI.
const DefaultPromptPreviewMaxRunes = 96

// DefaultFullPromptMaxRunes caps how much unredacted prompt text is persisted
// on an audit event for admin review. It is deliberately generous so realistic
// prompts are kept intact while bounding per-row storage.
const DefaultFullPromptMaxRunes = 65536

func extractProtocolSegments(protocol string, document any) []promptSegment {
	root, _ := document.(map[string]any)
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	switch protocol {
	case "openai_chat_completions", "openai_chat", "chat_completions":
		return extractChatLikeSegments(root)
	case "anthropic_messages", "claude_messages", "messages":
		return append(extractAnthropicSystem(root["system"]), extractMessages(root["messages"], clientInstructionRoles...)...)
	case "gemini", "gemini_generate_content":
		return extractGeminiRoot(root)
	case "openai_responses", "responses", "responses_websocket":
		if frameType := stringValue(root["type"]); frameType != "" || protocol == "responses_websocket" {
			if frameType != "response.create" {
				return nil
			}
			if input, exists := root["input"]; exists && input != nil {
				return append(extractInstructions(root["instructions"]), extractResponses(input)...)
			}
			if response, ok := root["response"].(map[string]any); ok {
				return append(extractInstructions(response["instructions"]), extractResponses(response["input"])...)
			}
			return extractInstructions(root["instructions"])
		}
		return append(extractInstructions(root["instructions"]), extractResponses(root["input"])...)
	case "openai_images", "grok_media", "media", "images":
		return userPromptSegments(extractMediaPrompts(root))
	default:
		if segments := extractChatLikeSegments(root); len(segments) > 0 {
			return segments
		}
		if responses := append(extractInstructions(root["instructions"]), extractResponses(root["input"])...); len(responses) > 0 {
			return responses
		}
		if gemini := extractGeminiRoot(root); len(gemini) > 0 {
			return gemini
		}
		return userPromptSegments(extractMediaPrompts(root))
	}
}

// clientInstructionRoles are roles a client may freely populate. Attackers can
// place jailbreak/PII text in assistant/tool turns, so blocking audit must scan
// them too—not only user/system/developer instructions.
var clientInstructionRoles = []string{"user", "system", "developer", "assistant", "tool"}

func extractChatLikeSegments(root map[string]any) []promptSegment {
	if root == nil {
		return nil
	}
	return extractMessages(root["messages"], clientInstructionRoles...)
}

func extractMessages(value any, wantedRoles ...string) []promptSegment {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	wanted := make(map[string]struct{}, len(wantedRoles))
	for _, role := range wantedRoles {
		wanted[strings.ToLower(strings.TrimSpace(role))] = struct{}{}
	}
	result := make([]promptSegment, 0, len(items))
	for _, item := range items {
		message, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role := strings.ToLower(stringValue(message["role"]))
		if _, match := wanted[role]; !match {
			continue
		}
		texts := contentTexts(message["content"])
		for _, text := range texts {
			result = append(result, promptSegment{text: text, user: role == "user", role: role})
		}
	}
	return result
}

func extractInstructions(value any) []promptSegment {
	switch typed := value.(type) {
	case string:
		if text := strings.TrimSpace(typed); text != "" {
			return []promptSegment{{text: text, role: "system"}}
		}
	case []any:
		return systemPromptSegments(contentTexts(typed))
	case map[string]any:
		return systemPromptSegments(contentTexts(typed))
	}
	return nil
}

func extractAnthropicSystem(value any) []promptSegment {
	switch typed := value.(type) {
	case string:
		if text := strings.TrimSpace(typed); text != "" {
			return []promptSegment{{text: text, role: "system"}}
		}
	case []any:
		return systemPromptSegments(contentTexts(typed))
	case map[string]any:
		return systemPromptSegments(contentTexts(typed))
	}
	return nil
}

func extractResponses(value any) []promptSegment {
	switch typed := value.(type) {
	case string:
		return []promptSegment{{text: typed, user: true, role: "user"}}
	case []any:
		result := make([]promptSegment, 0, len(typed))
		for _, item := range typed {
			switch entry := item.(type) {
			case string:
				result = append(result, promptSegment{text: entry, user: true, role: "user"})
			case map[string]any:
				role := strings.ToLower(stringValue(entry["role"]))
				if role == "" {
					role = inferredResponsesItemRole(strings.ToLower(stringValue(entry["type"])))
				}
				if role != "" && !isClientInstructionRole(role) {
					continue
				}
				for _, text := range responseItemPromptTexts(entry) {
					result = append(result, promptSegment{text: text, user: role == "" || role == "user", role: role})
				}
			}
		}
		return result
	case map[string]any:
		role := strings.ToLower(stringValue(typed["role"]))
		if role == "" {
			role = inferredResponsesItemRole(strings.ToLower(stringValue(typed["type"])))
		}
		if role != "" && !isClientInstructionRole(role) {
			return nil
		}
		texts := responseItemPromptTexts(typed)
		return promptSegmentsForRole(texts, role)
	default:
		return nil
	}
}

func inferredResponsesItemRole(typeName string) string {
	switch strings.ToLower(strings.TrimSpace(typeName)) {
	case "function_call_output", "custom_tool_call_output", "computer_call_output", "tool_result":
		return "tool"
	case "function_call", "custom_tool_call", "computer_call", "web_search_call":
		return "assistant"
	default:
		return ""
	}
}

// responseItemPromptTexts covers the text-bearing fields used by Responses
// tool calls. Keeping these fields in the role-preserving snapshot prevents a
// cyber event from losing the actual tool output or call arguments merely
// because the item has no content/text property.
func responseItemPromptTexts(entry map[string]any) []string {
	texts := make([]string, 0, 3)
	if content, exists := entry["content"]; exists {
		texts = append(texts, contentTexts(content)...)
	}
	if text := stringValue(entry["text"]); text != "" {
		texts = append(texts, text)
	}
	for _, field := range []string{"output", "arguments"} {
		texts = append(texts, responseItemValueTexts(entry[field])...)
	}
	return texts
}

func responseItemValueTexts(value any) []string {
	switch typed := value.(type) {
	case string:
		if text := strings.TrimSpace(typed); text != "" {
			return []string{typed}
		}
	case []any:
		texts := make([]string, 0, len(typed))
		for _, item := range typed {
			texts = append(texts, responseItemValueTexts(item)...)
		}
		return texts
	case map[string]any:
		if text := stringValue(typed["text"]); text != "" {
			return []string{text}
		}
		if content, exists := typed["content"]; exists {
			if texts := responseItemValueTexts(content); len(texts) > 0 {
				return texts
			}
		}
		// Structured arguments/results are valid Responses payloads even when
		// they do not have a text/content field. Canonical JSON keeps the
		// evidence deterministic while retaining the actual tool data.
		if encoded, err := json.Marshal(typed); err == nil {
			return []string{string(encoded)}
		}
	}
	return nil
}

func isClientInstructionRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "user", "system", "developer", "assistant", "tool", "model":
		return true
	default:
		return false
	}
}

func extractGemini(value any) []promptSegment {
	var contents []any
	switch typed := value.(type) {
	case []any:
		contents = typed
	case map[string]any:
		contents = []any{typed}
	default:
		return nil
	}
	result := make([]promptSegment, 0, len(contents))
	for _, item := range contents {
		content, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role := strings.ToLower(stringValue(content["role"]))
		if role != "" && !isClientInstructionRole(role) {
			continue
		}
		parts, _ := content["parts"].([]any)
		for _, part := range parts {
			if object, ok := part.(map[string]any); ok {
				if text := stringValue(object["text"]); text != "" {
					result = append(result, promptSegment{text: text, user: role == "" || role == "user", role: role})
				}
			}
		}
	}
	return result
}

func extractGeminiRoot(root map[string]any) []promptSegment {
	if root == nil {
		return nil
	}
	result := extractGeminiSystemInstruction(root["systemInstruction"])
	result = append(result, extractGeminiSystemInstruction(root["system_instruction"])...)
	result = append(result, extractGemini(root["contents"])...)
	result = append(result, extractGemini(root["content"])...)
	result = append(result, extractGeminiInstances(root["instances"])...)
	if requests, ok := root["requests"].([]any); ok {
		for _, item := range requests {
			request, ok := item.(map[string]any)
			if !ok {
				continue
			}
			result = append(result, extractGeminiSystemInstruction(request["systemInstruction"])...)
			result = append(result, extractGeminiSystemInstruction(request["system_instruction"])...)
			result = append(result, extractGemini(request["contents"])...)
			result = append(result, extractGemini(request["content"])...)
			result = append(result, extractGeminiInstances(request["instances"])...)
		}
	}
	return result
}

func extractGeminiSystemInstruction(value any) []promptSegment {
	switch typed := value.(type) {
	case string:
		if text := strings.TrimSpace(typed); text != "" {
			return []promptSegment{{text: text, role: "system"}}
		}
	case map[string]any:
		if parts, ok := typed["parts"].([]any); ok {
			result := make([]promptSegment, 0, len(parts))
			for _, part := range parts {
				if object, ok := part.(map[string]any); ok {
					if text := stringValue(object["text"]); text != "" {
						result = append(result, promptSegment{text: text, role: "system"})
					}
				}
			}
			return result
		}
		return systemPromptSegments(contentTexts(typed))
	case []any:
		segments := extractGemini(typed)
		for index := range segments {
			segments[index].user = false
			segments[index].role = "system"
		}
		return segments
	}
	return nil
}

func extractGeminiInstances(value any) []promptSegment {
	instances, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]promptSegment, 0, len(instances))
	for _, item := range instances {
		if instance, ok := item.(map[string]any); ok {
			if prompt := stringValue(instance["prompt"]); prompt != "" {
				result = append(result, promptSegment{text: prompt, user: true, role: "user"})
			}
		}
	}
	return result
}

func extractMediaPrompts(root map[string]any) []string {
	if root == nil {
		return nil
	}
	result := make([]string, 0, 4)
	seen := map[string]struct{}{}
	var walk func(any, string)
	walk = func(value any, key string) {
		switch typed := value.(type) {
		case map[string]any:
			keys := make([]string, 0, len(typed))
			for childKey := range typed {
				keys = append(keys, childKey)
			}
			sort.Strings(keys)
			for _, childKey := range keys {
				walk(typed[childKey], childKey)
			}
		case []any:
			for _, item := range typed {
				walk(item, key)
			}
		case string:
			if !isMediaPromptKey(key) || looksLikeMediaPayload(typed) {
				return
			}
			text := strings.TrimSpace(typed)
			if text == "" {
				return
			}
			if _, duplicate := seen[text]; duplicate {
				return
			}
			seen[text] = struct{}{}
			result = append(result, text)
		}
	}
	walk(root, "")
	return result
}

func isMediaPromptKey(key string) bool {
	normalized := strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(strings.TrimSpace(key)))
	switch normalized {
	case "prompt", "inputprompt", "textprompt", "description", "query", "lyrics", "negativeprompt",
		"positiveprompt", "gptdescriptionprompt", "prompten", "finalprompt", "finalzhprompt",
		"origprompt", "actualprompt", "imageprompt", "input":
		return true
	default:
		return false
	}
}

func looksLikeMediaPayload(value string) bool {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "data:image/") || strings.HasPrefix(lower, "data:video/") ||
		strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return true
	}
	if len(trimmed) >= 256 {
		for _, r := range trimmed {
			alphaNumeric := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
			if !alphaNumeric && r != '+' && r != '/' && r != '=' {
				return false
			}
		}
		return true
	}
	return false
}

func contentTexts(value any) []string {
	switch typed := value.(type) {
	case string:
		return []string{typed}
	case []any:
		result := make([]string, 0, len(typed))
		for _, part := range typed {
			object, ok := part.(map[string]any)
			if !ok {
				continue
			}
			typeName := strings.ToLower(stringValue(object["type"]))
			if typeName != "" && typeName != "text" && typeName != "input_text" && typeName != "output_text" {
				continue
			}
			if text := stringValue(object["text"]); text != "" {
				result = append(result, text)
			}
		}
		return result
	case map[string]any:
		if text := stringValue(typed["text"]); text != "" {
			return []string{text}
		}
	}
	return nil
}

func normalizeSegmentsLatestUserFirst(values []promptSegment) []string {
	normalized := normalizedPromptSegments(values)
	if len(normalized) == 0 {
		return nil
	}
	priorityIndex := len(normalized) - 1
	for index := len(normalized) - 1; index >= 0; index-- {
		if isUserSegment(normalized[index]) {
			priorityIndex = index
			break
		}
	}
	result := make([]string, 0, len(normalized))
	result = append(result, normalized[priorityIndex].text)
	for index, segment := range normalized {
		if index != priorityIndex {
			result = append(result, segment.text)
		}
	}
	return result
}

// blockingSegmentsLatestUserAndPreviousOutput limits synchronous guard input to
// the current user turn and the nearest preceding assistant/model turn. It is
// deliberately opt-in because full transcript scanning remains stronger at
// finding client-controlled content placed in older or non-user messages.
func blockingSegmentsLatestUserAndPreviousOutput(values []promptSegment) []string {
	normalized := normalizedPromptSegments(values)
	latestUserStart := latestUserSegmentStart(normalized)
	if latestUserStart < 0 {
		// A request without user content cannot be narrowed safely. Preserve the
		// established full-snapshot behavior for unusual protocol payloads.
		return normalizeSegmentsLatestUserFirst(values)
	}
	latestUserEnd := latestUserStart
	for latestUserEnd < len(normalized) && isUserSegment(normalized[latestUserEnd]) {
		latestUserEnd++
	}
	currentUserText := make([]string, 0, latestUserEnd-latestUserStart)
	for _, segment := range normalized[latestUserStart:latestUserEnd] {
		currentUserText = append(currentUserText, segment.text)
	}
	// A single client turn may have several text content parts. Keep it in one
	// priority segment so every part of the latest input is scanned before the
	// prior output begins.
	selected := []promptSegment{{text: strings.Join(currentUserText, "\n\n"), user: true, role: "user"}}
	for index := latestUserStart - 1; index >= 0; index-- {
		if !isAssistantOutputSegment(normalized[index]) {
			continue
		}
		start := index
		for start > 0 && isAssistantOutputSegment(normalized[start-1]) {
			start--
		}
		selected = append(selected, normalized[start:index+1]...)
		break
	}
	return promptSegmentTexts(selected)
}

func normalizedPromptSegments(values []promptSegment) []promptSegment {
	normalized := make([]promptSegment, 0, len(values))
	for _, value := range values {
		value.text = strings.TrimSpace(value.text)
		if value.text != "" {
			normalized = append(normalized, value)
		}
	}
	return normalized
}

func latestUserSegmentStart(values []promptSegment) int {
	latest := -1
	for index := len(values) - 1; index >= 0; index-- {
		if isUserSegment(values[index]) {
			latest = index
			break
		}
	}
	for latest > 0 && isUserSegment(values[latest-1]) {
		latest--
	}
	return latest
}

func isUserSegment(segment promptSegment) bool {
	return segment.user || segment.role == "user"
}

func isAssistantOutputSegment(segment promptSegment) bool {
	return segment.role == "assistant" || segment.role == "model"
}

func promptSegmentTexts(values []promptSegment) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.text)
	}
	return result
}

func buildPrioritizedScanText(segments []string) (scanText string, metadataText string) {
	metadataText = strings.Join(segments, "\n\n")
	if len(segments) <= 1 {
		return metadataText, metadataText
	}
	return segments[0] + promptAuditPrioritySeparator + strings.Join(segments[1:], "\n\n"), metadataText
}

func promptSegmentsForRole(texts []string, role string) []promptSegment {
	result := make([]promptSegment, 0, len(texts))
	for _, text := range texts {
		result = append(result, promptSegment{text: text, user: role == "" || role == "user", role: role})
	}
	return result
}

func userPromptSegments(texts []string) []promptSegment {
	return promptSegmentsForRole(texts, "user")
}

func systemPromptSegments(texts []string) []promptSegment {
	return promptSegmentsForRole(texts, "system")
}

func RedactPreview(value string, maxRunes int) string {
	value = bearerPattern.ReplaceAllString(value, "Bearer ***")
	value = apiKeyPattern.ReplaceAllStringFunc(value, func(match string) string {
		if index := strings.IndexAny(match, ":= \t"); index >= 0 {
			return match[:index+1] + "***"
		}
		return "***"
	})
	value = canaryPattern.ReplaceAllString(value, "${1}***")
	value = emailPattern.ReplaceAllString(value, "***@***")
	value = phonePattern.ReplaceAllString(value, "***PHONE***")
	return TrimRunes(value, maxRunes)
}

// BuildPromptPreview stores only a short, non-recoverable head of sanitized
// input. Ordinary confidential prompts must not land nearly intact in PostgreSQL
// or the admin UI merely because no secret regex matched.
func BuildPromptPreview(value string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = DefaultPromptPreviewMaxRunes
	}
	redacted := strings.TrimSpace(RedactPreview(value, maxRunes))
	if redacted == "" {
		return ""
	}
	runes := []rune(redacted)
	hadTruncation := strings.HasSuffix(redacted, "…")
	if hadTruncation && len(runes) > 0 {
		runes = runes[:len(runes)-1]
	}
	if len(runes) == 0 {
		return "***…"
	}
	// Short unlabelled secrets would otherwise leak a recoverable prefix (e.g.
	// 20 runes → 5 visible). Fully withhold anything below the keep threshold.
	const minLengthForPartialPreview = 32
	if len(runes) < minLengthForPartialPreview {
		if hadTruncation {
			return "***…"
		}
		return "***"
	}
	// Keep at most a quarter of the already-truncated text, and never more than
	// 24 runes, so the majority of prompt content is withheld by default.
	keep := len(runes) / 4
	if keep > 24 {
		keep = 24
	}
	preview := string(runes[:keep]) + "***"
	if hadTruncation || keep < len(runes) {
		preview += "…"
	}
	return preview
}

// BuildFullPrompt returns the complete prompt text for audit-event storage and
// admin review, without redaction. NUL bytes are stripped because PostgreSQL
// TEXT rejects them, and the result is capped at maxRunes.
func BuildFullPrompt(value string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = DefaultFullPromptMaxRunes
	}
	value = strings.ReplaceAll(value, "\x00", "")
	return TrimRunes(strings.TrimSpace(value), maxRunes)
}

// FullPromptFromScanText reconstructs the display prompt from the worker scan
// payload. buildPrioritizedScanText inserts exactly one priority separator
// between the prioritized segment and the remainder, so replacing it with the
// metadata joiner yields the original multi-segment text.
func FullPromptFromScanText(scanText string) string {
	return BuildFullPrompt(strings.ReplaceAll(scanText, promptAuditPrioritySeparator, "\n\n"), DefaultFullPromptMaxRunes)
}

func TrimRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
