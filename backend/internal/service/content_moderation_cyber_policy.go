package service

import (
	"sort"
	"strings"
)

const cyberPolicySnapshotVersion = 1

// CyberPolicySettings is a complete post-hit policy. Collection is deliberately
// not configurable here: every upstream cyber_policy hit is retained while the
// risk-control master switch is enabled.
type CyberPolicySettings struct {
	Mode                   string `json:"mode"`
	SessionBlockEnabled    bool   `json:"session_block_enabled"`
	SessionBlockTTLSeconds int    `json:"session_block_ttl_seconds"`
	ViolationCountEnabled  bool   `json:"violation_count_enabled"`
	EmailOnHit             bool   `json:"email_on_hit"`
	AutoBanEnabled         bool   `json:"auto_ban_enabled"`
	BanThreshold           int    `json:"ban_threshold"`
	ViolationWindowHours   int    `json:"violation_window_hours"`
}

// CyberPolicyGroupPolicy replaces the default cyber policy for one group.
type CyberPolicyGroupPolicy struct {
	GroupID int64               `json:"group_id"`
	Policy  CyberPolicySettings `json:"policy"`
}

// ResolvedCyberPolicy is persisted with each hit so later configuration edits
// cannot erase why a historical action was taken.
type ResolvedCyberPolicy struct {
	Version int                 `json:"version"`
	Source  string              `json:"source"`
	Policy  CyberPolicySettings `json:"policy"`
}

func defaultCyberPolicySettings() CyberPolicySettings {
	return CyberPolicySettings{
		Mode:                   ContentModerationCyberModeEnforce,
		SessionBlockEnabled:    true,
		SessionBlockTTLSeconds: 3600,
		ViolationCountEnabled:  true,
		EmailOnHit:             true,
		AutoBanEnabled:         true,
		BanThreshold:           defaultContentModerationBanThreshold,
		ViolationWindowHours:   defaultContentModerationViolationWindowHours,
	}
}

func legacyCyberPolicySettings(cfg *ContentModerationConfig) CyberPolicySettings {
	policy := defaultCyberPolicySettings()
	if cfg == nil {
		return policy
	}
	policy.ViolationCountEnabled = !cfg.CyberPolicyExcludeFromBanCount
	policy.AutoBanEnabled = cfg.AutoBanEnabled && policy.ViolationCountEnabled
	policy.BanThreshold = cfg.BanThreshold
	policy.ViolationWindowHours = cfg.ViolationWindowHours
	policy.normalize()
	return policy
}

// applyLegacyCyberPolicyScope converts the v0.2.2 all-groups/list selector into
// the new default + overrides representation without changing runtime behavior.
func applyLegacyCyberPolicyScope(cfg *ContentModerationConfig) {
	if cfg == nil {
		return
	}
	enforced := legacyCyberPolicySettings(cfg)
	if cfg.CyberPolicyEnforceAllGroups {
		cfg.CyberPolicyDefaultPolicy = enforced
		cfg.CyberPolicyGroupPolicies = []CyberPolicyGroupPolicy{}
		return
	}
	collect := enforced
	collect.Mode = ContentModerationCyberModeCollect
	cfg.CyberPolicyDefaultPolicy = collect
	cfg.CyberPolicyGroupPolicies = make([]CyberPolicyGroupPolicy, 0, len(cfg.CyberPolicyEnforceGroupIDs))
	for _, groupID := range normalizeInt64IDs(cfg.CyberPolicyEnforceGroupIDs) {
		cfg.CyberPolicyGroupPolicies = append(cfg.CyberPolicyGroupPolicies, CyberPolicyGroupPolicy{
			GroupID: groupID,
			Policy:  enforced,
		})
	}
}

func (policy *CyberPolicySettings) normalize() {
	if policy == nil {
		return
	}
	switch strings.ToLower(strings.TrimSpace(policy.Mode)) {
	case ContentModerationCyberModeCollect:
		policy.Mode = ContentModerationCyberModeCollect
	default:
		policy.Mode = ContentModerationCyberModeEnforce
	}
	if policy.SessionBlockTTLSeconds <= 0 {
		policy.SessionBlockTTLSeconds = 3600
	}
	if policy.SessionBlockTTLSeconds > maxCyberPolicySessionBlockTTLSeconds {
		policy.SessionBlockTTLSeconds = maxCyberPolicySessionBlockTTLSeconds
	}
	if policy.BanThreshold <= 0 {
		policy.BanThreshold = defaultContentModerationBanThreshold
	}
	if policy.BanThreshold > 1000 {
		policy.BanThreshold = 1000
	}
	if policy.ViolationWindowHours <= 0 {
		policy.ViolationWindowHours = defaultContentModerationViolationWindowHours
	}
	if policy.ViolationWindowHours > 8760 {
		policy.ViolationWindowHours = 8760
	}
}

func cloneCyberPolicySettings(policy CyberPolicySettings) CyberPolicySettings {
	policy.normalize()
	return policy
}

func cloneCyberPolicyGroupPolicies(in []CyberPolicyGroupPolicy) []CyberPolicyGroupPolicy {
	if len(in) == 0 {
		return []CyberPolicyGroupPolicy{}
	}
	out := make([]CyberPolicyGroupPolicy, len(in))
	for i, item := range in {
		out[i] = CyberPolicyGroupPolicy{GroupID: item.GroupID, Policy: cloneCyberPolicySettings(item.Policy)}
	}
	return out
}

func normalizeCyberPolicyGroupPolicies(in []CyberPolicyGroupPolicy) []CyberPolicyGroupPolicy {
	byGroup := make(map[int64]CyberPolicySettings, len(in))
	for _, item := range in {
		if item.GroupID <= 0 {
			continue
		}
		item.Policy.normalize()
		byGroup[item.GroupID] = item.Policy
	}
	groupIDs := make([]int64, 0, len(byGroup))
	for groupID := range byGroup {
		groupIDs = append(groupIDs, groupID)
	}
	sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i] < groupIDs[j] })
	out := make([]CyberPolicyGroupPolicy, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		out = append(out, CyberPolicyGroupPolicy{GroupID: groupID, Policy: byGroup[groupID]})
	}
	return out
}

func syncLegacyCyberPolicyFields(cfg *ContentModerationConfig) {
	if cfg == nil {
		return
	}
	cfg.CyberPolicyEnforceAllGroups = cfg.CyberPolicyDefaultPolicy.Mode == ContentModerationCyberModeEnforce
	cfg.CyberPolicyEnforceGroupIDs = []int64{}
	if !cfg.CyberPolicyEnforceAllGroups {
		for _, item := range cfg.CyberPolicyGroupPolicies {
			if item.Policy.Mode == ContentModerationCyberModeEnforce {
				cfg.CyberPolicyEnforceGroupIDs = append(cfg.CyberPolicyEnforceGroupIDs, item.GroupID)
			}
		}
	}
	cfg.CyberPolicyExcludeFromBanCount = !cfg.CyberPolicyDefaultPolicy.ViolationCountEnabled
}

func (cfg *ContentModerationConfig) resolvedCyberPolicyForGroup(groupID *int64) ResolvedCyberPolicy {
	policy := defaultCyberPolicySettings()
	if cfg != nil {
		policy = cloneCyberPolicySettings(cfg.CyberPolicyDefaultPolicy)
	}
	source := ContentModerationCyberPolicySourceDefault
	if cfg != nil && groupID != nil {
		for _, item := range cfg.CyberPolicyGroupPolicies {
			if item.GroupID == *groupID {
				policy = cloneCyberPolicySettings(item.Policy)
				source = ContentModerationCyberPolicySourceGroupOverride
				break
			}
		}
	}
	// A collect-only policy is an observation policy. Its stored configuration is
	// preserved, while the resolved snapshot records that no action can execute.
	if policy.Mode == ContentModerationCyberModeCollect {
		policy.SessionBlockEnabled = false
		policy.ViolationCountEnabled = false
		policy.EmailOnHit = false
		policy.AutoBanEnabled = false
	}
	if !policy.ViolationCountEnabled {
		policy.AutoBanEnabled = false
	}
	return ResolvedCyberPolicy{Version: cyberPolicySnapshotVersion, Source: source, Policy: policy}
}

func (policy ResolvedCyberPolicy) Enforces() bool {
	return policy.Policy.Mode == ContentModerationCyberModeEnforce
}

func (policy ResolvedCyberPolicy) BlocksSession() bool {
	return policy.Enforces() && policy.Policy.SessionBlockEnabled
}
