package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type upstreamPanelSessionRepoStub struct {
	*upstreamFundsRepositoryStub
	mu                   sync.Mutex
	wallet               UpstreamWallet
	ciphertext           string
	state                UpstreamPanelSessionState
	credentialCiphertext string
}

func newUpstreamPanelSessionRepoStub() *upstreamPanelSessionRepoStub {
	return &upstreamPanelSessionRepoStub{
		upstreamFundsRepositoryStub: &upstreamFundsRepositoryStub{},
		wallet: UpstreamWallet{
			ID: 9, Name: "Panel wallet", Currency: "USD", RechargeMode: "direct",
			AccountIDs: []int64{42}, Accounts: []UpstreamFundsAccount{{ID: 42, Name: "Linked account"}},
		},
	}
}

func (r *upstreamPanelSessionRepoStub) GetWallet(_ context.Context, id int64) (*UpstreamWallet, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id != r.wallet.ID {
		return nil, ErrUpstreamWalletNotFound
	}
	wallet := r.wallet
	wallet.PanelSessionCiphertext = r.ciphertext
	wallet.PanelSession = r.state
	wallet.PanelCredentialAccountID = r.wallet.PanelCredentialAccountID
	wallet.PanelCredentialIdentity = r.wallet.PanelCredentialIdentity
	wallet.PanelCredentialCiphertext = r.credentialCiphertext
	return &wallet, nil
}

func (r *upstreamPanelSessionRepoStub) SaveUpstreamPanelCredentials(_ context.Context, walletID, accountID int64, identity, passwordCiphertext string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if walletID != r.wallet.ID {
		return ErrUpstreamWalletNotFound
	}
	r.wallet.PanelCredentialAccountID = accountID
	r.wallet.PanelCredentialIdentity = identity
	r.credentialCiphertext = passwordCiphertext
	return nil
}

func (r *upstreamPanelSessionRepoStub) ClearUpstreamPanelCredentials(_ context.Context, walletID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if walletID != r.wallet.ID {
		return ErrUpstreamWalletNotFound
	}
	r.wallet.PanelCredentialAccountID = 0
	r.wallet.PanelCredentialIdentity = ""
	r.credentialCiphertext = ""
	return nil
}

func (r *upstreamPanelSessionRepoStub) SaveUpstreamPanelSession(_ context.Context, walletID int64, ciphertext string, state UpstreamPanelSessionState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if walletID != r.wallet.ID {
		return ErrUpstreamWalletNotFound
	}
	r.ciphertext = ciphertext
	r.state = state
	return nil
}

func (r *upstreamPanelSessionRepoStub) CompareAndSwapUpstreamPanelSession(_ context.Context, walletID int64, expected, ciphertext string, state UpstreamPanelSessionState) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if walletID != r.wallet.ID {
		return false, ErrUpstreamWalletNotFound
	}
	if r.ciphertext != expected {
		return false, nil
	}
	r.ciphertext = ciphertext
	r.state = state
	return true, nil
}

func (r *upstreamPanelSessionRepoStub) ClearUpstreamPanelSession(_ context.Context, walletID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if walletID != r.wallet.ID {
		return ErrUpstreamWalletNotFound
	}
	r.ciphertext = ""
	r.state = UpstreamPanelSessionState{}
	return nil
}

func (r *upstreamPanelSessionRepoStub) ListDueUpstreamPanelSessionWalletIDs(context.Context, time.Time, int) ([]int64, error) {
	return nil, nil
}

type upstreamPanelAccountRepoStub struct {
	AccountRepository
	account *Account
}

func (r *upstreamPanelAccountRepoStub) GetByIDs(_ context.Context, ids []int64) ([]*Account, error) {
	for _, id := range ids {
		if r.account != nil && id == r.account.ID {
			return []*Account{r.account}, nil
		}
	}
	return []*Account{}, nil
}

type upstreamPanelTestEncryptor struct{}

func (upstreamPanelTestEncryptor) Encrypt(plaintext string) (string, error) {
	return base64.RawStdEncoding.EncodeToString([]byte("encrypted:" + plaintext)), nil
}

func (upstreamPanelTestEncryptor) Decrypt(ciphertext string) (string, error) {
	decoded, err := base64.RawStdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(string(decoded), "encrypted:"), nil
}

type upstreamPanelHTTPStub struct {
	mu        sync.Mutex
	responses []*http.Response
	requests  []*http.Request
	bodies    []string
}

func (s *upstreamPanelHTTPStub) Do(req *http.Request, proxyURL string, accountID int64, concurrency int) (*http.Response, error) {
	return s.DoWithTLS(req, proxyURL, accountID, concurrency, nil)
}

func (s *upstreamPanelHTTPStub) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	body := ""
	if req.Body != nil {
		payload, _ := io.ReadAll(req.Body)
		body = string(payload)
	}
	s.requests = append(s.requests, req)
	s.bodies = append(s.bodies, body)
	if len(s.responses) == 0 {
		return nil, &upstreamPanelRequestError{code: "missing_test_response"}
	}
	response := s.responses[0]
	s.responses = s.responses[1:]
	return response, nil
}

func upstreamPanelJSONResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func newUpstreamPanelTestService(repo *upstreamPanelSessionRepoStub, upstream *upstreamPanelHTTPStub) (*UpstreamFundsService, upstreamPanelTestEncryptor) {
	account := &Account{
		ID: 42, Name: "Linked account", Type: AccountTypeAPIKey, Platform: PlatformOpenAI, Concurrency: 2,
		Credentials: map[string]any{"base_url": "https://panel.example.com/v1", "api_key": "api-key"},
	}
	accountRepo := &upstreamPanelAccountRepoStub{account: account}
	accountTest := &AccountTestService{
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	svc := NewUpstreamFundsService(repo, accountRepo, accountTest)
	encryptor := upstreamPanelTestEncryptor{}
	svc.panelSessions = &upstreamPanelSessionRuntime{
		service: svc, repo: repo, encryptor: encryptor, keyConfigured: true, stopCh: make(chan struct{}),
	}
	return svc, encryptor
}

func TestUpstreamPanelLoginStoresOnlyEncryptedSessionTokens(t *testing.T) {
	repo := newUpstreamPanelSessionRepoStub()
	upstream := &upstreamPanelHTTPStub{responses: []*http.Response{upstreamPanelJSONResponse(`{
		"code":0,"data":{"access_token":"access-one","refresh_token":"refresh-one","expires_in":3600,"user":{"email":"owner@example.com"}}
	}`)}}
	svc, encryptor := newUpstreamPanelTestService(repo, upstream)

	result, err := svc.LoginPanelSession(context.Background(), repo.wallet.ID, UpstreamPanelLoginInput{
		AccountID: 42, Email: "owner@example.com", Password: "secret-password",
	})
	require.NoError(t, err)
	require.False(t, result.Requires2FA)
	require.True(t, result.Session.Configured)
	require.True(t, result.Session.CredentialsSaved)
	require.Equal(t, "owner@example.com", result.Session.SavedIdentity)
	require.Equal(t, int64(42), result.Session.SavedAccountID)
	require.Equal(t, UpstreamPanelSessionStatusHealthy, result.Session.Status)
	require.Equal(t, "o***r@example.com", result.Session.Identity)
	require.NotEmpty(t, repo.ciphertext)
	require.NotContains(t, repo.ciphertext, "secret-password")

	plaintext, err := encryptor.Decrypt(repo.ciphertext)
	require.NoError(t, err)
	require.Contains(t, plaintext, "access-one")
	require.Contains(t, plaintext, "refresh-one")
	require.NotContains(t, plaintext, "secret-password")
	require.Contains(t, upstream.bodies[0], `"password":"secret-password"`)
	require.NotContains(t, result.Session.Identity, "secret-password")
}

func TestUpstreamPanelLoginCompletesOpaqueTwoFactorChallenge(t *testing.T) {
	repo := newUpstreamPanelSessionRepoStub()
	upstream := &upstreamPanelHTTPStub{responses: []*http.Response{
		upstreamPanelJSONResponse(`{"code":0,"data":{"requires_2fa":true,"temp_token":"upstream-temp-secret"}}`),
		upstreamPanelJSONResponse(`{"code":0,"data":{"access_token":"access-two","refresh_token":"refresh-two","expires_in":3600}}`),
	}}
	svc, _ := newUpstreamPanelTestService(repo, upstream)

	challenge, err := svc.LoginPanelSession(context.Background(), repo.wallet.ID, UpstreamPanelLoginInput{
		AccountID: 42, Email: "owner@example.com", Password: "secret-password",
	})
	require.NoError(t, err)
	require.True(t, challenge.Requires2FA)
	require.NotEmpty(t, challenge.Challenge)
	require.NotContains(t, challenge.Challenge, "upstream-temp-secret")
	require.Empty(t, repo.ciphertext)
	require.NotEmpty(t, repo.credentialCiphertext)

	result, err := svc.CompletePanelSessionTwoFactor(context.Background(), repo.wallet.ID, UpstreamPanelTwoFactorInput{
		Challenge: challenge.Challenge, Code: "123456",
	})
	require.NoError(t, err)
	require.True(t, result.Session.Configured)
	require.Contains(t, upstream.bodies[1], `"temp_token":"upstream-temp-secret"`)
	require.Contains(t, upstream.bodies[1], `"totp_code":"123456"`)
}

func TestUpstreamPanelCheckRefreshesExpiringTokenBeforeProbe(t *testing.T) {
	repo := newUpstreamPanelSessionRepoStub()
	upstream := &upstreamPanelHTTPStub{responses: []*http.Response{
		upstreamPanelJSONResponse(`{"code":0,"data":{"access_token":"access-old","refresh_token":"refresh-old","expires_in":1}}`),
		upstreamPanelJSONResponse(`{"code":0,"data":{"access_token":"access-new","refresh_token":"refresh-new","expires_in":3600}}`),
		upstreamPanelJSONResponse(`{"code":0,"data":{"email":"owner@example.com"}}`),
	}}
	svc, encryptor := newUpstreamPanelTestService(repo, upstream)
	_, err := svc.LoginPanelSession(context.Background(), repo.wallet.ID, UpstreamPanelLoginInput{
		AccountID: 42, Email: "owner@example.com", Password: "secret-password",
	})
	require.NoError(t, err)

	state, err := svc.CheckPanelSession(context.Background(), repo.wallet.ID)
	require.NoError(t, err)
	require.Equal(t, UpstreamPanelSessionStatusHealthy, state.Status)
	require.Len(t, upstream.requests, 3)
	require.Equal(t, "/api/v1/auth/refresh", upstream.requests[1].URL.Path)
	require.Equal(t, "/api/v1/user/profile", upstream.requests[2].URL.Path)
	require.Equal(t, "Bearer access-new", upstream.requests[2].Header.Get("Authorization"))

	plaintext, err := encryptor.Decrypt(repo.ciphertext)
	require.NoError(t, err)
	require.Contains(t, plaintext, "access-new")
	require.Contains(t, plaintext, "refresh-new")
	require.NotContains(t, plaintext, "access-old")
}

func TestUpstreamPanelLoginReusesSavedCredentialsWhenFieldsAreOmitted(t *testing.T) {
	repo := newUpstreamPanelSessionRepoStub()
	upstream := &upstreamPanelHTTPStub{responses: []*http.Response{
		upstreamPanelJSONResponse(`{"code":0,"data":{"access_token":"access-one","expires_in":3600}}`),
		upstreamPanelJSONResponse(`{"code":0,"data":{"access_token":"access-two","expires_in":3600}}`),
	}}
	svc, _ := newUpstreamPanelTestService(repo, upstream)
	_, err := svc.LoginPanelSession(context.Background(), repo.wallet.ID, UpstreamPanelLoginInput{
		AccountID: 42, Email: "owner@example.com", Password: "secret-password",
	})
	require.NoError(t, err)
	_, err = svc.LoginPanelSession(context.Background(), repo.wallet.ID, UpstreamPanelLoginInput{})
	require.NoError(t, err)
	require.Contains(t, upstream.bodies[1], `"email":"owner@example.com"`)
	require.Contains(t, upstream.bodies[1], `"password":"secret-password"`)
}

func TestUpstreamPanelProbeReloginsAfterSessionWasCleared(t *testing.T) {
	repo := newUpstreamPanelSessionRepoStub()
	upstream := &upstreamPanelHTTPStub{responses: []*http.Response{
		upstreamPanelJSONResponse(`{"code":0,"data":{"access_token":"access-one","expires_in":3600}}`),
		upstreamPanelJSONResponse(`{"code":0,"data":{"access_token":"access-two","expires_in":3600}}`),
		upstreamPanelJSONResponse(`{"code":0,"data":{"email":"owner@example.com"}}`),
	}}
	svc, _ := newUpstreamPanelTestService(repo, upstream)
	_, err := svc.LoginPanelSession(context.Background(), repo.wallet.ID, UpstreamPanelLoginInput{
		AccountID: 42, Email: "owner@example.com", Password: "secret-password",
	})
	require.NoError(t, err)
	state, err := svc.DeletePanelSession(context.Background(), repo.wallet.ID)
	require.NoError(t, err)
	require.False(t, state.Configured)
	require.True(t, state.CredentialsSaved)
	state, err = svc.CheckPanelSession(context.Background(), repo.wallet.ID)
	require.NoError(t, err)
	require.True(t, state.Configured)
	require.Equal(t, UpstreamPanelSessionStatusHealthy, state.Status)
	require.Equal(t, "/api/v1/auth/login", upstream.requests[1].URL.Path)
	require.Equal(t, "/api/v1/user/profile", upstream.requests[2].URL.Path)
}

func TestUpstreamPanelSessionImportsEncryptedAccessToken(t *testing.T) {
	repo := newUpstreamPanelSessionRepoStub()
	upstream := &upstreamPanelHTTPStub{}
	svc, encryptor := newUpstreamPanelTestService(repo, upstream)
	state, err := svc.ImportPanelSession(context.Background(), repo.wallet.ID, UpstreamPanelImportInput{
		AccountID: 42, AccessToken: "browser-access", RefreshToken: "browser-refresh", Identity: "owner@example.com", ExpiresAt: timePointer(time.Now().Add(2 * time.Hour)),
	})
	require.NoError(t, err)
	require.True(t, state.Configured)
	require.Equal(t, "o***r@example.com", state.Identity)
	require.NotEmpty(t, repo.ciphertext)
	plaintext, err := encryptor.Decrypt(repo.ciphertext)
	require.NoError(t, err)
	require.Contains(t, plaintext, "browser-access")
	require.Contains(t, plaintext, "browser-refresh")
	require.NotContains(t, string(mustMarshalPanelState(*state)), "browser-access")
}

func TestUpstreamPanelLoginReturnsManualImportForForbiddenChallenge(t *testing.T) {
	repo := newUpstreamPanelSessionRepoStub()
	upstream := &upstreamPanelHTTPStub{responses: []*http.Response{{
		StatusCode: http.StatusForbidden, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("<html>cloudflare challenge</html>")),
	}}}
	svc, _ := newUpstreamPanelTestService(repo, upstream)
	_, err := svc.LoginPanelSession(context.Background(), repo.wallet.ID, UpstreamPanelLoginInput{
		AccountID: 42, Email: "owner@example.com", Password: "secret-password",
	})
	require.ErrorIs(t, err, ErrUpstreamPanelManualImportRequired)
}

func mustMarshalPanelState(state UpstreamPanelSessionState) []byte {
	encoded, _ := json.Marshal(state)
	return encoded
}

func TestUpstreamPanelSessionEnablesRechargeWithoutLegacyCredential(t *testing.T) {
	repo := newUpstreamPanelSessionRepoStub()
	upstream := &upstreamPanelHTTPStub{responses: []*http.Response{
		upstreamPanelJSONResponse(`{"code":0,"data":{"access_token":"access-one","refresh_token":"refresh-one","expires_in":3600}}`),
		upstreamPanelJSONResponse(`{"code":0,"data":{"methods":{"alipay":{"currency":"CNY","display_name":"Alipay","daily_remaining":1000,"single_min":1,"single_max":500,"fee_rate":0,"available":true}},"balance_disabled":false}}`),
	}}
	svc, _ := newUpstreamPanelTestService(repo, upstream)
	_, err := svc.LoginPanelSession(context.Background(), repo.wallet.ID, UpstreamPanelLoginInput{
		AccountID: 42, Email: "owner@example.com", Password: "secret-password",
	})
	require.NoError(t, err)

	channels, err := svc.ListPaymentChannels(context.Background(), repo.wallet.ID)
	require.NoError(t, err)
	require.Len(t, channels, 1)
	require.Equal(t, "alipay", channels[0].ID)
	require.Len(t, upstream.requests, 1, "fixed Alipay discovery must not request checkout-info")
}

func TestUpstreamPanelSessionDeleteImmediatelyDisablesAuthorization(t *testing.T) {
	repo := newUpstreamPanelSessionRepoStub()
	upstream := &upstreamPanelHTTPStub{responses: []*http.Response{upstreamPanelJSONResponse(`{
		"code":0,"data":{"access_token":"access-one","refresh_token":"refresh-one","expires_in":3600}
	}`)}}
	svc, _ := newUpstreamPanelTestService(repo, upstream)
	_, err := svc.LoginPanelSession(context.Background(), repo.wallet.ID, UpstreamPanelLoginInput{
		AccountID: 42, Email: "owner@example.com", Password: "secret-password",
	})
	require.NoError(t, err)

	state, err := svc.DeletePanelSession(context.Background(), repo.wallet.ID)
	require.NoError(t, err)
	require.False(t, state.Configured)
	require.Equal(t, UpstreamPanelSessionStatusNotConfigured, state.Status)
	require.Empty(t, repo.ciphertext)
}
