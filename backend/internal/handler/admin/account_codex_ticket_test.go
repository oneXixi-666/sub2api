package admin

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountResponseCodexTicketsUsesConfiguredPolicy(t *testing.T) {
	account := &service.Account{ID: 41, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth}
	h := &AccountHandler{cfg: &config.Config{}}
	require.Empty(t, h.accountResponseFromService(account).CodexTurnTickets)
	require.Empty(t, h.accountListResponseFromService(account).CodexTurnTickets)
	h.cfg.Gateway.OpenAICodexTicket = config.OpenAICodexTicketConfig{Enabled: true, Models: []string{"configured-model"}, FailClosed: false}
	status := h.accountListResponseFromService(account).CodexTurnTickets
	require.Len(t, status, 1)
	require.Equal(t, "configured-model", status[0].Model)
	require.False(t, status[0].Blocked)
	h.cfg.Gateway.OpenAICodexTicket.FailClosed = true
	require.True(t, h.accountResponseFromService(account).CodexTurnTickets[0].Blocked)
}

type codexTicketStatusStub struct {
	status []service.OpenAICodexTicketStatus
}

func (s *codexTicketStatusStub) OpenAICodexTicketStatuses(context.Context, *service.Account, time.Time) []service.OpenAICodexTicketStatus {
	return append([]service.OpenAICodexTicketStatus(nil), s.status...)
}

func TestAccountResponseCodexTicketsPreservesLiveStatusAndChangesETag(t *testing.T) {
	account := &service.Account{ID: 41, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth}
	provider := &codexTicketStatusStub{status: []service.OpenAICodexTicketStatus{{
		Model: "gpt-6-astra", HarvestEnabled: true, Attempts: 2, Harvesting: true,
	}}}
	h := &AccountHandler{}
	h.SetCodexTicketStatusProvider(provider)
	require.Equal(t, provider.status, h.accountResponseFromService(account).CodexTurnTickets)
	compact := func() *dto.AccountListItem {
		return dto.AccountListItemFromAccount(h.accountListResponseFromService(account))
	}
	require.Equal(t, provider.status, compact().CodexTurnTickets)
	etag := func() string {
		return buildAccountsListETag([]*dto.AccountListItem{compact()}, 1, 1, 20, "openai", "", "", "", true)
	}
	firstETag := etag()
	provider.status[0].Attempts++
	attemptETag := etag()
	require.NotEqual(t, firstETag, attemptETag, "a new attempt must invalidate the lightweight list cache")
	next := time.Now().Add(6 * time.Second)
	provider.status[0].NextHarvestAt = &next
	deadlineETag := etag()
	require.NotEqual(t, attemptETag, deadlineETag, "a new deadline must invalidate the lightweight list cache")
	require.Equal(t, provider.status, compact().CodexTurnTickets)
	require.Equal(t, deadlineETag, etag(), "unchanged progress keeps a stable ETag")
}

func TestAccountResponseCodexTicketsReadsLiveSettingsAfterRestart(t *testing.T) {
	cfg := &config.Config{}
	repo := &settingHandlerRepoStub{values: map[string]string{service.SettingKeyOpenAICodexTicketEnabled: "true"}}
	settings := service.NewSettingService(repo, cfg)
	h := &AccountHandler{cfg: cfg}
	h.SetCodexTicketSettings(settings)
	account := &service.Account{ID: 41, Platform: service.PlatformOpenAI, Type: service.AccountTypeSetupToken}
	require.Len(t, h.accountListResponseFromService(account).CodexTurnTickets, 2)
	require.False(t, cfg.Gateway.OpenAICodexTicket.Enabled)
	repo.values[service.SettingKeyOpenAICodexTicketEnabled] = "false"
	settings.InvalidateOpenAICodexTicketEnabledCache()
	require.Empty(t, h.accountResponseFromService(account).CodexTurnTickets)
}
