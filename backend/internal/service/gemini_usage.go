package service

import "github.com/tidwall/gjson"

// mergeGeminiUsageMetadata merges cumulative Gemini usage chunks without
// treating an omitted field as an authoritative zero.
func mergeGeminiUsageMetadata(dst *ClaudeUsage, data []byte) bool {
	if dst == nil {
		return false
	}

	metadata := gjson.GetBytes(data, "usageMetadata")
	if !metadata.Exists() {
		return false
	}

	promptResult := metadata.Get("promptTokenCount")
	cachedResult := metadata.Get("cachedContentTokenCount")
	if promptResult.Exists() || cachedResult.Exists() {
		promptTokens := dst.InputTokens + dst.CacheReadInputTokens
		cachedTokens := dst.CacheReadInputTokens
		if promptResult.Exists() {
			promptTokens = nonNegativeGeminiTokens(promptResult.Int())
		}
		if cachedResult.Exists() {
			cachedTokens = nonNegativeGeminiTokens(cachedResult.Int())
		}
		if promptTokens < cachedTokens {
			promptTokens = cachedTokens
		}
		dst.InputTokens = promptTokens - cachedTokens
		dst.CacheReadInputTokens = cachedTokens
	}

	// Gemini reports cumulative output counters. Taking the greatest complete
	// snapshot retains thoughts when a later partial chunk omits them.
	candidateResult := metadata.Get("candidatesTokenCount")
	thoughtsResult := metadata.Get("thoughtsTokenCount")
	if candidateResult.Exists() || thoughtsResult.Exists() {
		outputTokens := nonNegativeGeminiTokens(candidateResult.Int()) + nonNegativeGeminiTokens(thoughtsResult.Int())
		if outputTokens > dst.OutputTokens {
			dst.OutputTokens = outputTokens
		}
	}

	candidateDetails := metadata.Get("candidatesTokensDetails")
	if candidateDetails.Exists() {
		candidateDetails.ForEach(func(_, detail gjson.Result) bool {
			if detail.Get("modality").String() == "IMAGE" {
				imageTokens := nonNegativeGeminiTokens(detail.Get("tokenCount").Int())
				if imageTokens > dst.ImageOutputTokens {
					dst.ImageOutputTokens = imageTokens
				}
				return false
			}
			return true
		})
	}

	return true
}

func extractGeminiUsage(data []byte) *ClaudeUsage {
	usage := &ClaudeUsage{}
	if !mergeGeminiUsageMetadata(usage, data) {
		return nil
	}
	return usage
}

func nonNegativeGeminiTokens(value int64) int {
	if value <= 0 {
		return 0
	}
	return int(value)
}

func hasGeminiTokenUsage(usage *ClaudeUsage) bool {
	return usage != nil && (usage.InputTokens > 0 || usage.OutputTokens > 0 || usage.CacheReadInputTokens > 0)
}

func geminiClaudeUsageMap(usage *ClaudeUsage) map[string]any {
	if usage == nil {
		usage = &ClaudeUsage{}
	}
	result := map[string]any{
		"input_tokens":  usage.InputTokens,
		"output_tokens": usage.OutputTokens,
	}
	if usage.CacheReadInputTokens > 0 {
		result["cache_read_input_tokens"] = usage.CacheReadInputTokens
	}
	return result
}

func applyGeminiClaudeUsage(response map[string]any, usage *ClaudeUsage) {
	if response == nil || usage == nil {
		return
	}
	response["usage"] = geminiClaudeUsageMap(usage)
}
