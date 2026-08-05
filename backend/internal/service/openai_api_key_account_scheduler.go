package service

import (
	"context"
	"log/slog"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

// selectOpenAIAPIKeyAccountRoute applies an API Key-specific OpenAI account
// override before sticky/load-balanced scheduling. Any route error or account
// incompatibility abandons the override and lets the normal scheduler proceed.
func (s *OpenAIGatewayService) selectOpenAIAPIKeyAccountRoute(
	ctx context.Context,
	req OpenAIAccountScheduleRequest,
) (*AccountSelectionResult, bool, error) {
	if s == nil || s.settingService == nil || normalizeOpenAICompatiblePlatform(req.Platform) != PlatformOpenAI {
		return nil, false, nil
	}
	apiKeyID, ok := ctx.Value(ctxkey.APIKeyID).(int64)
	if !ok || apiKeyID <= 0 {
		return nil, false, nil
	}
	accountID, configured, err := s.settingService.GetOpenAIAPIKeyAccountRoute(ctx, apiKeyID)
	if err != nil {
		slog.Warn("openai api key account route skipped; using normal scheduling", "api_key_id", apiKeyID, "reason", "configuration_error", "error", err)
		return nil, false, nil
	}
	if !configured {
		return nil, false, nil
	}
	if req.ExcludedIDs != nil {
		if _, excluded := req.ExcludedIDs[accountID]; excluded {
			return nil, false, nil
		}
	}

	account, err := s.getSchedulableAccount(ctx, accountID)
	if err != nil || account == nil {
		return nil, false, nil
	}
	checker := &defaultOpenAIAccountScheduler{service: s}
	if !checker.isAccountRequestCompatible(ctx, account, req) ||
		!checker.isAccountTransportCompatible(account, req.RequiredTransport) {
		return nil, false, nil
	}
	account = s.recheckSelectedOpenAIAccountFromDBForForcedRoute(ctx, account, req.Platform, req.RequestedModel, req.RequireCompact, req.RequiredCapability)
	if account == nil ||
		!checker.isAccountTransportCompatible(account, req.RequiredTransport) ||
		!checker.isAccountRequestCompatible(ctx, account, req) {
		return nil, false, nil
	}
	if req.RequireCompact && openAICompactSupportTier(account) == 0 {
		return nil, false, nil
	}

	result, err := s.tryAcquireAccountSlot(ctx, account.ID, account.Concurrency)
	if err != nil || result == nil || !result.Acquired {
		return nil, false, nil
	}
	selection, err := s.newAcquiredSelectionResult(ctx, account, result.ReleaseFunc)
	if err != nil || selection == nil || selection.Account == nil {
		return nil, false, nil
	}
	return selection, true, nil
}
