package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

// cyberOrderingTestRepo records the sequence of repo calls to verify F7 ordering.
type cyberOrderingTestRepo struct {
	mu         sync.Mutex
	calls      []string
	emailSents []bool // EmailSent value captured at each CreateLog call
}

func (r *cyberOrderingTestRepo) CreateLog(ctx context.Context, log *ContentModerationLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, "create")
	if log != nil {
		r.emailSents = append(r.emailSents, log.EmailSent)
		log.ID = 1 // simulate DB-assigned ID so UpdateLogEmailSent guard passes
	}
	return nil
}

func (r *cyberOrderingTestRepo) UpdateLogEmailSent(ctx context.Context, id int64, sent bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, "update_email_sent")
	return nil
}

func (r *cyberOrderingTestRepo) ListLogs(ctx context.Context, filter ContentModerationLogFilter) ([]ContentModerationLog, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *cyberOrderingTestRepo) CountFlaggedByUserSince(ctx context.Context, userID int64, since time.Time, excludeCyberPolicy bool) (int, error) {
	return 0, nil
}

func (r *cyberOrderingTestRepo) CountCyberPolicyByUserAndGroupSince(ctx context.Context, userID int64, groupID *int64, since time.Time) (int, error) {
	return 0, nil
}

func (r *cyberOrderingTestRepo) CleanupExpiredLogs(ctx context.Context, hitBefore time.Time, nonHitBefore time.Time) (*ContentModerationCleanupResult, error) {
	return &ContentModerationCleanupResult{}, nil
}

func (r *cyberOrderingTestRepo) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	copy(out, r.calls)
	return out
}

func (r *cyberOrderingTestRepo) snapshotEmailSents() []bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]bool, len(r.emailSents))
	copy(out, r.emailSents)
	return out
}

func TestRecordCyberPolicyEvent_DisabledWhenRiskControlOff(t *testing.T) {
	repo := &contentModerationTestRepo{}
	svc := NewContentModerationService(
		&contentModerationTestSettingRepo{values: map[string]string{
			SettingKeyRiskControlEnabled: "false",
		}},
		repo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{
		UserID:          1,
		UserEmail:       "u@x.com",
		Model:           "gpt-5",
		Endpoint:        "/v1/responses",
		UpstreamMessage: "flagged",
		UpstreamBody:    `{"error":{"code":"cyber_policy"}}`,
		UpstreamStatus:  400,
	})

	require.Empty(t, repo.snapshotLogs(), "CreateLog must NOT be called when risk_control_enabled is off")
}

func TestRecordCyberPolicyEvent_WritesLogWhenEnabled(t *testing.T) {
	repo := &contentModerationTestRepo{}
	svc := NewContentModerationService(
		&contentModerationTestSettingRepo{values: map[string]string{
			SettingKeyRiskControlEnabled: "true",
		}},
		repo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil, // emailService=nil: email path safely skipped
	)

	svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{
		UserID:          1,
		UserEmail:       "u@x.com",
		Model:           "gpt-5",
		Endpoint:        "/v1/responses",
		Protocol:        ContentModerationProtocolOpenAIResponses,
		AuditStage:      "first_turn",
		TurnNumber:      1,
		InputSnapshot:   "[system]\npolicy\n\n[user]\nrun exploit token=abc123456789xyz",
		InputHash:       strings.Repeat("a", 64),
		InputLength:     57,
		MessageCount:    2,
		InputTruncated:  true,
		UpstreamMessage: "flagged",
		UpstreamBody:    `{"error":{"code":"cyber_policy"}}`,
		UpstreamStatus:  400,
	})

	logs := repo.snapshotLogs()
	require.Len(t, logs, 1)
	log := logs[0]

	require.Equal(t, "cyber_policy", log.Action)
	require.Equal(t, ContentModerationCyberModeEnforce, log.CyberPolicyMode)
	require.Equal(t, ContentModerationCyberPolicySourceDefault, log.CyberPolicySource)
	require.NotNil(t, log.CyberPolicySnapshot)
	require.True(t, log.CyberPolicySnapshot.Policy.ViolationCountEnabled)
	require.True(t, log.Flagged)
	require.Equal(t, "cyber_policy", log.HighestCategory)
	require.Contains(t, log.Error, "flagged")
	require.False(t, log.AutoBanned)
	// emailService is nil, so EmailSent must be false
	require.False(t, log.EmailSent)

	// UserID pointer must be set
	require.NotNil(t, log.UserID)
	require.Equal(t, int64(1), *log.UserID)

	// score for cyber_policy is always 1.0
	require.Equal(t, 1.0, log.HighestScore)

	// mode must be post_upstream
	require.Equal(t, "post_upstream", log.Mode)

	// provider
	require.Equal(t, "openai", log.Provider)

	// model
	require.Equal(t, "gpt-5", log.Model)

	// endpoint
	require.Equal(t, "/v1/responses", log.Endpoint)
	require.Equal(t, ContentModerationProtocolOpenAIResponses, log.Protocol)
	require.Equal(t, "first_turn", log.AuditStage)
	require.Equal(t, 1, log.TurnNumber)
	require.Equal(t, strings.Repeat("a", 64), log.InputHash)
	require.Equal(t, 57, log.InputLength)
	require.Equal(t, 2, log.MessageCount)
	require.True(t, log.InputTruncated)
	require.Contains(t, log.InputExcerpt, "run exploit")
	require.Contains(t, log.InputSnapshot, "[system]")
	require.NotContains(t, log.InputSnapshot, "abc123456789xyz")
	require.Empty(t, log.MatchedKeyword, "upstream cyber evidence must not be presented as an exact keyword hit")

	// violation count >= 1 (side-effects ran)
	require.GreaterOrEqual(t, log.ViolationCount, 1)

	// Error field should also contain the upstream body JSON
	require.True(t, strings.Contains(log.Error, "cyber_policy") || strings.Contains(log.Error, "flagged"),
		"Error should mention flagged or cyber_policy")
}

func TestRecordCyberPolicyEvent_DedupesRetriesBeforeAllSideEffects(t *testing.T) {
	group7, group8 := int64(7), int64(8)
	repo := &banCountArgsTestRepo{}
	cache := &contentModerationTestHashCache{}
	svc := NewContentModerationService(&contentModerationTestSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled: "true",
	}}, repo, cache, nil, nil, nil, nil, nil)
	base := CyberPolicyRecordInput{
		UserID: 741, APIKeyID: 99, GroupID: &group7,
		InputHash: strings.Repeat("b", 64), InputSnapshot: "[user]\nsame request",
	}

	svc.RecordCyberPolicyEvent(context.Background(), base)
	svc.RecordCyberPolicyEvent(context.Background(), base)

	require.Len(t, repo.snapshotLogs(), 1, "a retry must not create another log row")
	require.Len(t, repo.snapshotCyberCountCalls(), 1, "a retry must not increment the violation counter")

	base.GroupID = &group8
	svc.RecordCyberPolicyEvent(context.Background(), base)
	require.Len(t, repo.snapshotLogs(), 2, "different groups must not share a dedupe key")
	require.Len(t, repo.snapshotCyberCountCalls(), 2)
}

func TestRecordCyberPolicyEvent_DedupeFailureFailsOpenForEvidence(t *testing.T) {
	repo := &banCountArgsTestRepo{}
	cache := &contentModerationTestHashCache{eventErr: errors.New("redis unavailable")}
	svc := NewContentModerationService(&contentModerationTestSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled: "true",
	}}, repo, cache, nil, nil, nil, nil, nil)

	svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{
		UserID: 741, InputHash: strings.Repeat("c", 64), InputSnapshot: "[user]\nevidence",
	})

	require.Len(t, repo.snapshotLogs(), 1, "Redis failure must not discard upstream evidence")
	require.Len(t, repo.snapshotCyberCountCalls(), 1)
}

func TestRecordCyberPolicyEvent_UsesFrozenResolvedPolicy(t *testing.T) {
	repo := &contentModerationTestRepo{}
	groupID := int64(7)
	svc := NewContentModerationService(
		&contentModerationTestSettingRepo{values: map[string]string{
			SettingKeyRiskControlEnabled: "false",
		}},
		repo, nil, nil, nil, nil, nil, nil,
	)
	frozen := ResolvedCyberPolicy{
		Version: cyberPolicySnapshotVersion,
		Source:  ContentModerationCyberPolicySourceGroupOverride,
		Policy: CyberPolicySettings{
			Mode: ContentModerationCyberModeCollect,
		},
	}

	svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{
		UserID:         1,
		GroupID:        &groupID,
		ResolvedPolicy: &frozen,
	})

	logs := repo.snapshotLogs()
	require.Len(t, logs, 1)
	require.Equal(t, ContentModerationCyberModeCollect, logs[0].CyberPolicyMode)
	require.Equal(t, ContentModerationCyberPolicySourceGroupOverride, logs[0].CyberPolicySource)
	require.Equal(t, frozen, *logs[0].CyberPolicySnapshot)
	require.Zero(t, logs[0].ViolationCount, "a frozen collect-only hit must not enter the ban counter")
}

func TestRecordCyberPolicyEvent_UsesIndependentEnforcementScope(t *testing.T) {
	groupID := int64(7)
	tests := []struct {
		name           string
		config         string
		groupID        *int64
		model          string
		wantCyberCount bool
		wantLogs       int
		wantMode       string
		wantBanned     bool
	}{
		{
			name:           "ordinary moderation group scope does not limit cyber enforcement",
			config:         `{"all_groups":false,"group_ids":[8],"ban_threshold":1}`,
			groupID:        &groupID,
			model:          "gpt-5",
			wantCyberCount: true,
			wantLogs:       1,
			wantMode:       ContentModerationCyberModeEnforce,
			wantBanned:     true,
		},
		{
			name:     "group outside cyber scope is collected without enforcement",
			config:   `{"cyber_policy_enforce_all_groups":false,"cyber_policy_enforce_group_ids":[8],"ban_threshold":1}`,
			groupID:  &groupID,
			model:    "gpt-5",
			wantLogs: 1,
			wantMode: ContentModerationCyberModeCollect,
		},
		{
			name:     "ungrouped request outside selected cyber scope is collected",
			config:   `{"cyber_policy_enforce_all_groups":false,"cyber_policy_enforce_group_ids":[7],"ban_threshold":1}`,
			groupID:  nil,
			model:    "gpt-5",
			wantLogs: 1,
			wantMode: ContentModerationCyberModeCollect,
		},
		{
			name:           "proactive model filter does not limit cyber collection",
			config:         `{"all_groups":true,"model_filter":{"type":"include","models":["gpt-4o"]},"ban_threshold":1}`,
			groupID:        &groupID,
			model:          "gpt-5",
			wantCyberCount: true,
			wantLogs:       1,
			wantMode:       ContentModerationCyberModeEnforce,
			wantBanned:     true,
		},
		{
			name:           "group inside selected cyber scope is enforced",
			config:         `{"enabled":false,"mode":"off","sample_rate":0,"cyber_policy_enforce_all_groups":false,"cyber_policy_enforce_group_ids":[7],"model_filter":{"type":"include","models":["gpt-5"]},"ban_threshold":1}`,
			groupID:        &groupID,
			model:          "gpt-5",
			wantCyberCount: true,
			wantLogs:       1,
			wantMode:       ContentModerationCyberModeEnforce,
			wantBanned:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &banCountArgsTestRepo{}
			userRepo := &contentModerationTestUserRepo{user: &User{ID: 1, Role: RoleUser, Status: StatusActive}}
			svc := NewContentModerationService(
				&contentModerationTestSettingRepo{values: map[string]string{
					SettingKeyRiskControlEnabled:      "true",
					SettingKeyContentModerationConfig: tt.config,
				}},
				repo, nil, nil, userRepo, nil, nil, nil,
			)

			svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{
				UserID:  1,
				GroupID: tt.groupID,
				Model:   tt.model,
			})

			require.Equal(t, tt.wantCyberCount, len(repo.snapshotCyberCountCalls()) > 0)
			logs := repo.snapshotLogs()
			require.Len(t, logs, tt.wantLogs)
			if tt.wantLogs > 0 {
				require.Equal(t, tt.wantMode, logs[0].CyberPolicyMode)
			}
			require.Equal(t, tt.wantBanned, userRepo.user.Status == StatusDisabled)
			if tt.wantBanned {
				require.Len(t, userRepo.updated, 1)
			} else {
				require.Empty(t, userRepo.updated)
			}
		})
	}
}

func TestCyberPolicyEnforcementScopeDefaultsAndNormalizes(t *testing.T) {
	legacy, err := parseContentModerationConfig(`{"all_groups":false,"group_ids":[7]}`)
	require.NoError(t, err)
	require.True(t, legacy.CyberPolicyEnforceAllGroups, "legacy configuration must preserve all-group enforcement")
	require.Equal(t, ContentModerationCyberModeEnforce, legacy.CyberPolicyDefaultPolicy.Mode)
	require.Empty(t, legacy.CyberPolicyEnforceGroupIDs)

	selected, err := parseContentModerationConfig(`{"cyber_policy_enforce_all_groups":false,"cyber_policy_enforce_group_ids":[9,7,9,0,-1]}`)
	require.NoError(t, err)
	require.False(t, selected.CyberPolicyEnforceAllGroups)
	require.Equal(t, []int64{7, 9}, selected.CyberPolicyEnforceGroupIDs)
	require.Equal(t, ContentModerationCyberModeCollect, selected.CyberPolicyDefaultPolicy.Mode)
	require.Len(t, selected.CyberPolicyGroupPolicies, 2)
	require.Equal(t, int64(7), selected.CyberPolicyGroupPolicies[0].GroupID)
	require.Equal(t, ContentModerationCyberModeEnforce, selected.CyberPolicyGroupPolicies[0].Policy.Mode)
}

func TestShouldEnforceCyberPolicyForGroup(t *testing.T) {
	group7 := int64(7)
	group8 := int64(8)
	tests := []struct {
		name      string
		risk      string
		config    string
		groupID   *int64
		wantAllow bool
	}{
		{name: "default all groups", risk: "true", groupID: &group8, wantAllow: true},
		{name: "selected group", risk: "true", config: `{"cyber_policy_enforce_all_groups":false,"cyber_policy_enforce_group_ids":[7]}`, groupID: &group7, wantAllow: true},
		{name: "unselected group", risk: "true", config: `{"cyber_policy_enforce_all_groups":false,"cyber_policy_enforce_group_ids":[7]}`, groupID: &group8, wantAllow: false},
		{name: "ungrouped outside selected scope", risk: "true", config: `{"cyber_policy_enforce_all_groups":false,"cyber_policy_enforce_group_ids":[7]}`, groupID: nil, wantAllow: false},
		{name: "global risk switch off", risk: "false", groupID: &group7, wantAllow: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := map[string]string{SettingKeyRiskControlEnabled: tt.risk}
			if tt.config != "" {
				values[SettingKeyContentModerationConfig] = tt.config
			}
			svc := NewContentModerationService(&contentModerationTestSettingRepo{values: values}, &contentModerationTestRepo{}, nil, nil, nil, nil, nil, nil)
			require.Equal(t, tt.wantAllow, svc.ShouldEnforceCyberPolicyForGroup(context.Background(), tt.groupID))
		})
	}
}

func TestUpdateConfigPersistsCyberPolicyEnforcementScope(t *testing.T) {
	settingRepo := &contentModerationTestSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled: "true",
	}}
	svc := NewContentModerationService(settingRepo, &contentModerationTestRepo{}, nil, nil, nil, nil, nil, nil)
	defaultPolicy := defaultCyberPolicySettings()
	defaultPolicy.Mode = ContentModerationCyberModeCollect
	groupPolicies := []CyberPolicyGroupPolicy{{
		GroupID: 7,
		Policy: CyberPolicySettings{
			Mode: ContentModerationCyberModeEnforce, SessionBlockEnabled: false,
			SessionBlockTTLSeconds: 900, ViolationCountEnabled: true, EmailOnHit: false,
			AutoBanEnabled: true, BanThreshold: 3, ViolationWindowHours: 24,
		},
	}}

	view, err := svc.UpdateConfig(context.Background(), UpdateContentModerationConfigInput{
		CyberPolicyDefaultPolicy: &defaultPolicy,
		CyberPolicyGroupPolicies: &groupPolicies,
	})
	require.NoError(t, err)
	require.False(t, view.CyberPolicyEnforceAllGroups)
	require.Equal(t, []int64{7}, view.CyberPolicyEnforceGroupIDs)
	require.Len(t, view.CyberPolicyGroupPolicies, 1)
	require.Equal(t, 3, view.CyberPolicyGroupPolicies[0].Policy.BanThreshold)

	saved, err := parseContentModerationConfig(settingRepo.values[SettingKeyContentModerationConfig])
	require.NoError(t, err)
	require.False(t, saved.CyberPolicyEnforceAllGroups)
	require.Equal(t, []int64{7}, saved.CyberPolicyEnforceGroupIDs)
	group7, group8 := int64(7), int64(8)
	require.True(t, svc.ShouldEnforceCyberPolicyForGroup(context.Background(), &group7), "saved runtime snapshot must take effect immediately")
	require.False(t, svc.ShouldEnforceCyberPolicyForGroup(context.Background(), &group8))
}

func TestRecordCyberPolicyEvent_InitialRuntimeSnapshotLoadFailureSkipsEvent(t *testing.T) {
	repo := &banCountArgsTestRepo{}
	settingRepo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: `{invalid`,
	}}
	svc := NewContentModerationService(settingRepo, repo, nil, nil, nil, nil, nil, nil)

	svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{
		UserID: 1,
		Model:  "gpt-5",
	})

	require.Empty(t, repo.snapshotCountCalls())
	require.Empty(t, repo.snapshotLogs())
	getValue, getMultiple := settingRepo.calls()
	require.Zero(t, getValue)
	require.GreaterOrEqual(t, getMultiple, 1)
}

func TestRecordCyberPolicyEvent_RuntimeSnapshotRefreshFailureKeepsStaleScope(t *testing.T) {
	repo := &banCountArgsTestRepo{}
	settingRepo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: `{"all_groups":true,"model_filter":{"type":"include","models":["gpt-5"]}}`,
	}}
	svc := NewContentModerationService(settingRepo, repo, nil, nil, nil, nil, nil, nil)
	svc.runtimeCacheTTL = time.Minute

	_, err := svc.loadRuntimeSnapshot(context.Background())
	require.NoError(t, err)
	current := svc.runtimeSnapshot.Load()
	require.NotNil(t, current)
	expired := *current
	expired.loadedAt = time.Now().Add(-2 * time.Minute)
	svc.runtimeSnapshot.Store(&expired)
	settingRepo.failMultiple(errors.New("database unavailable"))

	svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{
		UserID: 1,
		Model:  "gpt-5",
	})

	require.Len(t, repo.snapshotLogs(), 1)
	require.Eventually(t, func() bool {
		_, calls := settingRepo.calls()
		return calls == 2
	}, time.Second, time.Millisecond)
	getValue, getMultiple := settingRepo.calls()
	require.Zero(t, getValue)
	require.Equal(t, 2, getMultiple)
}

// TestRecordCyberPolicyEvent_CreateLogBeforeEmail verifies F7: the moderation
// log is persisted BEFORE email delivery, and EmailSent is patched afterwards —
// SMTP hangs can no longer swallow the audit record.
//
// Note on email ordering: EmailService is a concrete type with no injectable
// send interface, so SMTP-success cannot be simulated in unit tests.
// With emailService=nil the email block is skipped and UpdateLogEmailSent is not
// called (correct: logPersisted && emailSent guard). The test therefore asserts
// the two invariants that ARE observable without real SMTP:
//  1. CreateLog runs first (calls[0]=="create").
//  2. The log is stored with EmailSent=false (not pre-set to true).
//
// The update_email_sent path is covered by integration/e2e tests where a real
// (or test-double) SMTP endpoint is available.
func TestRecordCyberPolicyEvent_CreateLogBeforeEmail(t *testing.T) {
	repo := &cyberOrderingTestRepo{}
	svc := NewContentModerationService(
		&contentModerationTestSettingRepo{values: map[string]string{
			SettingKeyRiskControlEnabled: "true",
		}},
		repo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil, // emailService=nil: email path safely skipped; see doc comment above
	)

	svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{
		RequestID:       "req-1",
		UserID:          7,
		UserEmail:       "u@example.com",
		Model:           "gpt-5",
		UpstreamMessage: "blocked",
	})

	calls := repo.snapshot()
	require.GreaterOrEqual(t, len(calls), 1, "CreateLog must be called")
	require.Equal(t, "create", calls[0], "CreateLog must run first (F7: log-before-email)")

	// EmailSent must be false when the log is first persisted (new code sets it
	// false before CreateLog; email result is patched via UpdateLogEmailSent).
	emailSents := repo.snapshotEmailSents()
	require.NotEmpty(t, emailSents, "CreateLog must have captured EmailSent value")
	require.False(t, emailSents[0], "log must be stored with EmailSent=false initially (F7)")

	// With emailService=nil, no email is sent, so UpdateLogEmailSent must NOT
	// be called (logPersisted && emailSent guard correctly suppresses the patch).
	require.NotContains(t, calls, "update_email_sent",
		"UpdateLogEmailSent must not be called when no email was sent")
}

// banCountArgsTestRepo 在 contentModerationTestRepo 基础上记录
// CountFlaggedByUserSince 收到的 excludeCyberPolicy 参数，供透传断言。
type banCountArgsTestRepo struct {
	contentModerationTestRepo
	argsMu          sync.Mutex
	countCalls      []bool
	cyberCountCalls []struct {
		userID  int64
		groupID *int64
	}
}

func (r *banCountArgsTestRepo) CountCyberPolicyByUserAndGroupSince(ctx context.Context, userID int64, groupID *int64, since time.Time) (int, error) {
	r.argsMu.Lock()
	r.cyberCountCalls = append(r.cyberCountCalls, struct {
		userID  int64
		groupID *int64
	}{userID: userID, groupID: cloneInt64Ptr(groupID)})
	r.argsMu.Unlock()
	return r.contentModerationTestRepo.CountCyberPolicyByUserAndGroupSince(ctx, userID, groupID, since)
}

func (r *banCountArgsTestRepo) CountFlaggedByUserSince(ctx context.Context, userID int64, since time.Time, excludeCyberPolicy bool) (int, error) {
	r.argsMu.Lock()
	r.countCalls = append(r.countCalls, excludeCyberPolicy)
	r.argsMu.Unlock()
	return r.contentModerationTestRepo.CountFlaggedByUserSince(ctx, userID, since, excludeCyberPolicy)
}

func (r *banCountArgsTestRepo) snapshotCountCalls() []bool {
	r.argsMu.Lock()
	defer r.argsMu.Unlock()
	out := make([]bool, len(r.countCalls))
	copy(out, r.countCalls)
	return out
}

func (r *banCountArgsTestRepo) snapshotCyberCountCalls() []struct {
	userID  int64
	groupID *int64
} {
	r.argsMu.Lock()
	defer r.argsMu.Unlock()
	out := make([]struct {
		userID  int64
		groupID *int64
	}, len(r.cyberCountCalls))
	copy(out, r.cyberCountCalls)
	return out
}

func TestApplyFlaggedAccountSideEffects_AlwaysExcludesCyberPolicy(t *testing.T) {
	repo := &banCountArgsTestRepo{}
	svc := NewContentModerationService(
		&contentModerationTestSettingRepo{values: map[string]string{}},
		repo, nil, nil, nil, nil, nil, nil,
	)
	userID := int64(42)

	cfgExclude := defaultContentModerationConfig()
	cfgExclude.CyberPolicyExcludeFromBanCount = true
	svc.applyFlaggedAccountSideEffects(context.Background(), cfgExclude, &ContentModerationLog{Flagged: true, UserID: &userID})

	cfgDefault := defaultContentModerationConfig() // 默认 false
	svc.applyFlaggedAccountSideEffects(context.Background(), cfgDefault, &ContentModerationLog{Flagged: true, UserID: &userID})

	require.Equal(t, []bool{true, true}, repo.snapshotCountCalls(),
		"普通内容审核必须始终排除由分组策略独立计数的 Cyber 行")
}

func TestRecordCyberPolicyEvent_ExcludeFromBanCount_SkipsBanJudgment(t *testing.T) {
	repo := &banCountArgsTestRepo{}
	svc := NewContentModerationService(
		&contentModerationTestSettingRepo{values: map[string]string{
			SettingKeyRiskControlEnabled:      "true",
			SettingKeyContentModerationConfig: `{"cyber_policy_exclude_from_ban_count":true}`,
		}},
		repo, nil, nil, nil, nil, nil, nil,
	)

	svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{
		UserID:          1,
		UserEmail:       "u@x.com",
		Model:           "gpt-5",
		Endpoint:        "/v1/responses",
		UpstreamMessage: "flagged",
		UpstreamStatus:  400,
	})

	require.Empty(t, repo.snapshotCountCalls(), "开关开时不得执行封号计数查询")
	require.Empty(t, repo.snapshotCyberCountCalls(), "开关开时不得执行 Cyber 分组计数查询")
	logs := repo.snapshotLogs()
	require.Len(t, logs, 1, "风控日志必须照记")
	require.True(t, logs[0].Flagged, "日志仍标记 Flagged=true（列表可见可筛）")
	require.Equal(t, "cyber_policy", logs[0].Action)
	require.Equal(t, ContentModerationCyberModeEnforce, logs[0].CyberPolicyMode)
	require.Equal(t, 0, logs[0].ViolationCount, "不参与计数时 ViolationCount 保持 0")
	require.False(t, logs[0].AutoBanned)
}

func TestRecordCyberPolicyEvent_DefaultCountsTowardBan(t *testing.T) {
	repo := &banCountArgsTestRepo{}
	svc := NewContentModerationService(
		&contentModerationTestSettingRepo{values: map[string]string{
			SettingKeyRiskControlEnabled: "true",
		}},
		repo, nil, nil, nil, nil, nil, nil,
	)

	svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{
		UserID:          1,
		UserEmail:       "u@x.com",
		Model:           "gpt-5",
		Endpoint:        "/v1/responses",
		UpstreamMessage: "flagged",
		UpstreamStatus:  400,
	})

	require.Empty(t, repo.snapshotCountCalls(), "Cyber 不再复用普通违规总计数")
	require.Len(t, repo.snapshotCyberCountCalls(), 1, "默认配置必须执行 Cyber 分组计数")
	logs := repo.snapshotLogs()
	require.Len(t, logs, 1)
	require.Equal(t, ContentModerationCyberModeEnforce, logs[0].CyberPolicyMode)
	require.GreaterOrEqual(t, logs[0].ViolationCount, 1, "默认路径行为不变（现状回归）")
}

func TestRecordCyberPolicyEvent_CounterIsolatedByGroup(t *testing.T) {
	group7, group8 := int64(7), int64(8)
	config := `{"cyber_policy_default_policy":{"mode":"enforce","session_block_enabled":false,"session_block_ttl_seconds":3600,"violation_count_enabled":true,"email_on_hit":false,"auto_ban_enabled":true,"ban_threshold":2,"violation_window_hours":24},"cyber_policy_group_policies":[]}`

	t.Run("another group does not reach threshold", func(t *testing.T) {
		userID := int64(1)
		repo := &contentModerationTestRepo{logs: []ContentModerationLog{{
			UserID: &userID, GroupID: &group8, Action: ContentModerationActionCyberPolicy,
			CyberPolicyMode: ContentModerationCyberModeEnforce, Flagged: true, CreatedAt: time.Now(),
		}}}
		userRepo := &contentModerationTestUserRepo{user: &User{ID: userID, Role: RoleUser, Status: StatusActive}}
		svc := NewContentModerationService(&contentModerationTestSettingRepo{values: map[string]string{
			SettingKeyRiskControlEnabled: "true", SettingKeyContentModerationConfig: config,
		}}, repo, nil, nil, userRepo, nil, nil, nil)

		svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{UserID: userID, GroupID: &group7})

		logs := repo.snapshotLogs()
		require.Len(t, logs, 2)
		require.Equal(t, 1, logs[1].ViolationCount)
		require.Equal(t, StatusActive, userRepo.user.Status)
	})

	t.Run("same group reaches threshold and disables account", func(t *testing.T) {
		userID := int64(1)
		repo := &contentModerationTestRepo{logs: []ContentModerationLog{{
			UserID: &userID, GroupID: &group7, Action: ContentModerationActionCyberPolicy,
			CyberPolicyMode: ContentModerationCyberModeEnforce, Flagged: true, CreatedAt: time.Now(),
		}}}
		userRepo := &contentModerationTestUserRepo{user: &User{ID: userID, Role: RoleUser, Status: StatusActive}}
		svc := NewContentModerationService(&contentModerationTestSettingRepo{values: map[string]string{
			SettingKeyRiskControlEnabled: "true", SettingKeyContentModerationConfig: config,
		}}, repo, nil, nil, userRepo, nil, nil, nil)

		svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{UserID: userID, GroupID: &group7})

		logs := repo.snapshotLogs()
		require.Len(t, logs, 2)
		require.Equal(t, 2, logs[1].ViolationCount)
		require.True(t, logs[1].AutoBanned)
		require.Equal(t, StatusDisabled, userRepo.user.Status)
		require.Len(t, userRepo.updated, 1)
	})
}

func TestCollectOnlyCyberHistoryDoesNotContributeToLaterBan(t *testing.T) {
	userID := int64(1)
	repo := &contentModerationTestRepo{logs: []ContentModerationLog{{
		UserID:            &userID,
		Action:            ContentModerationActionCyberPolicy,
		CyberPolicyMode:   ContentModerationCyberModeCollect,
		Flagged:           true,
		CreatedAt:         time.Now(),
		ViolationCount:    0,
		ThresholdSnapshot: map[string]float64{},
	}}}
	userRepo := &contentModerationTestUserRepo{user: &User{ID: userID, Role: RoleUser, Status: StatusActive}}
	svc := NewContentModerationService(&contentModerationTestSettingRepo{values: map[string]string{}}, repo, nil, nil, userRepo, nil, nil, nil)
	cfg := defaultContentModerationConfig()
	cfg.BanThreshold = 2
	current := &ContentModerationLog{UserID: &userID, Flagged: true, Action: ContentModerationActionBlock, CreatedAt: time.Now()}

	autoBanned := svc.applyFlaggedAccountSideEffects(context.Background(), cfg, current)

	require.False(t, autoBanned)
	require.Equal(t, 1, current.ViolationCount)
	require.Equal(t, StatusActive, userRepo.user.Status)
	require.Empty(t, userRepo.updated)
}
