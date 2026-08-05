package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	OpenAIAPIKeyAccountRoutesFileEnv         = "OPENAI_API_KEY_ACCOUNT_ROUTES_FILE"
	openAIAPIKeyAccountRoutesFilename        = "openai-api-key-account-routes.json"
	installedOpenAIAPIKeyAccountRoutesDir    = "/opt/sub2api/data"
	defaultOpenAIAPIKeyAccountReloadInterval = time.Second
)

type openAIAPIKeyAccountRouteSnapshot struct {
	routes map[int64]int64
	err    error
}

// OpenAIAPIKeyAccountRouteStore keeps the request hot path in memory while a
// small local JSON file remains the persistent, locally editable source.
type OpenAIAPIKeyAccountRouteStore struct {
	path           string
	reloadInterval time.Duration
	snapshot       atomic.Pointer[openAIAPIKeyAccountRouteSnapshot]

	fileMu      sync.Mutex
	lastHash    [sha256.Size]byte
	hasLastHash bool
	lastLoadErr error

	startOnce sync.Once
	stopOnce  sync.Once
	started   atomic.Bool
	startErr  error
	stopCh    chan struct{}
	doneCh    chan struct{}
}

func ResolveOpenAIAPIKeyAccountRoutesPath() string {
	if configured := strings.TrimSpace(os.Getenv(OpenAIAPIKeyAccountRoutesFileEnv)); configured != "" {
		return filepath.Clean(configured)
	}
	if dataDir := strings.TrimSpace(os.Getenv("DATA_DIR")); dataDir != "" {
		return filepath.Join(dataDir, openAIAPIKeyAccountRoutesFilename)
	}
	if info, err := os.Stat(installedOpenAIAPIKeyAccountRoutesDir); err == nil && info.IsDir() {
		return filepath.Join(installedOpenAIAPIKeyAccountRoutesDir, openAIAPIKeyAccountRoutesFilename)
	}
	return filepath.Join("data", openAIAPIKeyAccountRoutesFilename)
}

func NewOpenAIAPIKeyAccountRouteStore(path string) *OpenAIAPIKeyAccountRouteStore {
	if strings.TrimSpace(path) == "" {
		path = ResolveOpenAIAPIKeyAccountRoutesPath()
	}
	store := &OpenAIAPIKeyAccountRouteStore{
		path:           filepath.Clean(path),
		reloadInterval: defaultOpenAIAPIKeyAccountReloadInterval,
		stopCh:         make(chan struct{}),
		doneCh:         make(chan struct{}),
	}
	store.snapshot.Store(&openAIAPIKeyAccountRouteSnapshot{routes: map[int64]int64{}})
	return store
}

// Start loads the file once and then watches it in the background. Any invalid
// file state publishes an empty map so the normal scheduler remains active.
func (s *OpenAIAPIKeyAccountRouteStore) Start() error {
	if s == nil {
		return errors.New("OpenAI API key account route store is nil")
	}
	s.startOnce.Do(func() {
		_, s.startErr = s.reload()
		s.started.Store(true)
		go s.watch()
	})
	return s.startErr
}

func (s *OpenAIAPIKeyAccountRouteStore) Close() {
	if s == nil || !s.started.Load() {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	<-s.doneCh
}

func (s *OpenAIAPIKeyAccountRouteStore) watch() {
	defer close(s.doneCh)
	interval := s.reloadInterval
	if interval <= 0 {
		interval = defaultOpenAIAPIKeyAccountReloadInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			changed, err := s.reload()
			if !changed {
				continue
			}
			if err != nil {
				slog.Warn("openai api key account routes reload failed; normal scheduling remains active", "path", s.path, "error", err)
				continue
			}
			slog.Info("openai api key account routes reloaded", "path", s.path, "count", s.routeCount())
		case <-s.stopCh:
			return
		}
	}
}

func (s *OpenAIAPIKeyAccountRouteStore) Get(apiKeyID int64) (int64, bool, error) {
	if s == nil || apiKeyID <= 0 {
		return 0, false, nil
	}
	snapshot := s.snapshot.Load()
	if snapshot == nil {
		return 0, false, errors.New("OpenAI API key account routes are not initialized")
	}
	if snapshot.err != nil {
		return 0, false, snapshot.err
	}
	accountID, configured := snapshot.routes[apiKeyID]
	return accountID, configured, nil
}

func (s *OpenAIAPIKeyAccountRouteStore) List(ctx context.Context) (map[int64]int64, error) {
	if s == nil {
		return map[int64]int64{}, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	routes, raw, err := readOpenAIAPIKeyAccountRoutesFile(s.path)
	if err != nil {
		return nil, err
	}
	s.publishLocked(raw, routes)
	return cloneOpenAIAPIKeyAccountRoutes(routes), nil
}

func (s *OpenAIAPIKeyAccountRouteStore) Set(ctx context.Context, apiKeyID, accountID int64) error {
	if apiKeyID <= 0 || accountID <= 0 {
		return errors.New("api key id and account id must be positive")
	}
	return s.update(ctx, func(routes map[int64]int64) {
		routes[apiKeyID] = accountID
	})
}

func (s *OpenAIAPIKeyAccountRouteStore) Delete(ctx context.Context, apiKeyID int64) error {
	if apiKeyID <= 0 {
		return errors.New("api key id must be positive")
	}
	return s.update(ctx, func(routes map[int64]int64) {
		delete(routes, apiKeyID)
	})
}

func (s *OpenAIAPIKeyAccountRouteStore) update(ctx context.Context, mutate func(map[int64]int64)) error {
	if s == nil {
		return errors.New("OpenAI API key account route store is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	routes, _, err := readOpenAIAPIKeyAccountRoutesFile(s.path)
	if err != nil {
		return err
	}
	mutate(routes)
	raw, err := encodeOpenAIAPIKeyAccountRoutes(routes)
	if err != nil {
		return err
	}
	if err := writeOpenAIAPIKeyAccountRoutesFile(s.path, raw); err != nil {
		return err
	}
	s.publishLocked(raw, routes)
	return nil
}

func (s *OpenAIAPIKeyAccountRouteStore) reload() (bool, error) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	routes, raw, err := readOpenAIAPIKeyAccountRoutesFile(s.path)
	if err != nil {
		changed := s.lastLoadErr == nil || s.lastLoadErr.Error() != err.Error()
		s.lastLoadErr = err
		s.snapshot.Store(&openAIAPIKeyAccountRouteSnapshot{routes: map[int64]int64{}})
		return changed, err
	}
	hash := sha256.Sum256(raw)
	if s.hasLastHash && hash == s.lastHash && s.lastLoadErr == nil {
		return false, s.lastLoadErr
	}
	s.lastHash = hash
	s.hasLastHash = true
	if routes == nil {
		err = errors.New("OpenAI API key account routes decoded to nil")
		s.lastLoadErr = err
		s.snapshot.Store(&openAIAPIKeyAccountRouteSnapshot{routes: map[int64]int64{}})
		return true, err
	}
	s.lastLoadErr = nil
	s.snapshot.Store(&openAIAPIKeyAccountRouteSnapshot{routes: routes})
	return true, nil
}

func (s *OpenAIAPIKeyAccountRouteStore) publishLocked(raw []byte, routes map[int64]int64) {
	s.lastHash = sha256.Sum256(raw)
	s.hasLastHash = true
	s.lastLoadErr = nil
	s.snapshot.Store(&openAIAPIKeyAccountRouteSnapshot{routes: cloneOpenAIAPIKeyAccountRoutes(routes)})
}

func (s *OpenAIAPIKeyAccountRouteStore) routeCount() int {
	snapshot := s.snapshot.Load()
	if snapshot == nil {
		return 0
	}
	return len(snapshot.routes)
}

func readOpenAIAPIKeyAccountRoutesFile(path string) (map[int64]int64, []byte, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		raw = []byte("{}\n")
	} else if err != nil {
		return nil, nil, fmt.Errorf("read OpenAI API key account routes file: %w", err)
	}
	routes := make(map[string]int64)
	if err := json.Unmarshal(raw, &routes); err != nil {
		return nil, raw, fmt.Errorf("decode OpenAI API key account routes file: %w", err)
	}
	result := make(map[int64]int64, len(routes))
	for apiKeyID, accountID := range routes {
		parsedAPIKeyID, err := strconv.ParseInt(apiKeyID, 10, 64)
		if err != nil || parsedAPIKeyID <= 0 || accountID <= 0 {
			return nil, raw, fmt.Errorf("invalid OpenAI API key account route %q=%d", apiKeyID, accountID)
		}
		result[parsedAPIKeyID] = accountID
	}
	return result, raw, nil
}

func encodeOpenAIAPIKeyAccountRoutes(routes map[int64]int64) ([]byte, error) {
	encoded := make(map[string]int64, len(routes))
	for apiKeyID, accountID := range routes {
		if apiKeyID <= 0 || accountID <= 0 {
			return nil, errors.New("api key id and account id must be positive")
		}
		encoded[strconv.FormatInt(apiKeyID, 10)] = accountID
	}
	raw, err := json.MarshalIndent(encoded, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode OpenAI API key account routes file: %w", err)
	}
	return append(raw, '\n'), nil
}

func writeOpenAIAPIKeyAccountRoutesFile(path string, raw []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create OpenAI API key account routes directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".openai-api-key-account-routes-*")
	if err != nil {
		return fmt.Errorf("create temporary OpenAI API key account routes file: %w", err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("set OpenAI API key account routes file permissions: %w", err)
	}
	if _, err := temp.Write(raw); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write OpenAI API key account routes file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync OpenAI API key account routes file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close OpenAI API key account routes file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace OpenAI API key account routes file: %w", err)
	}
	return nil
}

func cloneOpenAIAPIKeyAccountRoutes(routes map[int64]int64) map[int64]int64 {
	cloned := make(map[int64]int64, len(routes))
	for apiKeyID, accountID := range routes {
		cloned[apiKeyID] = accountID
	}
	return cloned
}

func (s *SettingService) StartOpenAIAPIKeyAccountRouteReload() error {
	if s == nil || s.openAIAPIKeyAccountRouteStore == nil {
		return errors.New("OpenAI API key account route store is not configured")
	}
	return s.openAIAPIKeyAccountRouteStore.Start()
}

func (s *SettingService) GetOpenAIAPIKeyAccountRoutes(ctx context.Context) (map[int64]int64, error) {
	if s == nil || s.openAIAPIKeyAccountRouteStore == nil {
		return map[int64]int64{}, nil
	}
	return s.openAIAPIKeyAccountRouteStore.List(ctx)
}

func (s *SettingService) GetOpenAIAPIKeyAccountRoute(_ context.Context, apiKeyID int64) (int64, bool, error) {
	if s == nil || s.openAIAPIKeyAccountRouteStore == nil {
		return 0, false, nil
	}
	return s.openAIAPIKeyAccountRouteStore.Get(apiKeyID)
}

func (s *SettingService) SetOpenAIAPIKeyAccountRoute(ctx context.Context, apiKeyID, accountID int64) error {
	if s == nil || s.openAIAPIKeyAccountRouteStore == nil {
		return errors.New("OpenAI API key account route store is not configured")
	}
	return s.openAIAPIKeyAccountRouteStore.Set(ctx, apiKeyID, accountID)
}

func (s *SettingService) DeleteOpenAIAPIKeyAccountRoute(ctx context.Context, apiKeyID int64) error {
	if s == nil || s.openAIAPIKeyAccountRouteStore == nil {
		return errors.New("OpenAI API key account route store is not configured")
	}
	return s.openAIAPIKeyAccountRouteStore.Delete(ctx, apiKeyID)
}
