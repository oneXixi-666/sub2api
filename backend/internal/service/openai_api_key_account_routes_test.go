package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func newTestOpenAIAPIKeyAccountRouteSettings(t *testing.T) (*SettingService, *OpenAIAPIKeyAccountRouteStore) {
	t.Helper()
	store := NewOpenAIAPIKeyAccountRouteStore(filepath.Join(t.TempDir(), "routes.json"))
	settings := NewSettingService(nil, nil)
	settings.openAIAPIKeyAccountRouteStore = store
	t.Cleanup(store.Close)
	return settings, store
}

func TestOpenAIAPIKeyAccountRoutesCRUD(t *testing.T) {
	settings, _ := newTestOpenAIAPIKeyAccountRouteSettings(t)
	ctx := context.Background()

	require.NoError(t, settings.SetOpenAIAPIKeyAccountRoute(ctx, 10, 100))
	require.NoError(t, settings.SetOpenAIAPIKeyAccountRoute(ctx, 20, 200))
	routes, err := settings.GetOpenAIAPIKeyAccountRoutes(ctx)
	require.NoError(t, err)
	require.Equal(t, map[int64]int64{10: 100, 20: 200}, routes)

	require.NoError(t, settings.DeleteOpenAIAPIKeyAccountRoute(ctx, 10))
	accountID, configured, err := settings.GetOpenAIAPIKeyAccountRoute(ctx, 10)
	require.NoError(t, err)
	require.False(t, configured)
	require.Zero(t, accountID)
	require.NoError(t, settings.DeleteOpenAIAPIKeyAccountRoute(ctx, 20))
	routes, err = settings.GetOpenAIAPIKeyAccountRoutes(ctx)
	require.NoError(t, err)
	require.Empty(t, routes)
}

func TestOpenAIAPIKeyAccountRouteHotReloadKeepsRequestsOffDatabase(t *testing.T) {
	settings, store := newTestOpenAIAPIKeyAccountRouteSettings(t)
	store.reloadInterval = 10 * time.Millisecond
	require.NoError(t, store.Start())
	ctx := context.Background()
	require.NoError(t, settings.SetOpenAIAPIKeyAccountRoute(ctx, 900, 101))

	require.Eventually(t, func() bool {
		accountID, configured, err := settings.GetOpenAIAPIKeyAccountRoute(ctx, 900)
		return err == nil && configured && accountID == 101
	}, time.Second, 10*time.Millisecond)

	raw := []byte(`{"900":102}
`)
	require.NoError(t, os.WriteFile(store.path, raw, 0o600))
	require.Eventually(t, func() bool {
		accountID, configured, err := settings.GetOpenAIAPIKeyAccountRoute(ctx, 900)
		return err == nil && configured && accountID == 102
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, os.WriteFile(store.path, []byte(`{"900":`), 0o600))
	require.Eventually(t, func() bool {
		accountID, configured, err := settings.GetOpenAIAPIKeyAccountRoute(ctx, 900)
		return err == nil && !configured && accountID == 0
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, os.WriteFile(store.path, raw, 0o600))
	require.Eventually(t, func() bool {
		accountID, configured, err := settings.GetOpenAIAPIKeyAccountRoute(ctx, 900)
		return err == nil && configured && accountID == 102
	}, time.Second, 10*time.Millisecond)
}

func TestOpenAIAPIKeyAccountRouteOnlyOverridesNativeOpenAIScheduling(t *testing.T) {
	groupID := int64(7)
	settings, store := newTestOpenAIAPIKeyAccountRouteSettings(t)
	require.NoError(t, store.Set(context.Background(), 900, 102))
	accounts := []Account{
		{ID: 101, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{groupID}},
		{ID: 102, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{groupID}},
		{ID: 103, Platform: PlatformGrok, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{groupID}},
	}
	svc := &OpenAIGatewayService{
		accountRepo:    stubOpenAIAccountRepo{accounts: accounts},
		settingService: settings,
		cfg:            &config.Config{RunMode: config.RunModeStandard},
	}
	ctx := context.WithValue(context.Background(), ctxkey.APIKeyID, int64(900))

	selection, handled, err := svc.selectOpenAIAPIKeyAccountRoute(ctx, OpenAIAccountScheduleRequest{
		GroupID:           &groupID,
		Platform:          PlatformOpenAI,
		RequestedModel:    "gpt-test",
		RequiredTransport: OpenAIUpstreamTransportAny,
	})
	require.NoError(t, err)
	require.True(t, handled)
	require.NotNil(t, selection)
	require.Equal(t, int64(102), selection.Account.ID)

	_, handled, err = svc.selectOpenAIAPIKeyAccountRoute(
		context.WithValue(context.Background(), ctxkey.APIKeyID, int64(901)),
		OpenAIAccountScheduleRequest{GroupID: &groupID, Platform: PlatformOpenAI, RequestedModel: "gpt-test", RequiredTransport: OpenAIUpstreamTransportAny},
	)
	require.NoError(t, err)
	require.False(t, handled)

	_, handled, err = svc.selectOpenAIAPIKeyAccountRoute(ctx, OpenAIAccountScheduleRequest{
		GroupID:           &groupID,
		Platform:          PlatformGrok,
		RequestedModel:    "grok-test",
		RequiredTransport: OpenAIUpstreamTransportAny,
	})
	require.NoError(t, err)
	require.False(t, handled)
}

func TestOpenAIAPIKeyAccountRouteAllowsAccountOutsideCurrentGroup(t *testing.T) {
	requestedGroupID := int64(7)
	otherGroupID := int64(8)
	settings, store := newTestOpenAIAPIKeyAccountRouteSettings(t)
	require.NoError(t, store.Set(context.Background(), 900, 201))
	svc := &OpenAIGatewayService{
		accountRepo: stubOpenAIAccountRepo{accounts: []Account{
			{ID: 201, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{otherGroupID}},
			{ID: 202, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{requestedGroupID}},
		}},
		settingService: settings,
		cfg:            &config.Config{RunMode: config.RunModeStandard},
	}
	ctx := context.WithValue(context.Background(), ctxkey.APIKeyID, int64(900))

	selection, handled, err := svc.selectOpenAIAPIKeyAccountRoute(ctx, OpenAIAccountScheduleRequest{
		GroupID:           &requestedGroupID,
		Platform:          PlatformOpenAI,
		RequestedModel:    "gpt-test",
		RequiredTransport: OpenAIUpstreamTransportAny,
	})
	require.NoError(t, err)
	require.True(t, handled)
	require.NotNil(t, selection)
	require.Equal(t, int64(201), selection.Account.ID)
}

func TestOpenAIAPIKeyAccountRouteInitialConfigErrorFallsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routes.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"900":`), 0o600))
	store := NewOpenAIAPIKeyAccountRouteStore(path)
	store.reloadInterval = 10 * time.Millisecond
	settings := NewSettingService(nil, nil)
	settings.openAIAPIKeyAccountRouteStore = store
	t.Cleanup(store.Close)
	require.Error(t, store.Start())

	svc := &OpenAIGatewayService{settingService: settings}
	ctx := context.WithValue(context.Background(), ctxkey.APIKeyID, int64(900))
	selection, handled, err := svc.selectOpenAIAPIKeyAccountRoute(ctx, OpenAIAccountScheduleRequest{Platform: PlatformOpenAI})
	require.NoError(t, err)
	require.False(t, handled)
	require.Nil(t, selection)
}
