package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/util/httputil"
	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
)

const (
	UpstreamPanelSessionStatusNotConfigured = "not_configured"
	UpstreamPanelSessionStatusUnchecked     = "unchecked"
	UpstreamPanelSessionStatusHealthy       = "healthy"
	UpstreamPanelSessionStatusDegraded      = "degraded"
	UpstreamPanelSessionStatusExpired       = "expired"

	upstreamPanelSessionSecretVersion       = 1
	upstreamPanelRequestTimeout             = 10 * time.Second
	upstreamPanelAccessFallbackTTL          = 15 * time.Minute
	upstreamPanelImportedFallbackTTL        = 24 * time.Hour
	upstreamPanelRefreshWindow              = 2 * time.Minute
	upstreamPanelHealthyCheckInterval       = 2 * time.Minute
	upstreamPanelRetryCheckInterval         = 30 * time.Second
	upstreamPanelRunnerInterval             = 30 * time.Second
	upstreamPanelRunnerTimeout              = 45 * time.Second
	upstreamPanelRunnerConcurrency          = 4
	upstreamPanelRunnerBatchSize            = 40
	upstreamPanelLeaderLockKey              = "upstream-funds:panel-session:probe"
	upstreamPanelLeaderLockTTL              = time.Minute
	upstreamPanelChallengeTTL               = 10 * time.Minute
	upstreamPanelMaxResponseBody      int64 = 128 * 1024
)

var sixDigitPanelCode = regexp.MustCompile(`^[0-9]{6}$`)

var (
	ErrUpstreamPanelSessionUnavailable = infraerrors.ServiceUnavailable(
		"UPSTREAM_PANEL_SESSION_UNAVAILABLE",
		"upstream panel authorization is unavailable",
	)
	ErrUpstreamPanelLoginRejected = infraerrors.Unauthorized(
		"UPSTREAM_PANEL_LOGIN_REJECTED",
		"upstream panel login was rejected",
	)
	ErrUpstreamPanelChallengeInvalid = infraerrors.BadRequest(
		"UPSTREAM_PANEL_CHALLENGE_INVALID",
		"upstream panel verification challenge is invalid or expired",
	)
	ErrUpstreamPanelAccountInvalid = infraerrors.BadRequest(
		"UPSTREAM_PANEL_ACCOUNT_INVALID",
		"select a linked account with a valid upstream base URL",
	)
	ErrUpstreamPanelManualImportRequired = infraerrors.ServiceUnavailable(
		"UPSTREAM_PANEL_MANUAL_IMPORT_REQUIRED",
		"upstream login requires browser verification; import the authenticated panel session instead",
	)
)

type UpstreamPanelSessionState struct {
	Configured              bool       `json:"configured"`
	EncryptionKeyConfigured bool       `json:"encryption_key_configured"`
	CredentialsSaved        bool       `json:"credentials_saved"`
	SavedIdentity           string     `json:"saved_identity,omitempty"`
	SavedAccountID          int64      `json:"saved_account_id,omitempty"`
	SavedAccountName        string     `json:"saved_account_name,omitempty"`
	Status                  string     `json:"status"`
	Identity                string     `json:"identity,omitempty"`
	AccountID               int64      `json:"account_id,omitempty"`
	AccountName             string     `json:"account_name,omitempty"`
	ExpiresAt               *time.Time `json:"expires_at,omitempty"`
	LastCheckedAt           *time.Time `json:"last_checked_at,omitempty"`
	NextCheckAt             *time.Time `json:"next_check_at,omitempty"`
	LastError               string     `json:"last_error,omitempty"`
}

type UpstreamPanelLoginInput struct {
	AccountID int64
	Email     string
	Password  string
}

type UpstreamPanelTwoFactorInput struct {
	Challenge string
	Code      string
}

type UpstreamPanelImportInput struct {
	AccountID    int64
	AccessToken  string
	RefreshToken string
	Identity     string
	ExpiresIn    int
	ExpiresAt    *time.Time
}

type UpstreamPanelLoginResult struct {
	Requires2FA bool                      `json:"requires_2fa"`
	Challenge   string                    `json:"challenge,omitempty"`
	Session     UpstreamPanelSessionState `json:"session"`
}

type UpstreamPanelSessionRepository interface {
	SaveUpstreamPanelSession(ctx context.Context, walletID int64, ciphertext string, state UpstreamPanelSessionState) error
	SaveUpstreamPanelCredentials(ctx context.Context, walletID, accountID int64, identity, passwordCiphertext string) error
	ClearUpstreamPanelCredentials(ctx context.Context, walletID int64) error
	CompareAndSwapUpstreamPanelSession(ctx context.Context, walletID int64, expectedCiphertext, ciphertext string, state UpstreamPanelSessionState) (bool, error)
	ClearUpstreamPanelSession(ctx context.Context, walletID int64) error
	ListDueUpstreamPanelSessionWalletIDs(ctx context.Context, now time.Time, limit int) ([]int64, error)
}

type upstreamPanelSessionSecret struct {
	Version      int       `json:"version"`
	Origin       string    `json:"origin"`
	Identity     string    `json:"identity"`
	AccountID    int64     `json:"account_id"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	AccessExpiry time.Time `json:"access_expiry"`
}

const upstreamPanelCredentialsVersion = 1

type upstreamPanelLoginCredentials struct {
	Version   int    `json:"version"`
	AccountID int64  `json:"account_id"`
	Identity  string `json:"identity"`
	Password  string `json:"password"`
}

type upstreamPanelChallenge struct {
	WalletID  int64     `json:"wallet_id"`
	AccountID int64     `json:"account_id"`
	Origin    string    `json:"origin"`
	Identity  string    `json:"identity"`
	TempToken string    `json:"temp_token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type upstreamPanelAuthResponse struct {
	AccessToken     string `json:"access_token"`
	RefreshToken    string `json:"refresh_token"`
	ExpiresIn       int    `json:"expires_in"`
	Requires2FA     bool   `json:"requires_2fa"`
	TempToken       string `json:"temp_token"`
	UserEmailMasked string `json:"user_email_masked"`
	User            *struct {
		Email string `json:"email"`
	} `json:"user"`
}

type upstreamPanelSessionRuntime struct {
	service       *UpstreamFundsService
	repo          UpstreamPanelSessionRepository
	encryptor     SecretEncryptor
	keyConfigured bool
	lockCache     LeaderLockCache
	db            *sql.DB
	instanceID    string
	refreshGroup  singleflight.Group
	stopCh        chan struct{}
	stopOnce      sync.Once
	wg            sync.WaitGroup
}

type upstreamPanelRequestError struct {
	code       string
	statusCode int
}

func (e *upstreamPanelRequestError) Error() string { return e.code }

func ProvideUpstreamFundsService(
	repo UpstreamFundsRepository,
	accountRepo AccountRepository,
	accountTestService *AccountTestService,
	encryptor SecretEncryptor,
	cfg *config.Config,
	lockCache LeaderLockCache,
	db *sql.DB,
) *UpstreamFundsService {
	svc := NewUpstreamFundsService(repo, accountRepo, accountTestService)
	panelRepo, ok := repo.(UpstreamPanelSessionRepository)
	if !ok || encryptor == nil || accountTestService == nil || accountTestService.httpUpstream == nil {
		return svc
	}
	runtime := &upstreamPanelSessionRuntime{
		service: svc, repo: panelRepo, encryptor: encryptor, lockCache: lockCache, db: db,
		instanceID: uuid.NewString(), stopCh: make(chan struct{}),
	}
	if cfg != nil {
		runtime.keyConfigured = cfg.Totp.EncryptionKeyConfigured
	}
	svc.panelSessions = runtime
	runtime.Start()
	return svc
}

func (s *UpstreamFundsService) Stop() {
	if s != nil && s.panelSessions != nil {
		s.panelSessions.Stop()
	}
}

func (s *UpstreamFundsService) normalizePanelSessionState(wallet *UpstreamWallet) {
	if wallet == nil {
		return
	}
	state := wallet.PanelSession
	state.Configured = strings.TrimSpace(wallet.PanelSessionCiphertext) != ""
	state.EncryptionKeyConfigured = s != nil && s.panelSessions != nil && s.panelSessions.keyConfigured
	state.CredentialsSaved = strings.TrimSpace(wallet.PanelCredentialCiphertext) != ""
	state.SavedIdentity = strings.TrimSpace(wallet.PanelCredentialIdentity)
	state.SavedAccountID = wallet.PanelCredentialAccountID
	state.SavedAccountName = ""
	for _, account := range wallet.Accounts {
		if account.ID == state.SavedAccountID {
			state.SavedAccountName = account.Name
			break
		}
	}
	if !state.Configured {
		state.Status = UpstreamPanelSessionStatusNotConfigured
		state.Identity = ""
		state.AccountID = 0
		state.AccountName = ""
		state.ExpiresAt = nil
		state.LastCheckedAt = nil
		state.NextCheckAt = nil
		state.LastError = ""
	} else if state.Status == "" {
		state.Status = UpstreamPanelSessionStatusUnchecked
	}
	wallet.PanelSession = state
}

func (s *UpstreamFundsService) PanelSession(ctx context.Context, walletID int64) (*UpstreamPanelSessionState, error) {
	wallet, err := s.repo.GetWallet(ctx, walletID)
	if err != nil {
		return nil, err
	}
	s.normalizePanelSessionState(wallet)
	state := wallet.PanelSession
	return &state, nil
}

func (s *UpstreamFundsService) LoginPanelSession(ctx context.Context, walletID int64, input UpstreamPanelLoginInput) (*UpstreamPanelLoginResult, error) {
	runtime, err := s.requirePanelSessionRuntime()
	if err != nil {
		return nil, err
	}
	wallet, err := s.repo.GetWallet(ctx, walletID)
	if err != nil {
		return nil, err
	}
	email := strings.TrimSpace(input.Email)
	password := input.Password
	accountID := input.AccountID
	if email == "" {
		email = strings.TrimSpace(wallet.PanelCredentialIdentity)
	}
	if accountID <= 0 {
		accountID = wallet.PanelCredentialAccountID
	}
	if saved, savedErr := runtime.decryptCredentials(wallet.PanelCredentialCiphertext); savedErr == nil {
		if password == "" && ((accountID > 0 && accountID != saved.AccountID) || (email != "" && !strings.EqualFold(email, saved.Identity))) {
			return nil, ErrUpstreamPanelLoginRejected
		}
		if email == "" {
			email = saved.Identity
		}
		if password == "" {
			password = saved.Password
		}
		if accountID <= 0 {
			accountID = saved.AccountID
		}
	}
	if email == "" || len(email) > 320 || password == "" || len(password) > 4096 {
		return nil, ErrUpstreamPanelLoginRejected
	}
	wallet, account, origin, err := s.panelLoginTarget(ctx, walletID, accountID)
	if err != nil {
		return nil, err
	}
	var auth upstreamPanelAuthResponse
	err = runtime.doJSON(ctx, account, http.MethodPost, origin, "/api/v1/auth/login", map[string]string{
		"email": email, "password": password,
	}, "", &auth)
	if err != nil {
		if panelLoginRequiresManualImport(err) {
			return nil, ErrUpstreamPanelManualImportRequired
		}
		return nil, ErrUpstreamPanelLoginRejected
	}
	encrypted, encryptErr := runtime.encryptCredentials(upstreamPanelLoginCredentials{
		Version: upstreamPanelCredentialsVersion, AccountID: account.ID, Identity: email, Password: password,
	})
	if encryptErr != nil {
		return nil, ErrUpstreamPanelSessionUnavailable
	}
	if saveErr := runtime.repo.SaveUpstreamPanelCredentials(ctx, wallet.ID, account.ID, email, encrypted); saveErr != nil {
		return nil, ErrUpstreamPanelSessionUnavailable
	}
	wallet.PanelCredentialAccountID = account.ID
	wallet.PanelCredentialIdentity = email
	wallet.PanelCredentialCiphertext = encrypted
	if auth.Requires2FA {
		if strings.TrimSpace(auth.TempToken) == "" {
			return nil, ErrUpstreamPanelLoginRejected
		}
		challenge, err := runtime.encryptChallenge(upstreamPanelChallenge{
			WalletID: wallet.ID, AccountID: account.ID, Origin: origin, Identity: email,
			TempToken: auth.TempToken, ExpiresAt: time.Now().UTC().Add(upstreamPanelChallengeTTL),
		})
		if err != nil {
			return nil, ErrUpstreamPanelSessionUnavailable
		}
		s.normalizePanelSessionState(wallet)
		state := wallet.PanelSession
		return &UpstreamPanelLoginResult{Requires2FA: true, Challenge: challenge, Session: state}, nil
	}
	state, err := runtime.saveAuthResponse(ctx, wallet, account, origin, email, auth)
	if err != nil {
		return nil, err
	}
	return &UpstreamPanelLoginResult{Session: *state}, nil
}

func (s *UpstreamFundsService) CompletePanelSessionTwoFactor(ctx context.Context, walletID int64, input UpstreamPanelTwoFactorInput) (*UpstreamPanelLoginResult, error) {
	runtime, err := s.requirePanelSessionRuntime()
	if err != nil {
		return nil, err
	}
	code := strings.TrimSpace(input.Code)
	if !sixDigitPanelCode.MatchString(code) {
		return nil, ErrUpstreamPanelChallengeInvalid
	}
	challenge, err := runtime.decryptChallenge(strings.TrimSpace(input.Challenge))
	if err != nil || challenge.WalletID != walletID || challenge.ExpiresAt.Before(time.Now().UTC()) {
		return nil, ErrUpstreamPanelChallengeInvalid
	}
	wallet, account, origin, err := s.panelLoginTarget(ctx, walletID, challenge.AccountID)
	if err != nil || origin != challenge.Origin {
		return nil, ErrUpstreamPanelChallengeInvalid
	}
	var auth upstreamPanelAuthResponse
	err = runtime.doJSON(ctx, account, http.MethodPost, origin, "/api/v1/auth/login/2fa", map[string]string{
		"temp_token": challenge.TempToken, "totp_code": code,
	}, "", &auth)
	if err != nil {
		return nil, ErrUpstreamPanelLoginRejected
	}
	state, err := runtime.saveAuthResponse(ctx, wallet, account, origin, challenge.Identity, auth)
	if err != nil {
		return nil, err
	}
	return &UpstreamPanelLoginResult{Session: *state}, nil
}

func (s *UpstreamFundsService) ImportPanelSession(ctx context.Context, walletID int64, input UpstreamPanelImportInput) (*UpstreamPanelSessionState, error) {
	runtime, err := s.requirePanelSessionRuntime()
	if err != nil {
		return nil, err
	}
	accessToken := strings.TrimSpace(input.AccessToken)
	refreshToken := strings.TrimSpace(input.RefreshToken)
	identity := strings.TrimSpace(input.Identity)
	if input.AccountID <= 0 || accessToken == "" || len(accessToken) > 65536 || len(refreshToken) > 65536 || len(identity) > 320 || input.ExpiresIn < 0 || input.ExpiresIn > 31536000 {
		return nil, ErrUpstreamPanelLoginRejected
	}
	wallet, account, origin, err := s.panelLoginTarget(ctx, walletID, input.AccountID)
	if err != nil {
		return nil, err
	}
	if wallet.PanelCredentialAccountID != 0 && wallet.PanelCredentialAccountID != account.ID {
		if err := runtime.repo.ClearUpstreamPanelCredentials(ctx, wallet.ID); err != nil {
			return nil, ErrUpstreamPanelSessionUnavailable
		}
		wallet.PanelCredentialAccountID = 0
		wallet.PanelCredentialIdentity = ""
		wallet.PanelCredentialCiphertext = ""
	}
	expiresIn := input.ExpiresIn
	if input.ExpiresAt != nil {
		remaining := time.Until(input.ExpiresAt.UTC())
		if remaining <= 0 || remaining > 365*24*time.Hour {
			return nil, ErrUpstreamPanelLoginRejected
		}
		expiresIn = int(remaining.Round(time.Second) / time.Second)
	}
	if expiresIn == 0 {
		expiresIn = int(upstreamPanelImportedFallbackTTL / time.Second)
	}
	return runtime.saveAuthResponse(ctx, wallet, account, origin, identity, upstreamPanelAuthResponse{
		AccessToken: accessToken, RefreshToken: refreshToken, ExpiresIn: expiresIn,
	})
}

func (s *UpstreamFundsService) CheckPanelSession(ctx context.Context, walletID int64) (*UpstreamPanelSessionState, error) {
	runtime, err := s.requirePanelSessionRuntime()
	if err != nil {
		return nil, err
	}
	return runtime.check(ctx, walletID)
}

func (s *UpstreamFundsService) DeletePanelSession(ctx context.Context, walletID int64) (*UpstreamPanelSessionState, error) {
	runtime, err := s.requirePanelSessionRuntime()
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.GetWallet(ctx, walletID); err != nil {
		return nil, err
	}
	if err := runtime.repo.ClearUpstreamPanelSession(ctx, walletID); err != nil {
		return nil, err
	}
	return s.PanelSession(ctx, walletID)
}

func (s *UpstreamFundsService) requirePanelSessionRuntime() (*upstreamPanelSessionRuntime, error) {
	if s == nil || s.panelSessions == nil || s.panelSessions.repo == nil || s.panelSessions.encryptor == nil {
		return nil, ErrUpstreamPanelSessionUnavailable
	}
	return s.panelSessions, nil
}

func (s *UpstreamFundsService) panelLoginTarget(ctx context.Context, walletID, accountID int64) (*UpstreamWallet, *Account, string, error) {
	wallet, err := s.repo.GetWallet(ctx, walletID)
	if err != nil {
		return nil, nil, "", err
	}
	accounts, err := s.loadWalletAccounts(ctx, wallet)
	if err != nil {
		return nil, nil, "", err
	}
	provider := s.panelSessions.serviceProvider()
	if provider == nil || provider.accountTestService == nil {
		return nil, nil, "", ErrUpstreamPanelSessionUnavailable
	}
	for _, account := range accounts {
		if account == nil || (accountID > 0 && account.ID != accountID) {
			continue
		}
		baseURL, err := provider.accountTestService.validateUpstreamBaseURL(strings.TrimSpace(account.GetCredential("base_url")))
		if err != nil {
			continue
		}
		return wallet, account, normalizeUpstreamPanelOrigin(baseURL), nil
	}
	return nil, nil, "", ErrUpstreamPanelAccountInvalid
}

func (r *upstreamPanelSessionRuntime) serviceProvider() *sub2APIUsageBalanceProvider {
	provider, _ := r.service.balanceProvider.(*sub2APIUsageBalanceProvider)
	return provider
}

func (s *UpstreamFundsService) panelSessionConfigured(wallet *UpstreamWallet, accounts []*Account) bool {
	if s == nil || s.panelSessions == nil || wallet == nil || strings.TrimSpace(wallet.PanelSessionCiphertext) == "" {
		return false
	}
	for _, account := range accounts {
		if account == nil || (wallet.PanelSession.AccountID > 0 && account.ID != wallet.PanelSession.AccountID) {
			continue
		}
		if strings.TrimSpace(account.GetCredential("base_url")) != "" {
			return true
		}
	}
	return false
}

func (s *UpstreamFundsService) resolvePanelCredential(ctx context.Context, wallet *UpstreamWallet, accounts []*Account) (*Account, string, error) {
	runtime, err := s.requirePanelSessionRuntime()
	if err != nil {
		return nil, "", err
	}
	if wallet == nil || strings.TrimSpace(wallet.PanelSessionCiphertext) == "" {
		return nil, "", ErrUpstreamPanelSessionUnavailable
	}
	secret, err := runtime.decryptSecret(wallet.PanelSessionCiphertext)
	if err != nil {
		return nil, "", ErrUpstreamPanelSessionUnavailable
	}
	account := runtime.matchAccount(accounts, secret)
	if account == nil {
		return nil, "", ErrUpstreamPanelAccountInvalid
	}
	if time.Until(secret.AccessExpiry) <= upstreamPanelRefreshWindow {
		value, refreshErr, _ := runtime.refreshGroup.Do(strconv.FormatInt(wallet.ID, 10), func() (any, error) {
			return runtime.refresh(ctx, wallet.ID)
		})
		if refreshErr != nil {
			return nil, "", refreshErr
		}
		refreshed, ok := value.(*upstreamPanelSessionSecret)
		if !ok || refreshed == nil {
			return nil, "", ErrUpstreamPanelSessionUnavailable
		}
		secret = refreshed
		account = runtime.matchAccount(accounts, secret)
		if account == nil {
			return nil, "", ErrUpstreamPanelAccountInvalid
		}
	}
	if strings.TrimSpace(secret.AccessToken) == "" || secret.AccessExpiry.Before(time.Now().UTC()) {
		return nil, "", ErrUpstreamPanelSessionUnavailable
	}
	return account, secret.AccessToken, nil
}

func (r *upstreamPanelSessionRuntime) saveAuthResponse(ctx context.Context, wallet *UpstreamWallet, account *Account, origin, identity string, auth upstreamPanelAuthResponse) (*UpstreamPanelSessionState, error) {
	if strings.TrimSpace(auth.AccessToken) == "" {
		return nil, ErrUpstreamPanelLoginRejected
	}
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(auth.ExpiresIn) * time.Second)
	if auth.ExpiresIn <= 0 {
		expiresAt = now.Add(upstreamPanelAccessFallbackTTL)
	}
	if auth.User != nil && strings.TrimSpace(auth.User.Email) != "" {
		identity = strings.TrimSpace(auth.User.Email)
	}
	secret := upstreamPanelSessionSecret{
		Version: upstreamPanelSessionSecretVersion, Origin: origin, Identity: identity, AccountID: account.ID,
		AccessToken: strings.TrimSpace(auth.AccessToken), RefreshToken: strings.TrimSpace(auth.RefreshToken), AccessExpiry: expiresAt,
	}
	ciphertext, err := r.encryptSecret(secret)
	if err != nil {
		return nil, ErrUpstreamPanelSessionUnavailable
	}
	next := nextUpstreamPanelCheck(now, expiresAt)
	state := UpstreamPanelSessionState{
		Configured: true, EncryptionKeyConfigured: r.keyConfigured, Status: UpstreamPanelSessionStatusHealthy,
		CredentialsSaved: strings.TrimSpace(wallet.PanelCredentialCiphertext) != "",
		SavedIdentity:    strings.TrimSpace(wallet.PanelCredentialIdentity),
		SavedAccountID:   wallet.PanelCredentialAccountID, SavedAccountName: account.Name,
		Identity: MaskEmail(identity), AccountID: account.ID, AccountName: account.Name,
		ExpiresAt: &expiresAt, LastCheckedAt: &now, NextCheckAt: &next,
	}
	if err := r.repo.SaveUpstreamPanelSession(ctx, wallet.ID, ciphertext, state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (r *upstreamPanelSessionRuntime) encryptSecret(secret upstreamPanelSessionSecret) (string, error) {
	payload, err := json.Marshal(secret)
	if err != nil {
		return "", err
	}
	return r.encryptor.Encrypt(string(payload))
}

func (r *upstreamPanelSessionRuntime) encryptCredentials(credentials upstreamPanelLoginCredentials) (string, error) {
	payload, err := json.Marshal(credentials)
	if err != nil {
		return "", err
	}
	return r.encryptor.Encrypt(string(payload))
}

func (r *upstreamPanelSessionRuntime) decryptCredentials(ciphertext string) (*upstreamPanelLoginCredentials, error) {
	plaintext, err := r.encryptor.Decrypt(strings.TrimSpace(ciphertext))
	if err != nil {
		return nil, err
	}
	var credentials upstreamPanelLoginCredentials
	if err := json.Unmarshal([]byte(plaintext), &credentials); err != nil {
		return nil, err
	}
	if credentials.Version != upstreamPanelCredentialsVersion || credentials.AccountID <= 0 || strings.TrimSpace(credentials.Identity) == "" || credentials.Password == "" {
		return nil, errors.New("invalid upstream panel login credentials")
	}
	return &credentials, nil
}

func (r *upstreamPanelSessionRuntime) decryptSecret(ciphertext string) (*upstreamPanelSessionSecret, error) {
	plaintext, err := r.encryptor.Decrypt(strings.TrimSpace(ciphertext))
	if err != nil {
		return nil, err
	}
	var secret upstreamPanelSessionSecret
	if err := json.Unmarshal([]byte(plaintext), &secret); err != nil {
		return nil, err
	}
	if secret.Version != upstreamPanelSessionSecretVersion || secret.AccountID <= 0 || strings.TrimSpace(secret.Origin) == "" || strings.TrimSpace(secret.AccessToken) == "" {
		return nil, errors.New("invalid upstream panel session secret")
	}
	return &secret, nil
}

func (r *upstreamPanelSessionRuntime) encryptChallenge(challenge upstreamPanelChallenge) (string, error) {
	payload, err := json.Marshal(challenge)
	if err != nil {
		return "", err
	}
	return r.encryptor.Encrypt(string(payload))
}

func (r *upstreamPanelSessionRuntime) decryptChallenge(ciphertext string) (*upstreamPanelChallenge, error) {
	plaintext, err := r.encryptor.Decrypt(ciphertext)
	if err != nil {
		return nil, err
	}
	var challenge upstreamPanelChallenge
	if err := json.Unmarshal([]byte(plaintext), &challenge); err != nil {
		return nil, err
	}
	if challenge.WalletID <= 0 || challenge.AccountID <= 0 || challenge.TempToken == "" || challenge.Origin == "" {
		return nil, errors.New("invalid upstream panel challenge")
	}
	return &challenge, nil
}

func (r *upstreamPanelSessionRuntime) matchAccount(accounts []*Account, secret *upstreamPanelSessionSecret) *Account {
	provider := r.serviceProvider()
	if provider == nil || provider.accountTestService == nil || secret == nil {
		return nil
	}
	for _, account := range accounts {
		if account == nil || account.ID != secret.AccountID {
			continue
		}
		baseURL, err := provider.accountTestService.validateUpstreamBaseURL(strings.TrimSpace(account.GetCredential("base_url")))
		if err == nil && normalizeUpstreamPanelOrigin(baseURL) == secret.Origin {
			return account
		}
	}
	return nil
}

func (r *upstreamPanelSessionRuntime) refresh(ctx context.Context, walletID int64) (*upstreamPanelSessionSecret, error) {
	wallet, err := r.service.repo.GetWallet(ctx, walletID)
	if err != nil {
		return nil, err
	}
	secret, err := r.decryptSecret(wallet.PanelSessionCiphertext)
	if err != nil {
		r.markState(ctx, wallet, UpstreamPanelSessionStatusExpired, "decrypt_failed", nil)
		return nil, ErrUpstreamPanelSessionUnavailable
	}
	if time.Until(secret.AccessExpiry) > upstreamPanelRefreshWindow {
		return secret, nil
	}
	if strings.TrimSpace(secret.RefreshToken) == "" {
		r.markState(ctx, wallet, UpstreamPanelSessionStatusExpired, "refresh_token_missing", secret)
		return nil, ErrUpstreamPanelSessionUnavailable
	}
	accounts, err := r.service.loadWalletAccounts(ctx, wallet)
	if err != nil {
		return nil, err
	}
	account := r.matchAccount(accounts, secret)
	if account == nil {
		r.markState(ctx, wallet, UpstreamPanelSessionStatusDegraded, "account_mismatch", secret)
		return nil, ErrUpstreamPanelAccountInvalid
	}
	var auth upstreamPanelAuthResponse
	err = r.doJSON(ctx, account, http.MethodPost, secret.Origin, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": secret.RefreshToken,
	}, "", &auth)
	if err != nil || strings.TrimSpace(auth.AccessToken) == "" {
		r.markState(ctx, wallet, UpstreamPanelSessionStatusExpired, "refresh_rejected", secret)
		return nil, ErrUpstreamPanelSessionUnavailable
	}
	now := time.Now().UTC()
	secret.AccessToken = strings.TrimSpace(auth.AccessToken)
	if strings.TrimSpace(auth.RefreshToken) != "" {
		secret.RefreshToken = strings.TrimSpace(auth.RefreshToken)
	}
	secret.AccessExpiry = now.Add(time.Duration(auth.ExpiresIn) * time.Second)
	if auth.ExpiresIn <= 0 {
		secret.AccessExpiry = now.Add(upstreamPanelAccessFallbackTTL)
	}
	ciphertext, err := r.encryptSecret(*secret)
	if err != nil {
		return nil, ErrUpstreamPanelSessionUnavailable
	}
	next := nextUpstreamPanelCheck(now, secret.AccessExpiry)
	state := wallet.PanelSession
	state.Configured = true
	state.EncryptionKeyConfigured = r.keyConfigured
	state.Status = UpstreamPanelSessionStatusHealthy
	state.ExpiresAt = timePointer(secret.AccessExpiry)
	state.LastCheckedAt = &now
	state.NextCheckAt = &next
	state.LastError = ""
	swapped, err := r.repo.CompareAndSwapUpstreamPanelSession(ctx, wallet.ID, wallet.PanelSessionCiphertext, ciphertext, state)
	if err != nil {
		return nil, err
	}
	if swapped {
		return secret, nil
	}
	latest, err := r.service.repo.GetWallet(ctx, wallet.ID)
	if err != nil {
		return nil, err
	}
	return r.decryptSecret(latest.PanelSessionCiphertext)
}

// relogin uses the encrypted credentials saved by the last successful login.
// It is intentionally server-owned so probing continues after the admin page closes.
func (r *upstreamPanelSessionRuntime) relogin(ctx context.Context, wallet *UpstreamWallet) error {
	if wallet == nil || strings.TrimSpace(wallet.PanelCredentialCiphertext) == "" {
		return ErrUpstreamPanelLoginRejected
	}
	credentials, err := r.decryptCredentials(wallet.PanelCredentialCiphertext)
	if err != nil {
		return ErrUpstreamPanelSessionUnavailable
	}
	wallet, account, origin, err := r.service.panelLoginTarget(ctx, wallet.ID, credentials.AccountID)
	if err != nil {
		return err
	}
	var auth upstreamPanelAuthResponse
	if err := r.doJSON(ctx, account, http.MethodPost, origin, "/api/v1/auth/login", map[string]string{
		"email": credentials.Identity, "password": credentials.Password,
	}, "", &auth); err != nil {
		if strings.TrimSpace(wallet.PanelSessionCiphertext) != "" {
			r.markState(ctx, wallet, UpstreamPanelSessionStatusExpired, "relogin_rejected", nil)
		}
		return ErrUpstreamPanelLoginRejected
	}
	if auth.Requires2FA || strings.TrimSpace(auth.AccessToken) == "" {
		if strings.TrimSpace(wallet.PanelSessionCiphertext) != "" {
			r.markState(ctx, wallet, UpstreamPanelSessionStatusDegraded, "relogin_requires_2fa", nil)
		}
		return ErrUpstreamPanelLoginRejected
	}
	_, err = r.saveAuthResponse(ctx, wallet, account, origin, credentials.Identity, auth)
	return err
}

func (r *upstreamPanelSessionRuntime) check(ctx context.Context, walletID int64) (*UpstreamPanelSessionState, error) {
	wallet, err := r.service.repo.GetWallet(ctx, walletID)
	if err != nil {
		return nil, err
	}
	r.service.normalizePanelSessionState(wallet)
	if !wallet.PanelSession.Configured {
		if wallet.PanelSession.CredentialsSaved {
			if reloginErr := r.relogin(ctx, wallet); reloginErr == nil {
				wallet, err = r.service.repo.GetWallet(ctx, walletID)
				if err != nil {
					return nil, err
				}
				r.service.normalizePanelSessionState(wallet)
			}
		}
		if !wallet.PanelSession.Configured {
			state := wallet.PanelSession
			return &state, nil
		}
	}
	accounts, err := r.service.loadWalletAccounts(ctx, wallet)
	if err != nil {
		return nil, err
	}
	account, token, resolveErr := r.service.resolvePanelCredential(ctx, wallet, accounts)
	if resolveErr != nil && wallet.PanelSession.CredentialsSaved {
		if reloginErr := r.relogin(ctx, wallet); reloginErr == nil {
			wallet, err = r.service.repo.GetWallet(ctx, walletID)
			if err != nil {
				return nil, err
			}
			accounts, err = r.service.loadWalletAccounts(ctx, wallet)
			if err != nil {
				return nil, err
			}
			account, token, resolveErr = r.service.resolvePanelCredential(ctx, wallet, accounts)
		}
	}
	if resolveErr != nil {
		latest, _ := r.service.repo.GetWallet(ctx, walletID)
		if latest != nil {
			r.service.normalizePanelSessionState(latest)
			state := latest.PanelSession
			return &state, nil
		}
		return nil, resolveErr
	}
	requestErr := r.doJSON(ctx, account, http.MethodGet, normalizeUpstreamPanelOrigin(account.GetCredential("base_url")), "/api/v1/user/profile", nil, token, &map[string]any{})
	if panelRequestStatus(requestErr) == http.StatusUnauthorized {
		secret, refreshErr := r.forceRefresh(ctx, walletID)
		if refreshErr == nil {
			account = r.matchAccount(accounts, secret)
			if account != nil {
				requestErr = r.doJSON(ctx, account, http.MethodGet, secret.Origin, "/api/v1/user/profile", nil, secret.AccessToken, &map[string]any{})
			}
		}
	}
	latest, err := r.service.repo.GetWallet(ctx, walletID)
	if err != nil {
		return nil, err
	}
	secret, decryptErr := r.decryptSecret(latest.PanelSessionCiphertext)
	if decryptErr != nil {
		r.markState(ctx, latest, UpstreamPanelSessionStatusExpired, "decrypt_failed", nil)
	} else if requestErr != nil {
		status := UpstreamPanelSessionStatusDegraded
		code := "profile_check_failed"
		if panelRequestStatus(requestErr) == http.StatusUnauthorized {
			status = UpstreamPanelSessionStatusExpired
			code = "session_expired"
		}
		r.markState(ctx, latest, status, code, secret)
	} else {
		r.markState(ctx, latest, UpstreamPanelSessionStatusHealthy, "", secret)
	}
	result, err := r.service.repo.GetWallet(ctx, walletID)
	if err != nil {
		return nil, err
	}
	r.service.normalizePanelSessionState(result)
	state := result.PanelSession
	return &state, nil
}

func (r *upstreamPanelSessionRuntime) forceRefresh(ctx context.Context, walletID int64) (*upstreamPanelSessionSecret, error) {
	wallet, err := r.service.repo.GetWallet(ctx, walletID)
	if err != nil {
		return nil, err
	}
	secret, err := r.decryptSecret(wallet.PanelSessionCiphertext)
	if err != nil {
		return nil, err
	}
	secret.AccessExpiry = time.Now().UTC()
	ciphertext, err := r.encryptSecret(*secret)
	if err != nil {
		return nil, err
	}
	state := wallet.PanelSession
	state.ExpiresAt = timePointer(secret.AccessExpiry)
	swapped, err := r.repo.CompareAndSwapUpstreamPanelSession(ctx, walletID, wallet.PanelSessionCiphertext, ciphertext, state)
	if err != nil {
		return nil, err
	}
	if !swapped {
		return r.refresh(ctx, walletID)
	}
	return r.refresh(ctx, walletID)
}

func (r *upstreamPanelSessionRuntime) markState(ctx context.Context, wallet *UpstreamWallet, status, errorCode string, secret *upstreamPanelSessionSecret) {
	if wallet == nil || strings.TrimSpace(wallet.PanelSessionCiphertext) == "" {
		return
	}
	now := time.Now().UTC()
	next := now.Add(upstreamPanelHealthyCheckInterval)
	if status != UpstreamPanelSessionStatusHealthy {
		next = now.Add(upstreamPanelRetryCheckInterval)
	}
	state := wallet.PanelSession
	state.Configured = true
	state.EncryptionKeyConfigured = r.keyConfigured
	state.Status = status
	state.LastCheckedAt = &now
	state.NextCheckAt = &next
	state.LastError = errorCode
	if secret != nil {
		state.ExpiresAt = timePointer(secret.AccessExpiry)
	}
	_, err := r.repo.CompareAndSwapUpstreamPanelSession(ctx, wallet.ID, wallet.PanelSessionCiphertext, wallet.PanelSessionCiphertext, state)
	if err != nil {
		slog.Warn("upstream panel session state update failed", "wallet_id", wallet.ID, "error", err)
	}
}

func (r *upstreamPanelSessionRuntime) doJSON(ctx context.Context, account *Account, method, origin, endpoint string, payload any, accessToken string, destination any) error {
	provider := r.serviceProvider()
	if provider == nil || provider.accountTestService == nil || provider.accountTestService.httpUpstream == nil || account == nil {
		return &upstreamPanelRequestError{code: "request_unavailable"}
	}
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return &upstreamPanelRequestError{code: "request_build_failed"}
		}
		body = bytes.NewReader(encoded)
	}
	requestCtx, cancel := context.WithTimeout(ctx, upstreamPanelRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, method, buildSub2APIPanelEndpointURL(origin, endpoint), body)
	if err != nil {
		return &upstreamPanelRequestError{code: "request_build_failed"}
	}
	req = req.WithContext(WithHTTPUpstreamRedirectsDisabled(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileDefault)))
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(accessToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	}
	proxyURL := ""
	if account.ProxyID != nil {
		if account.Proxy == nil || account.Proxy.ID != *account.ProxyID {
			return &upstreamPanelRequestError{code: "proxy_unavailable"}
		}
		proxyURL = account.Proxy.URL()
	}
	var tlsProfile *tlsfingerprint.Profile
	if provider.accountTestService.tlsFPProfileService != nil {
		tlsProfile = provider.accountTestService.tlsFPProfileService.ResolveTLSProfile(account)
	}
	resp, err := provider.accountTestService.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, tlsProfile)
	if err != nil {
		return &upstreamPanelRequestError{code: "request_failed"}
	}
	if resp == nil || resp.Body == nil {
		return &upstreamPanelRequestError{code: "empty_response"}
	}
	defer func() { _ = resp.Body.Close() }()
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, upstreamPanelMaxResponseBody+1))
	if err != nil || int64(len(bodyBytes)) > upstreamPanelMaxResponseBody {
		return &upstreamPanelRequestError{code: "response_read_failed", statusCode: resp.StatusCode}
	}
	if httputil.IsCloudflareChallengeResponse(resp.StatusCode, resp.Header, bodyBytes) {
		return &upstreamPanelRequestError{code: "cloudflare_challenge", statusCode: resp.StatusCode}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &upstreamPanelRequestError{code: fmt.Sprintf("http_%d", resp.StatusCode), statusCode: resp.StatusCode}
	}
	var envelope sub2APIEnvelope
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		return &upstreamPanelRequestError{code: "non_json_response", statusCode: resp.StatusCode}
	}
	if envelope.Code != 0 {
		return &upstreamPanelRequestError{code: "response_rejected", statusCode: resp.StatusCode}
	}
	if destination == nil {
		return nil
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return &upstreamPanelRequestError{code: "response_data_missing", statusCode: resp.StatusCode}
	}
	if err := json.Unmarshal(envelope.Data, destination); err != nil {
		return &upstreamPanelRequestError{code: "invalid_response", statusCode: resp.StatusCode}
	}
	return nil
}

func panelRequestStatus(err error) int {
	var requestErr *upstreamPanelRequestError
	if errors.As(err, &requestErr) {
		return requestErr.statusCode
	}
	return 0
}

func panelLoginRequiresManualImport(err error) bool {
	var requestErr *upstreamPanelRequestError
	if !errors.As(err, &requestErr) {
		return false
	}
	return requestErr.statusCode == http.StatusForbidden || requestErr.code == "cloudflare_challenge" || requestErr.code == "non_json_response" || requestErr.code == "invalid_response" || requestErr.code == "response_rejected"
}

func normalizeUpstreamPanelOrigin(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(trimmed, "/api/v1") {
		trimmed = strings.TrimSuffix(trimmed, "/api/v1")
	} else if strings.HasSuffix(trimmed, "/v1") {
		trimmed = strings.TrimSuffix(trimmed, "/v1")
	}
	return trimmed
}

func nextUpstreamPanelCheck(now, expiresAt time.Time) time.Time {
	next := now.Add(upstreamPanelHealthyCheckInterval)
	refreshAt := expiresAt.Add(-upstreamPanelRefreshWindow)
	if refreshAt.Before(next) {
		next = refreshAt
	}
	if !next.After(now) {
		next = now.Add(upstreamPanelRetryCheckInterval)
	}
	return next
}

func (r *upstreamPanelSessionRuntime) Start() {
	if r == nil || r.repo == nil {
		return
	}
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(upstreamPanelRunnerInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.runOnce()
			case <-r.stopCh:
				return
			}
		}
	}()
}

func (r *upstreamPanelSessionRuntime) Stop() {
	if r == nil {
		return
	}
	r.stopOnce.Do(func() { close(r.stopCh) })
	r.wg.Wait()
}

func (r *upstreamPanelSessionRuntime) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), upstreamPanelRunnerTimeout)
	defer cancel()
	release, acquired := tryAcquireSingletonLeaderLock(ctx, r.lockCache, r.db, upstreamPanelLeaderLockKey, r.instanceID, upstreamPanelLeaderLockTTL)
	if !acquired {
		return
	}
	defer release()
	ids, err := r.repo.ListDueUpstreamPanelSessionWalletIDs(ctx, time.Now().UTC(), upstreamPanelRunnerBatchSize)
	if err != nil {
		slog.Warn("upstream panel session probe list failed", "error", err)
		return
	}
	sem := make(chan struct{}, upstreamPanelRunnerConcurrency)
	var wg sync.WaitGroup
	for _, walletID := range ids {
		walletID := walletID
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			if _, err := r.check(ctx, walletID); err != nil && ctx.Err() == nil {
				slog.Warn("upstream panel session probe failed", "wallet_id", walletID, "error", err)
			}
		}()
	}
	wg.Wait()
}
