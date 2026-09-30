//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAICodexTicketModelsStayOnAstraAndSol(t *testing.T) {
	account := ticketTestAccount(7)
	models := openAICodexTicketModels(account, []string{"gpt-5.4", openAICodexTicketDefaultModel})
	require.Equal(t, []string{openAICodexTicketDefaultModel, openAICodexTicketDefaultSolModel}, models)
	require.False(t, openAICodexTicketModelEligible(account, "gpt-5.4"))
	require.Nil(t, openAICodexTicketModels(&Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, nil))
}

func TestCodexTicketHarvestSkipsUnschedulableAccountsAndExtraModels(t *testing.T) {
	var requests atomic.Int64
	upstream := &codexTicketFuncUpstream{do: func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		response := codexTicketResponse()
		response.Body = io.NopCloser(strings.NewReader(""))
		return response, nil
	}}
	account := ticketTestAccount(41)
	account.Schedulable = false
	repo := &codexTicketRefreshRepo{accounts: []Account{*account}}
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{
		Enabled: true, HarvestProxyURL: "http://proxy.example.com:8080",
		Models: []string{"gpt-5.4", openAICodexTicketDefaultModel},
	}, upstream)
	svc.accountRepo = repo

	svc.refreshOpenAICodexTickets(context.Background())
	svc.probeOnceOpenAICodexTicket(context.Background(), account, "gpt-5.4")
	require.Zero(t, requests.Load())

	account.Schedulable = true
	repo.accounts[0] = *account
	svc.refreshOpenAICodexTickets(context.Background())
	require.Equal(t, int64(2), requests.Load())
}
