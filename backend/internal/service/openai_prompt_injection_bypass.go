package service

import "github.com/gin-gonic/gin"

// ShouldBypassOpenAIPromptInjection reports whether the authenticated API Key
// is configured to receive no gateway-generated OpenAI/Codex prompt text.
// The lookup is an in-memory slice scan over startup configuration; it never
// queries a repository or database on the request path.
func (s *OpenAIGatewayService) ShouldBypassOpenAIPromptInjection(apiKeyID int64) bool {
	if s == nil || s.cfg == nil || apiKeyID <= 0 {
		return false
	}
	for _, configuredID := range s.cfg.Gateway.OpenAIPromptInjectionBypassAPIKeyIDs {
		if configuredID > 0 && configuredID == apiKeyID {
			return true
		}
	}
	return false
}

func (s *OpenAIGatewayService) shouldBypassOpenAIPromptInjection(c *gin.Context) bool {
	return s.ShouldBypassOpenAIPromptInjection(getAPIKeyIDFromContext(c))
}
