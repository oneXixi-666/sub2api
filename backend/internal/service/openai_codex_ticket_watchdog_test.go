package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAICodexTicketDegradedLength(t *testing.T) {
	require.Equal(t, 312, openAICodexTicketDegradedLength(292))
	require.Equal(t, 356, openAICodexTicketDegradedLength(332))
	require.Equal(t, 0, openAICodexTicketDegradedLength(318))
}

func TestOpenAICodexTicketWatchdog_InvalidatesPersonal312AndTeam356(t *testing.T) {
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{
		Enabled: true, TargetLength: 292, TTLSeconds: 3600, FailClosed: true,
		Models: []string{openAICodexTicketDefaultModel},
	}, nil)
	now := time.Now()

	personal := ticketTestAccount(41)
	personalTicket := &openAICodexTicket{
		AccountID: personal.ID, Model: openAICodexTicketDefaultModel,
		State: fakeCodexTicketState(292), Length: 292, CapturedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	svc.storeOpenAICodexTicket(context.Background(), personal, personalTicket)
	personalHeaders := http.Header{}
	receipt, err := svc.applyOpenAICodexTicketWithReceipt(context.Background(), personal, openAICodexTicketDefaultModel, personalHeaders)
	require.NoError(t, err)
	require.NotNil(t, receipt)
	req := attachOpenAICodexTicketReceipt((&http.Request{}).WithContext(context.Background()), receipt)
	respHeader := http.Header{}
	respHeader.Set(openAICodexTurnStateHeader, fakeCodexTicketState(312))
	resp := &http.Response{StatusCode: http.StatusOK, Header: respHeader}
	svc.observeOpenAICodexTicketWatchdog(req, personal, resp)
	require.False(t, svc.lookupOpenAICodexTicket(personal, openAICodexTicketDefaultModel).valid(time.Now(), 292))

	team := ticketTestAccount(42)
	team.Credentials["plan_type"] = "team"
	teamTicket := &openAICodexTicket{
		AccountID: team.ID, Model: openAICodexTicketDefaultModel,
		State: fakeCodexTicketState(332), Length: 332, CapturedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	svc.storeOpenAICodexTicket(context.Background(), team, teamTicket)
	teamHeaders := http.Header{}
	teamReceipt, err := svc.applyOpenAICodexTicketWithReceipt(context.Background(), team, openAICodexTicketDefaultModel, teamHeaders)
	require.NoError(t, err)
	teamReq := attachOpenAICodexTicketReceipt((&http.Request{}).WithContext(context.Background()), teamReceipt)
	teamRespHeader := http.Header{}
	teamRespHeader.Set(openAICodexTurnStateHeader, fakeCodexTicketState(356))
	teamResp := &http.Response{StatusCode: http.StatusOK, Header: teamRespHeader}
	svc.observeOpenAICodexTicketWatchdog(teamReq, team, teamResp)
	require.False(t, svc.lookupOpenAICodexTicket(team, openAICodexTicketDefaultModel).valid(time.Now(), 332))
}

func TestOpenAICodexTicketWatchdog_KeepsMatchingLengthAndSkipsNon2xx(t *testing.T) {
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{
		Enabled: true, TargetLength: 292, TTLSeconds: 3600, FailClosed: true,
		Models: []string{openAICodexTicketDefaultModel},
	}, nil)
	account := ticketTestAccount(41)
	now := time.Now()
	ticket := &openAICodexTicket{
		AccountID: account.ID, Model: openAICodexTicketDefaultModel,
		State: fakeCodexTicketState(292), Length: 292, CapturedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	svc.storeOpenAICodexTicket(context.Background(), account, ticket)
	headers := http.Header{}
	receipt, err := svc.applyOpenAICodexTicketWithReceipt(context.Background(), account, openAICodexTicketDefaultModel, headers)
	require.NoError(t, err)
	req := attachOpenAICodexTicketReceipt((&http.Request{}).WithContext(context.Background()), receipt)

	okHeader := http.Header{}
	okHeader.Set(openAICodexTurnStateHeader, ticket.State)
	svc.observeOpenAICodexTicketWatchdog(req, account, &http.Response{StatusCode: http.StatusOK, Header: okHeader})
	require.True(t, svc.lookupOpenAICodexTicket(account, openAICodexTicketDefaultModel).valid(time.Now(), 292))

	limitedHeader := http.Header{}
	limitedHeader.Set(openAICodexTurnStateHeader, fakeCodexTicketState(312))
	svc.observeOpenAICodexTicketWatchdog(req, account, &http.Response{StatusCode: http.StatusTooManyRequests, Header: limitedHeader})
	require.True(t, svc.lookupOpenAICodexTicket(account, openAICodexTicketDefaultModel).valid(time.Now(), 292))
}

func TestOpenAICodexTicketWatchdog_DoesNotKillNewerTicket(t *testing.T) {
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{
		Enabled: true, TargetLength: 292, TTLSeconds: 3600, FailClosed: true,
		Models: []string{openAICodexTicketDefaultModel},
	}, nil)
	account := ticketTestAccount(41)
	now := time.Now()
	old := &openAICodexTicket{
		AccountID: account.ID, Model: openAICodexTicketDefaultModel,
		State: fakeCodexTicketState(292), Length: 292, CapturedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	svc.storeOpenAICodexTicket(context.Background(), account, old)
	headers := http.Header{}
	receipt, err := svc.applyOpenAICodexTicketWithReceipt(context.Background(), account, openAICodexTicketDefaultModel, headers)
	require.NoError(t, err)
	newer := &openAICodexTicket{
		AccountID: account.ID, Model: openAICodexTicketDefaultModel,
		State:  fakeCodexTicketState(292)[:len(openAICodexTicketStatePrefix)] + strings.Repeat("C", 292-len(openAICodexTicketStatePrefix)),
		Length: 292, CapturedAt: now.Add(time.Second), ExpiresAt: now.Add(2 * time.Hour),
	}
	svc.storeOpenAICodexTicket(context.Background(), account, newer)
	req := attachOpenAICodexTicketReceipt((&http.Request{}).WithContext(context.Background()), receipt)
	degradedHeader := http.Header{}
	degradedHeader.Set(openAICodexTurnStateHeader, fakeCodexTicketState(312))
	svc.observeOpenAICodexTicketWatchdog(req, account, &http.Response{StatusCode: http.StatusOK, Header: degradedHeader})
	got := svc.lookupOpenAICodexTicket(account, openAICodexTicketDefaultModel)
	require.True(t, got.valid(time.Now(), 292))
	require.Equal(t, newer.State, got.State)
}

func TestOpenAICodexTicketWatchdog_ModelMismatchInvalidates(t *testing.T) {
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{
		Enabled: true, TargetLength: 292, TTLSeconds: 3600, FailClosed: true,
		Models: []string{openAICodexTicketDefaultModel},
	}, nil)
	account := ticketTestAccount(41)
	now := time.Now()
	ticket := &openAICodexTicket{
		AccountID: account.ID, Model: openAICodexTicketDefaultModel,
		State: fakeCodexTicketState(292), Length: 292, CapturedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	svc.storeOpenAICodexTicket(context.Background(), account, ticket)
	headers := http.Header{}
	receipt, err := svc.applyOpenAICodexTicketWithReceipt(context.Background(), account, openAICodexTicketDefaultModel, headers)
	require.NoError(t, err)
	req := attachOpenAICodexTicketReceipt((&http.Request{}).WithContext(context.Background()), receipt)
	body := io.NopCloser(strings.NewReader("event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"model\":\"gpt-5.4\"}}\n\n"))
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: body}
	svc.observeOpenAICodexTicketWatchdog(req, account, resp)
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.False(t, svc.lookupOpenAICodexTicket(account, openAICodexTicketDefaultModel).valid(time.Now(), 292))
}

func TestOpenAICodexTicketWatchdog_MatchingCompletedModelKeepsTicket(t *testing.T) {
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{
		Enabled: true, TargetLength: 292, TTLSeconds: 3600, FailClosed: true,
		Models: []string{openAICodexTicketDefaultModel},
	}, nil)
	account := ticketTestAccount(41)
	now := time.Now()
	ticket := &openAICodexTicket{
		AccountID: account.ID, Model: openAICodexTicketDefaultModel,
		State: fakeCodexTicketState(292), Length: 292, CapturedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	svc.storeOpenAICodexTicket(context.Background(), account, ticket)
	headers := http.Header{}
	receipt, err := svc.applyOpenAICodexTicketWithReceipt(context.Background(), account, openAICodexTicketDefaultModel, headers)
	require.NoError(t, err)
	req := attachOpenAICodexTicketReceipt((&http.Request{}).WithContext(context.Background()), receipt)
	body := io.NopCloser(strings.NewReader("event: response.completed\ndata: {\"response\":{\"model\":\"gpt-6-astra\"}}\n\n"))
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: body}
	svc.observeOpenAICodexTicketWatchdog(req, account, resp)
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.True(t, svc.lookupOpenAICodexTicket(account, openAICodexTicketDefaultModel).valid(time.Now(), 292))
}
