package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type codexTicketLogsAdminStub struct {
	service.AdminService
	account *service.Account
	err     error
	lookups int
}

func (s *codexTicketLogsAdminStub) GetAccount(context.Context, int64) (*service.Account, error) {
	s.lookups++
	return s.account, s.err
}

type codexTicketLogsStub struct {
	codexTicketStatusStub
	logs      *service.OpenAICodexTicketLogs
	err       error
	calls     int
	accountID int64
	model     string
}

func (s *codexTicketLogsStub) OpenAICodexTicketLogs(_ context.Context, account *service.Account, model string, _ time.Time) (*service.OpenAICodexTicketLogs, error) {
	s.calls++
	s.accountID, s.model = account.ID, model
	return s.logs, s.err
}

type codexTicketLogFeedStub struct {
	codexTicketStatusStub
	feed    *service.OpenAICodexTicketLogFeed
	cleared int
}

func (s *codexTicketLogFeedStub) OpenAICodexTicketLogFeed(context.Context) *service.OpenAICodexTicketLogFeed {
	if s.feed == nil {
		return &service.OpenAICodexTicketLogFeed{Items: []service.OpenAICodexTicketLogFeedItem{}, Limit: 500}
	}
	return s.feed
}

func (s *codexTicketLogFeedStub) ClearOpenAICodexTicketLogs() {
	s.cleared++
	if s.feed != nil {
		s.feed.Items = nil
	}
}

func codexTicketLogsRequest(handler *AccountHandler, path string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/admin/accounts/:id/codex-ticket-logs", handler.GetCodexTicketLogs)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

func TestAccountCodexTicketLogsReturnsScopedUncachedSnapshot(t *testing.T) {
	admin := &codexTicketLogsAdminStub{account: &service.Account{ID: 41, Platform: service.PlatformOpenAI, Type: service.AccountTypeSetupToken}}
	provider := &codexTicketLogsStub{logs: &service.OpenAICodexTicketLogs{
		Model: "gpt-6-astra", Entries: []service.OpenAICodexTicketLogEntry{{ID: 7, Event: "miss", Reason: "missing_state", Attempt: 3, TargetLength: 292}},
		Status: &service.OpenAICodexTicketStatus{Model: "gpt-6-astra", Attempts: 3}, Limit: 200,
	}}
	handler := &AccountHandler{adminService: admin}
	handler.SetCodexTicketStatusProvider(provider)
	response := codexTicketLogsRequest(handler, "/api/v1/admin/accounts/41/codex-ticket-logs?model=%20gpt-6-astra%20")
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Equal(t, int64(41), provider.accountID)
	require.Equal(t, "gpt-6-astra", provider.model)
	require.Equal(t, 1, provider.calls)
	var envelope struct {
		Data service.OpenAICodexTicketLogs `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	require.Equal(t, *provider.logs, envelope.Data)
}

func TestAccountCodexTicketLogsValidatesRequestAndAccount(t *testing.T) {
	parentID := int64(40)
	tests := []struct {
		name        string
		path        string
		account     *service.Account
		accountErr  error
		providerErr error
		noProvider  bool
		status      int
		wantLookups int
		wantCalls   int
	}{
		{name: "invalid ID", path: "oops?model=gpt-6-astra", status: 400},
		{name: "zero ID", path: "0?model=gpt-6-astra", status: 400},
		{name: "negative ID", path: "-1?model=gpt-6-astra", status: 400},
		{name: "missing model", path: "41", status: 400},
		{name: "blank model", path: "41?model=%20", status: 400},
		{name: "missing account", path: "41?model=gpt-6-astra", accountErr: service.ErrAccountNotFound, status: 404, wantLookups: 1},
		{name: "nil account", path: "41?model=gpt-6-astra", status: 404, wantLookups: 1},
		{name: "other platform", path: "41?model=gpt-6-astra", account: &service.Account{ID: 41, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth}, status: 400, wantLookups: 1},
		{name: "API key", path: "41?model=gpt-6-astra", account: &service.Account{ID: 41, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey}, status: 400, wantLookups: 1},
		{name: "shadow", path: "41?model=gpt-6-astra", account: &service.Account{ID: 41, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, ParentAccountID: &parentID}, status: 400, wantLookups: 1},
		{name: "unsupported model", path: "41?model=other", account: &service.Account{ID: 41, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth}, providerErr: service.ErrOpenAICodexTicketLogModel, status: 400, wantLookups: 1, wantCalls: 1},
		{name: "provider unavailable", path: "41?model=gpt-6-astra", account: &service.Account{ID: 41, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth}, noProvider: true, status: 503, wantLookups: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			admin := &codexTicketLogsAdminStub{account: tt.account, err: tt.accountErr}
			handler := &AccountHandler{adminService: admin}
			provider := &codexTicketLogsStub{err: tt.providerErr}
			if tt.noProvider {
				handler.SetCodexTicketStatusProvider(&codexTicketStatusStub{})
			} else {
				handler.SetCodexTicketStatusProvider(provider)
			}
			id, query := tt.path, ""
			for i, char := range tt.path {
				if char == '?' {
					id, query = tt.path[:i], tt.path[i:]
					break
				}
			}
			response := codexTicketLogsRequest(handler, "/api/v1/admin/accounts/"+id+"/codex-ticket-logs"+query)
			require.Equal(t, tt.status, response.Code)
			require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
			require.Equal(t, tt.wantLookups, admin.lookups)
			require.Equal(t, tt.wantCalls, provider.calls)
		})
	}
}

func TestListCodexTicketLogsReturnsProcessLocalFeed(t *testing.T) {
	provider := &codexTicketLogFeedStub{feed: &service.OpenAICodexTicketLogFeed{
		Items: []service.OpenAICodexTicketLogFeedItem{{
			OpenAICodexTicketLogEntry: service.OpenAICodexTicketLogEntry{ID: 9, Event: "success", Reason: "harvested"},
			AccountID:                 41,
			Model:                     "gpt-6-astra",
		}},
		Limit: 500,
	}}
	handler := &AccountHandler{}
	handler.SetCodexTicketStatusProvider(provider)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/admin/codex-ticket-logs", handler.ListCodexTicketLogs)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/codex-ticket-logs", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	var envelope struct {
		Data service.OpenAICodexTicketLogFeed `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, *provider.feed, envelope.Data)
}

func TestClearCodexTicketLogsDropsProcessLocalFeed(t *testing.T) {
	provider := &codexTicketLogFeedStub{feed: &service.OpenAICodexTicketLogFeed{
		Items: []service.OpenAICodexTicketLogFeedItem{{AccountID: 41}},
	}}
	handler := &AccountHandler{}
	handler.SetCodexTicketStatusProvider(provider)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/api/v1/admin/codex-ticket-logs", handler.ClearCodexTicketLogs)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/api/v1/admin/codex-ticket-logs", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, provider.cleared)
	require.Empty(t, provider.feed.Items)
}

func TestListCodexTicketLogsUnavailableWithoutFeedProvider(t *testing.T) {
	handler := &AccountHandler{}
	handler.SetCodexTicketStatusProvider(&codexTicketStatusStub{})
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/admin/codex-ticket-logs", handler.ListCodexTicketLogs)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/codex-ticket-logs", nil))
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}
