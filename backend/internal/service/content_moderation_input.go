package service

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/tidwall/gjson"
)

func ExtractContentModerationText(protocol string, body []byte) string {
	return ExtractContentModerationInput(protocol, body).Text
}

func ExtractContentModerationInput(protocol string, body []byte) ContentModerationInput {
	return extractContentModerationInputs(protocol, body).User
}

// ModerationSegment preserves provenance so observation rules never need to
// infer trust from text markers inside a flattened prompt.
type ModerationSegment struct {
	Role        string
	Source      string
	Text        string
	Enforceable bool
}

const (
	moderationRoleUser        = "user"
	moderationRoleAssistant   = "assistant"
	moderationRoleTool        = "tool"
	moderationRoleSystem      = "system"
	moderationRoleDeveloper   = "developer"
	moderationRoleEnvironment = "environment"
	moderationRoleAmbiguous   = "ambiguous"
)

// contentModerationExtractedInputs separates locally enforceable content from
// content that must be sent to the moderation API. Segments retain all
// reviewable client text without flattening roles and sources together.
type contentModerationExtractedInputs struct {
	User        ContentModerationInput
	Audit       ContentModerationInput
	Segments    []ModerationSegment
	RequiresAPI bool
}

func extractContentModerationInputs(protocol string, body []byte) contentModerationExtractedInputs {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return contentModerationExtractedInputs{}
	}
	var parts []string
	var images []string
	switch protocol {
	case ContentModerationProtocolAnthropicMessages:
		collectLastAnthropicUserMessage(gjson.GetBytes(body, "messages"), &parts, &images)
	case ContentModerationProtocolOpenAIChat:
		collectLastRoleMessage(gjson.GetBytes(body, "messages"), "user", &parts, &images)
	case ContentModerationProtocolOpenAIResponses:
		collectLastResponsesInput(responsesModerationInput(body), &parts, &images)
	case ContentModerationProtocolGemini:
		collectLastGeminiContent(gjson.GetBytes(body, "contents"), &parts, &images)
	case ContentModerationProtocolOpenAIImages:
		addModerationText(&parts, gjson.GetBytes(body, "prompt").String())
		collectContentValue(gjson.GetBytes(body, "images"), &parts, &images)
	default:
		collectLastResponsesInput(responsesModerationInput(body), &parts, &images)
		collectLastRoleMessage(gjson.GetBytes(body, "messages"), "user", &parts, &images)
		collectLastGeminiContent(gjson.GetBytes(body, "contents"), &parts, &images)
	}
	user := ContentModerationInput{
		Text:   normalizeContentModerationText(strings.Join(parts, "\n")),
		Images: normalizeModerationImages(images),
	}
	user.Normalize()
	segments := collectModerationSegments(protocol, body)
	audit := user
	requiresAPI := false
	if audit.IsEmpty() {
		audit, requiresAPI = collectAmbiguousModerationInput(protocol, body)
	}
	audit.Normalize()
	return contentModerationExtractedInputs{User: user, Audit: audit, Segments: segments, RequiresAPI: requiresAPI}
}

func responsesModerationInput(body []byte) gjson.Result {
	if input := gjson.GetBytes(body, "input"); input.Exists() {
		return input
	}
	return gjson.GetBytes(body, "response.input")
}

func collectLastRoleMessage(messages gjson.Result, role string, parts *[]string, images *[]string) {
	if !messages.IsArray() {
		return
	}
	array := messages.Array()
	if len(array) == 0 {
		return
	}
	last := array[len(array)-1]
	if strings.ToLower(strings.TrimSpace(last.Get("role").String())) != role {
		return
	}
	var candidate []string
	var candidateImages []string
	collectContentValue(last.Get("content"), &candidate, &candidateImages)
	cleaned, _ := splitSyntheticModerationEnvelope(strings.Join(candidate, "\n"))
	if normalizeContentModerationText(cleaned) == "" && len(candidateImages) == 0 {
		return
	}
	addModerationText(parts, cleaned)
	*images = append(*images, candidateImages...)
}

func collectLastAnthropicUserMessage(messages gjson.Result, parts *[]string, images *[]string) {
	if !messages.IsArray() {
		return
	}
	array := messages.Array()
	if len(array) == 0 {
		return
	}
	last := array[len(array)-1]
	if strings.ToLower(strings.TrimSpace(last.Get("role").String())) != "user" {
		return
	}
	var candidate []string
	var candidateImages []string
	collectAnthropicUserContentValue(last.Get("content"), &candidate, &candidateImages)
	cleaned, _ := splitSyntheticModerationEnvelope(strings.Join(candidate, "\n"))
	if normalizeContentModerationText(cleaned) == "" && len(candidateImages) == 0 {
		return
	}
	addModerationText(parts, cleaned)
	*images = append(*images, candidateImages...)
}

func collectAnthropicUserContentValue(value gjson.Result, parts *[]string, images *[]string) {
	switch {
	case !value.Exists():
		return
	case value.Type == gjson.String:
		addModerationText(parts, value.String())
	case value.IsArray():
		value.ForEach(func(_, item gjson.Result) bool {
			collectAnthropicUserContentValue(item, parts, images)
			return true
		})
	case value.IsObject():
		typ := strings.ToLower(strings.TrimSpace(value.Get("type").String()))
		switch typ {
		case "", "text", "input_text", "message":
			if value.Get("text").Exists() {
				addModerationText(parts, value.Get("text").String())
			}
			if value.Get("content").Exists() {
				collectAnthropicUserContentValue(value.Get("content"), parts, images)
			}
		case "image_url", "input_image", "image":
			collectContentValue(value, parts, images)
		}
	}
}

func collectLastResponsesInput(input gjson.Result, parts *[]string, images *[]string) {
	switch {
	case !input.Exists():
		return
	case input.Type == gjson.String:
		// Official Responses string input is attributable only as ambiguous
		// client input. It is audited by API but never locally hard-blocked.
		return
	case input.IsArray():
		array := input.Array()
		if len(array) == 0 {
			return
		}
		last := array[len(array)-1]
		if !isResponsesUserTextItem(last) {
			return
		}
		var candidate []string
		var candidateImages []string
		collectContentValue(last.Get("content"), &candidate, &candidateImages)
		if last.Get("type").String() == "input_text" || last.Get("text").Exists() {
			collectContentValue(last, &candidate, &candidateImages)
		}
		cleaned, _ := splitSyntheticModerationEnvelope(strings.Join(candidate, "\n"))
		addModerationText(parts, cleaned)
		*images = append(*images, candidateImages...)
	case input.IsObject():
		if isResponsesUserTextItem(input) {
			var candidate []string
			var candidateImages []string
			collectContentValue(input.Get("content"), &candidate, &candidateImages)
			if input.Get("type").String() == "input_text" || input.Get("text").Exists() {
				collectContentValue(input, &candidate, &candidateImages)
			}
			cleaned, _ := splitSyntheticModerationEnvelope(strings.Join(candidate, "\n"))
			addModerationText(parts, cleaned)
			*images = append(*images, candidateImages...)
		}
	}
}

func isResponsesUserTextItem(item gjson.Result) bool {
	role := strings.ToLower(strings.TrimSpace(item.Get("role").String()))
	return role == "user" && responseItemHasModerationText(item)
}

func responseItemHasModerationText(item gjson.Result) bool {
	var parts []string
	var images []string
	collectContentValue(item.Get("content"), &parts, &images)
	if item.Get("type").String() == "input_text" || item.Get("text").Exists() {
		collectContentValue(item, &parts, &images)
	}
	return normalizeContentModerationText(strings.Join(parts, "\n")) != "" || len(images) > 0
}

func collectLastGeminiContent(contents gjson.Result, parts *[]string, images *[]string) {
	if !contents.IsArray() {
		return
	}
	array := contents.Array()
	if len(array) == 0 {
		return
	}
	last := array[len(array)-1]
	role := strings.ToLower(strings.TrimSpace(last.Get("role").String()))
	if role != "" && role != "user" {
		return
	}
	var candidate []string
	var candidateImages []string
	if arr := last.Get("parts"); arr.IsArray() {
		arr.ForEach(func(_, part gjson.Result) bool {
			addModerationText(&candidate, part.Get("text").String())
			addGeminiModerationImage(&candidateImages, part)
			return true
		})
	}
	if normalizeContentModerationText(strings.Join(candidate, "\n")) == "" && len(candidateImages) == 0 {
		return
	}
	cleaned, _ := splitSyntheticModerationEnvelope(strings.Join(candidate, "\n"))
	addModerationText(parts, cleaned)
	*images = append(*images, candidateImages...)
}

func collectAmbiguousModerationInput(protocol string, body []byte) (ContentModerationInput, bool) {
	if protocol != ContentModerationProtocolOpenAIResponses && protocol != ContentModerationProtocolOpenAIResponsesWS && protocol != "" {
		return ContentModerationInput{}, false
	}
	input := responsesModerationInput(body)
	var candidate gjson.Result
	switch {
	case input.Type == gjson.String:
		candidate = input
	case input.IsArray():
		items := input.Array()
		if len(items) == 0 || strings.TrimSpace(items[len(items)-1].Get("role").String()) != "" {
			return ContentModerationInput{}, false
		}
		candidate = items[len(items)-1]
	case input.IsObject() && strings.TrimSpace(input.Get("role").String()) == "":
		candidate = input
	default:
		return ContentModerationInput{}, false
	}
	var parts []string
	var images []string
	if candidate.Type == gjson.String {
		addModerationText(&parts, candidate.String())
	} else {
		collectContentValue(candidate.Get("content"), &parts, &images)
		if candidate.Get("type").String() == "input_text" || candidate.Get("text").Exists() {
			collectContentValue(candidate, &parts, &images)
		}
	}
	out := ContentModerationInput{Text: normalizeContentModerationText(strings.Join(parts, "\n")), Images: normalizeModerationImages(images)}
	out.Normalize()
	return out, !out.IsEmpty()
}

func collectModerationSegments(protocol string, body []byte) []ModerationSegment {
	segments := make([]ModerationSegment, 0, 16)
	switch protocol {
	case ContentModerationProtocolAnthropicMessages:
		appendMessageSegments(&segments, gjson.GetBytes(body, "messages"), "messages")
		appendValueSegments(&segments, gjson.GetBytes(body, "system"), moderationRoleSystem, "system", false)
		appendValueSegments(&segments, gjson.GetBytes(body, "tools"), moderationRoleSystem, "tools", false)
	case ContentModerationProtocolOpenAIChat:
		appendMessageSegments(&segments, gjson.GetBytes(body, "messages"), "messages")
		appendValueSegments(&segments, gjson.GetBytes(body, "tools"), moderationRoleSystem, "tools", false)
	case ContentModerationProtocolOpenAIResponses, ContentModerationProtocolOpenAIResponsesWS:
		appendResponsesSegments(&segments, responsesModerationInput(body))
		instructions := gjson.GetBytes(body, "instructions")
		if !instructions.Exists() {
			instructions = gjson.GetBytes(body, "response.instructions")
		}
		appendValueSegments(&segments, instructions, moderationRoleDeveloper, "instructions", false)
		appendValueSegments(&segments, gjson.GetBytes(body, "tools"), moderationRoleSystem, "tools", false)
	case ContentModerationProtocolGemini:
		appendGeminiSegments(&segments, gjson.GetBytes(body, "contents"))
		appendValueSegments(&segments, gjson.GetBytes(body, "system_instruction"), moderationRoleSystem, "system_instruction", false)
		appendValueSegments(&segments, gjson.GetBytes(body, "systemInstruction"), moderationRoleSystem, "systemInstruction", false)
		appendValueSegments(&segments, gjson.GetBytes(body, "tools"), moderationRoleSystem, "tools", false)
	case ContentModerationProtocolOpenAIImages:
		appendTextSegment(&segments, moderationRoleUser, "prompt", gjson.GetBytes(body, "prompt").String(), true)
	default:
		appendResponsesSegments(&segments, responsesModerationInput(body))
		appendMessageSegments(&segments, gjson.GetBytes(body, "messages"), "messages")
		appendGeminiSegments(&segments, gjson.GetBytes(body, "contents"))
	}
	return segments
}

func appendMessageSegments(segments *[]ModerationSegment, messages gjson.Result, source string) {
	if !messages.IsArray() {
		return
	}
	items := messages.Array()
	for index, message := range items {
		role := normalizeModerationRole(message.Get("role").String())
		if role == "" {
			role = moderationRoleAmbiguous
		}
		enforceable := index == len(items)-1 && role == moderationRoleUser
		base := fmt.Sprintf("%s[%d]", source, index)
		appendMessageContentSegments(segments, message.Get("content"), role, base+".content", enforceable)
		appendValueSegments(segments, message.Get("tool_calls"), moderationRoleAssistant, base+".tool_calls", false)
	}
}

func appendMessageContentSegments(segments *[]ModerationSegment, content gjson.Result, role, source string, enforceable bool) {
	if !content.IsArray() {
		appendValueSegments(segments, content, role, source, enforceable)
		return
	}
	content.ForEach(func(key, item gjson.Result) bool {
		itemRole := role
		itemEnforceable := enforceable
		switch strings.ToLower(strings.TrimSpace(item.Get("type").String())) {
		case "tool_result", "function_call_output", "computer_call_output":
			itemRole = moderationRoleTool
			itemEnforceable = false
		}
		appendValueSegments(segments, item, itemRole, fmt.Sprintf("%s[%s]", source, key.String()), itemEnforceable)
		return true
	})
}

func appendResponsesSegments(segments *[]ModerationSegment, input gjson.Result) {
	switch {
	case !input.Exists():
		return
	case input.Type == gjson.String:
		appendTextSegment(segments, moderationRoleAmbiguous, "input[string]", input.String(), false)
	case input.IsArray():
		items := input.Array()
		for index, item := range items {
			appendResponsesItemSegments(segments, item, fmt.Sprintf("input[%d]", index), index == len(items)-1)
		}
	case input.IsObject():
		appendResponsesItemSegments(segments, input, "input", true)
	}
}

func appendResponsesItemSegments(segments *[]ModerationSegment, item gjson.Result, source string, last bool) {
	role := normalizeModerationRole(item.Get("role").String())
	typ := strings.ToLower(strings.TrimSpace(item.Get("type").String()))
	switch typ {
	case "function_call_output", "tool_result", "computer_call_output":
		role = moderationRoleTool
	case "mcp_approval_request", "mcp_approval_response":
		role = moderationRoleEnvironment
	case "function_call", "computer_call", "web_search_call":
		role = moderationRoleAssistant
	}
	if role == "" {
		role = moderationRoleAmbiguous
	}
	enforceable := last && role == moderationRoleUser
	for _, field := range []string{"content", "text", "output", "arguments"} {
		appendValueSegments(segments, item.Get(field), role, source+"."+field, enforceable)
	}
}

func appendGeminiSegments(segments *[]ModerationSegment, contents gjson.Result) {
	if !contents.IsArray() {
		return
	}
	items := contents.Array()
	for index, content := range items {
		role := normalizeModerationRole(content.Get("role").String())
		if role == "" {
			role = moderationRoleUser
		}
		enforceable := index == len(items)-1 && role == moderationRoleUser
		appendGeminiPartSegments(segments, content.Get("parts"), role, fmt.Sprintf("contents[%d].parts", index), enforceable)
	}
}

func appendGeminiPartSegments(segments *[]ModerationSegment, parts gjson.Result, role, source string, enforceable bool) {
	if !parts.IsArray() {
		appendValueSegments(segments, parts, role, source, enforceable)
		return
	}
	parts.ForEach(func(key, part gjson.Result) bool {
		partRole := role
		partEnforceable := enforceable
		if part.Get("functionResponse").Exists() || part.Get("function_response").Exists() {
			partRole = moderationRoleTool
			partEnforceable = false
		} else if part.Get("functionCall").Exists() || part.Get("function_call").Exists() {
			partRole = moderationRoleAssistant
			partEnforceable = false
		}
		appendValueSegments(segments, part, partRole, fmt.Sprintf("%s[%s]", source, key.String()), partEnforceable)
		return true
	})
}

func normalizeModerationRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "user":
		return moderationRoleUser
	case "assistant", "model":
		return moderationRoleAssistant
	case "tool", "function":
		return moderationRoleTool
	case "system":
		return moderationRoleSystem
	case "developer":
		return moderationRoleDeveloper
	default:
		return ""
	}
}

func appendValueSegments(segments *[]ModerationSegment, value gjson.Result, role, source string, enforceable bool) {
	switch {
	case !value.Exists():
		return
	case value.Type == gjson.String:
		appendTextSegment(segments, role, source, value.String(), enforceable)
	case value.IsArray():
		value.ForEach(func(key, item gjson.Result) bool {
			appendValueSegments(segments, item, role, fmt.Sprintf("%s[%s]", source, key.String()), enforceable)
			return true
		})
	case value.IsObject():
		value.ForEach(func(key, item gjson.Result) bool {
			field := strings.ToLower(strings.TrimSpace(key.String()))
			switch field {
			case "type", "role", "id", "call_id", "tool_call_id", "tool_use_id", "approval_request_id", "name", "url", "image_url", "data", "base64", "media_type", "mediatype", "mime_type", "mimetype":
				return true
			default:
				appendValueSegments(segments, item, role, source+"."+key.String(), enforceable)
				return true
			}
		})
	}
}

func appendTextSegment(segments *[]ModerationSegment, role, source, text string, enforceable bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if role == moderationRoleUser {
		cleaned, envelopes := splitSyntheticModerationEnvelope(text)
		if cleaned = strings.TrimSpace(cleaned); cleaned != "" {
			*segments = append(*segments, ModerationSegment{Role: role, Source: source, Text: cleaned, Enforceable: enforceable})
		}
		for _, envelope := range envelopes {
			*segments = append(*segments, ModerationSegment{Role: moderationRoleEnvironment, Source: source + ".envelope", Text: envelope})
		}
		return
	}
	*segments = append(*segments, ModerationSegment{Role: role, Source: source, Text: text, Enforceable: enforceable})
}

type moderationEnvelopeRange struct {
	start int
	end   int
}

// splitSyntheticModerationEnvelope removes only complete, line-aligned
// envelopes. A marker appearing in ordinary user prose has no special effect.
func splitSyntheticModerationEnvelope(text string) (string, []string) {
	lower := strings.ToLower(text)
	ranges := make([]moderationEnvelopeRange, 0, 6)
	for _, pair := range [][2]string{
		{"<environment_context>", "</environment_context>"},
		{"<system-reminder>", "</system-reminder>"},
		{"<permissions instructions>", "</permissions instructions>"},
		{"<collaboration_mode>", "</collaboration_mode>"},
		{"<plugins_instructions>", "</plugins_instructions>"},
	} {
		ranges = append(ranges, findCompleteModerationEnvelopes(lower, pair[0], pair[1])...)
	}
	for _, current := range findCompleteModerationEnvelopes(lower, "<instructions>", "</instructions>") {
		headerStart, header := previousNonEmptyModerationLine(lower, current.start)
		if header == "# agents.md instructions" {
			current.start = headerStart
			ranges = append(ranges, current)
		}
	}
	if len(ranges) == 0 {
		return text, nil
	}
	for index := range ranges {
		ranges[index].start = originalByteOffsetForLowerOffset(text, lower, ranges[index].start)
		ranges[index].end = originalByteOffsetForLowerOffset(text, lower, ranges[index].end)
	}
	sortModerationEnvelopeRanges(ranges)
	merged := ranges[:0]
	for _, current := range ranges {
		if len(merged) == 0 || current.start > merged[len(merged)-1].end {
			merged = append(merged, current)
			continue
		}
		if current.end > merged[len(merged)-1].end {
			merged[len(merged)-1].end = current.end
		}
	}
	var cleaned strings.Builder
	envelopes := make([]string, 0, len(merged))
	position := 0
	for _, current := range merged {
		cleaned.WriteString(text[position:current.start])
		envelopes = append(envelopes, strings.TrimSpace(text[current.start:current.end]))
		position = current.end
	}
	cleaned.WriteString(text[position:])
	return cleaned.String(), envelopes
}

func previousNonEmptyModerationLine(text string, before int) (int, string) {
	position := strings.LastIndex(text[:before], "\n")
	for position >= 0 {
		lineEnd := position
		previousBreak := strings.LastIndex(text[:lineEnd], "\n")
		lineStart := previousBreak + 1
		line := strings.TrimSpace(text[lineStart:lineEnd])
		if line != "" {
			return lineStart, line
		}
		position = previousBreak
	}
	line := strings.TrimSpace(text[:before])
	return 0, line
}

func findCompleteModerationEnvelopes(lower, open, close string) []moderationEnvelopeRange {
	ranges := make([]moderationEnvelopeRange, 0, 1)
	for offset := 0; offset < len(lower); {
		relative := strings.Index(lower[offset:], open)
		if relative < 0 {
			break
		}
		start := offset + relative
		lineStart := strings.LastIndex(lower[:start], "\n") + 1
		if strings.TrimSpace(lower[lineStart:start]) != "" {
			offset = start + len(open)
			continue
		}
		closeRelative := strings.Index(lower[start+len(open):], close)
		if closeRelative < 0 {
			break
		}
		end := start + len(open) + closeRelative + len(close)
		lineEnd := end
		for lineEnd < len(lower) && lower[lineEnd] != '\n' {
			if lower[lineEnd] != ' ' && lower[lineEnd] != '\t' && lower[lineEnd] != '\r' {
				offset = start + len(open)
				end = -1
				break
			}
			lineEnd++
		}
		if end < 0 {
			continue
		}
		if lineEnd < len(lower) {
			lineEnd++
		}
		ranges = append(ranges, moderationEnvelopeRange{start: lineStart, end: lineEnd})
		offset = lineEnd
	}
	return ranges
}

func sortModerationEnvelopeRanges(ranges []moderationEnvelopeRange) {
	for index := 1; index < len(ranges); index++ {
		current := ranges[index]
		insertAt := index
		for insertAt > 0 && current.start < ranges[insertAt-1].start {
			ranges[insertAt] = ranges[insertAt-1]
			insertAt--
		}
		ranges[insertAt] = current
	}
}

func collectContentValue(value gjson.Result, parts *[]string, images *[]string) {
	switch {
	case !value.Exists():
		return
	case value.Type == gjson.String:
		addModerationText(parts, value.String())
	case value.IsArray():
		value.ForEach(func(_, item gjson.Result) bool {
			collectContentValue(item, parts, images)
			return true
		})
	case value.IsObject():
		typ := strings.ToLower(strings.TrimSpace(value.Get("type").String()))
		addModerationImage(images, value.Get("image_url.url").String())
		addModerationImage(images, value.Get("image_url").String())
		addModerationImage(images, value.Get("url").String())
		addModerationImageData(images, value.Get("source.media_type").String(), value.Get("source.data").String())
		addModerationImageData(images, value.Get("source.mediaType").String(), value.Get("source.data").String())
		addModerationImageData(images, value.Get("media_type").String(), value.Get("data").String())
		addModerationImageData(images, value.Get("mime_type").String(), value.Get("data").String())
		addModerationImageData(images, value.Get("mimeType").String(), value.Get("data").String())
		addModerationImage(images, value.Get("source.data").String())
		addModerationImage(images, value.Get("data").String())
		addModerationImage(images, value.Get("base64").String())
		switch typ {
		case "", "text", "input_text", "message":
			if value.Get("text").Exists() {
				addModerationText(parts, value.Get("text").String())
			}
			if value.Get("content").Exists() {
				collectContentValue(value.Get("content"), parts, images)
			}
		case "image_url", "input_image", "image":
		}
	}
}

func addGeminiModerationImage(images *[]string, part gjson.Result) {
	if inlineData := part.Get("inline_data"); inlineData.IsObject() {
		mimeType := strings.TrimSpace(inlineData.Get("mime_type").String())
		data := strings.TrimSpace(inlineData.Get("data").String())
		if mimeType != "" && data != "" {
			addModerationImage(images, fmt.Sprintf("data:%s;base64,%s", mimeType, data))
		}
	}
	if inlineData := part.Get("inlineData"); inlineData.IsObject() {
		mimeType := strings.TrimSpace(inlineData.Get("mimeType").String())
		data := strings.TrimSpace(inlineData.Get("data").String())
		if mimeType != "" && data != "" {
			addModerationImage(images, fmt.Sprintf("data:%s;base64,%s", mimeType, data))
		}
	}
	addModerationImage(images, part.Get("file_data.file_uri").String())
	addModerationImage(images, part.Get("fileData.fileUri").String())
}

func addModerationImageData(images *[]string, mimeType string, data string) {
	mimeType = strings.TrimSpace(mimeType)
	data = strings.TrimSpace(data)
	if mimeType == "" || data == "" {
		return
	}
	addModerationImage(images, fmt.Sprintf("data:%s;base64,%s", mimeType, data))
}

func addModerationImage(images *[]string, image string) {
	image = strings.TrimSpace(image)
	if image == "" {
		return
	}
	if strings.HasPrefix(image, "data:") || strings.HasPrefix(image, "http://") || strings.HasPrefix(image, "https://") {
		*images = append(*images, image)
	}
}

func normalizeModerationImages(images []string) []string {
	out := make([]string, 0, len(images))
	seen := make(map[string]struct{}, len(images))
	for _, image := range images {
		image = strings.TrimSpace(image)
		if image == "" {
			continue
		}
		if _, ok := seen[image]; ok {
			continue
		}
		seen[image] = struct{}{}
		out = append(out, image)
	}
	return out
}

func limitContentModerationImages(images []string) []string {
	if len(images) <= maxContentModerationInputImages {
		return images
	}
	idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(images))))
	if err != nil {
		return images[:maxContentModerationInputImages]
	}
	return []string{images[int(idx.Int64())]}
}

func addModerationText(parts *[]string, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	*parts = append(*parts, text)
}

func normalizeContentModerationText(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}
