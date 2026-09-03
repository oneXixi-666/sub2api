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

// contentModerationExtractedInputs separates enforceable user content from
// contextual evidence. Contextual evidence remains useful for observing broad
// keyword hits, but must never drive hash, keyword, or moderation API blocking.
type contentModerationExtractedInputs struct {
	User        ContentModerationInput
	Observation ContentModerationInput
}

func extractContentModerationInputs(protocol string, body []byte) contentModerationExtractedInputs {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return contentModerationExtractedInputs{}
	}
	var parts []string
	var images []string
	var observationParts []string
	switch protocol {
	case ContentModerationProtocolAnthropicMessages:
		collectLastAnthropicUserMessage(gjson.GetBytes(body, "messages"), &parts, &images)
		collectMessageObservations(gjson.GetBytes(body, "messages"), &observationParts)
		collectObservationValue(gjson.GetBytes(body, "system"), &observationParts)
		collectObservationValue(gjson.GetBytes(body, "tools"), &observationParts)
	case ContentModerationProtocolOpenAIChat:
		collectLastRoleMessage(gjson.GetBytes(body, "messages"), "user", &parts, &images)
		collectMessageObservations(gjson.GetBytes(body, "messages"), &observationParts)
		collectObservationValue(gjson.GetBytes(body, "tools"), &observationParts)
	case ContentModerationProtocolOpenAIResponses:
		collectLastResponsesInput(gjson.GetBytes(body, "input"), &parts, &images)
		collectResponsesObservations(gjson.GetBytes(body, "input"), &observationParts)
		collectObservationValue(gjson.GetBytes(body, "instructions"), &observationParts)
		collectObservationValue(gjson.GetBytes(body, "tools"), &observationParts)
	case ContentModerationProtocolGemini:
		collectLastGeminiContent(gjson.GetBytes(body, "contents"), &parts, &images)
		collectGeminiObservations(gjson.GetBytes(body, "contents"), &observationParts)
		collectObservationValue(gjson.GetBytes(body, "system_instruction"), &observationParts)
		collectObservationValue(gjson.GetBytes(body, "systemInstruction"), &observationParts)
		collectObservationValue(gjson.GetBytes(body, "tools"), &observationParts)
	case ContentModerationProtocolOpenAIImages:
		addModerationText(&parts, gjson.GetBytes(body, "prompt").String())
		collectContentValue(gjson.GetBytes(body, "images"), &parts, &images)
		addObservationText(&observationParts, gjson.GetBytes(body, "prompt").String())
	default:
		collectLastResponsesInput(gjson.GetBytes(body, "input"), &parts, &images)
		collectLastRoleMessage(gjson.GetBytes(body, "messages"), "user", &parts, &images)
		collectLastGeminiContent(gjson.GetBytes(body, "contents"), &parts, &images)
		collectResponsesObservations(gjson.GetBytes(body, "input"), &observationParts)
		collectMessageObservations(gjson.GetBytes(body, "messages"), &observationParts)
		collectGeminiObservations(gjson.GetBytes(body, "contents"), &observationParts)
		collectObservationValue(gjson.GetBytes(body, "instructions"), &observationParts)
		collectObservationValue(gjson.GetBytes(body, "system"), &observationParts)
		collectObservationValue(gjson.GetBytes(body, "system_instruction"), &observationParts)
		collectObservationValue(gjson.GetBytes(body, "systemInstruction"), &observationParts)
		collectObservationValue(gjson.GetBytes(body, "tools"), &observationParts)
	}
	user := ContentModerationInput{
		Text:   normalizeContentModerationText(strings.Join(parts, "\n")),
		Images: normalizeModerationImages(images),
	}
	user.Normalize()
	observation := ContentModerationInput{
		Text: normalizeContentModerationText(strings.Join(observationParts, "\n")),
	}
	observation.Normalize()
	return contentModerationExtractedInputs{User: user, Observation: observation}
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
	if isSyntheticModerationUserText(strings.Join(candidate, "\n")) {
		return
	}
	if normalizeContentModerationText(strings.Join(candidate, "\n")) == "" && len(candidateImages) == 0 {
		return
	}
	*parts = append(*parts, candidate...)
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
	if isSyntheticModerationUserText(strings.Join(candidate, "\n")) {
		return
	}
	if normalizeContentModerationText(strings.Join(candidate, "\n")) == "" && len(candidateImages) == 0 {
		return
	}
	*parts = append(*parts, candidate...)
	*images = append(*images, candidateImages...)
}

func collectAnthropicUserContentValue(value gjson.Result, parts *[]string, images *[]string) {
	switch {
	case !value.Exists():
		return
	case value.Type == gjson.String:
		if !isAnthropicSystemReminderText(value.String()) {
			addModerationText(parts, value.String())
		}
	case value.IsArray():
		value.ForEach(func(_, item gjson.Result) bool {
			collectAnthropicUserContentValue(item, parts, images)
			return true
		})
	case value.IsObject():
		typ := strings.ToLower(strings.TrimSpace(value.Get("type").String()))
		switch typ {
		case "", "text", "input_text", "message":
			if value.Get("text").Exists() && !isAnthropicSystemReminderText(value.Get("text").String()) {
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

func isAnthropicSystemReminderText(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "<system-reminder>")
}

func collectLastResponsesInput(input gjson.Result, parts *[]string, images *[]string) {
	switch {
	case !input.Exists():
		return
	case input.Type == gjson.String:
		// A top-level string has no independently verifiable source boundary.
		// Codex clients may flatten tool output, approvals, and environment data
		// into it, so retain it for observation only.
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
		if isSyntheticModerationUserText(strings.Join(candidate, "\n")) {
			return
		}
		*parts = append(*parts, candidate...)
		*images = append(*images, candidateImages...)
	case input.IsObject():
		if isResponsesUserTextItem(input) {
			var candidate []string
			var candidateImages []string
			collectContentValue(input.Get("content"), &candidate, &candidateImages)
			if input.Get("type").String() == "input_text" || input.Get("text").Exists() {
				collectContentValue(input, &candidate, &candidateImages)
			}
			if isSyntheticModerationUserText(strings.Join(candidate, "\n")) {
				return
			}
			*parts = append(*parts, candidate...)
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
	if isSyntheticModerationUserText(strings.Join(candidate, "\n")) {
		return
	}
	*parts = append(*parts, candidate...)
	*images = append(*images, candidateImages...)
}

func collectMessageObservations(messages gjson.Result, parts *[]string) {
	if !messages.IsArray() {
		return
	}
	messages.ForEach(func(_, message gjson.Result) bool {
		collectObservationValue(message.Get("content"), parts)
		return true
	})
}

func collectResponsesObservations(input gjson.Result, parts *[]string) {
	switch {
	case !input.Exists():
		return
	case input.Type == gjson.String:
		addObservationText(parts, input.String())
	case input.IsArray():
		input.ForEach(func(_, item gjson.Result) bool {
			collectResponsesObservationItem(item, parts)
			return true
		})
	case input.IsObject():
		collectResponsesObservationItem(input, parts)
	}
}

func collectResponsesObservationItem(item gjson.Result, parts *[]string) {
	for _, field := range []string{"content", "text", "output", "arguments"} {
		collectObservationValue(item.Get(field), parts)
	}
}

func collectGeminiObservations(contents gjson.Result, parts *[]string) {
	if !contents.IsArray() {
		return
	}
	contents.ForEach(func(_, content gjson.Result) bool {
		collectObservationValue(content.Get("parts"), parts)
		return true
	})
}

func collectObservationValue(value gjson.Result, parts *[]string) {
	switch {
	case !value.Exists():
		return
	case value.Type == gjson.String:
		addObservationText(parts, value.String())
	case value.IsArray():
		value.ForEach(func(_, item gjson.Result) bool {
			collectObservationValue(item, parts)
			return true
		})
	case value.IsObject():
		value.ForEach(func(key, item gjson.Result) bool {
			switch strings.ToLower(strings.TrimSpace(key.String())) {
			case "type", "role", "id", "call_id", "tool_call_id", "name",
				"url", "image_url", "data", "base64", "media_type", "mediatype", "mime_type", "mimetype":
				return true
			default:
				collectObservationValue(item, parts)
				return true
			}
		})
	}
}

func isSyntheticModerationUserText(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	for _, marker := range []string{
		"# agents.md instructions",
		"<environment_context>",
		"<system-reminder>",
		"<permissions instructions>",
		"<collaboration_mode>",
		"<plugins_instructions>",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func addObservationText(parts *[]string, text string) {
	text = strings.TrimSpace(text)
	if text != "" {
		*parts = append(*parts, text)
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
	if strings.Contains(text, "<system-reminder>") {
		return
	}
	*parts = append(*parts, text)
}

func normalizeContentModerationText(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}
