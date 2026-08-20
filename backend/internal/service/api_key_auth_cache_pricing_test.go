package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func apiKeyAuthCachePricingPtr(v float64) *float64 { return &v }

func TestAPIKeyAuthSnapshotGroupPricingRedisRoundTrip(t *testing.T) {
	groupID := int64(51)
	apiKey := &APIKey{
		ID:      1043,
		UserID:  1,
		GroupID: &groupID,
		Key:     "sk-pricing-roundtrip",
		Status:  StatusActive,
		User: &User{
			ID:          1,
			Status:      StatusActive,
			Role:        RoleUser,
			Balance:     10,
			Concurrency: 3,
		},
		Group: &Group{
			ID:                        groupID,
			Name:                      "grok-pricing",
			Platform:                  PlatformGrok,
			Status:                    StatusActive,
			SubscriptionType:          SubscriptionTypeStandard,
			RateMultiplier:            1,
			LongContextPricingEnabled: true,
			ModelPricing: []ChannelModelPricing{{
				Platform:       PlatformGrok,
				Models:         []string{"grok-4.6"},
				BillingMode:    BillingModeToken,
				InputPrice:     apiKeyAuthCachePricingPtr(2e-6),
				OutputPrice:    apiKeyAuthCachePricingPtr(6e-6),
				CacheReadPrice: apiKeyAuthCachePricingPtr(0.5e-6),
				Intervals: []PricingInterval{{
					MinTokens:      200000,
					InputPrice:     apiKeyAuthCachePricingPtr(4e-6),
					OutputPrice:    apiKeyAuthCachePricingPtr(12e-6),
					CacheReadPrice: apiKeyAuthCachePricingPtr(1e-6),
				}},
			}},
		},
	}

	svc := &APIKeyService{}
	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	require.NotNil(t, snapshot)
	require.Equal(t, 20, snapshot.Version)
	require.NotNil(t, snapshot.Group)
	require.True(t, snapshot.Group.LongContextPricingEnabled)
	require.Equal(t, apiKey.Group.ModelPricing, snapshot.Group.ModelPricing)
	require.NotSame(t, apiKey.Group.ModelPricing[0].InputPrice, snapshot.Group.ModelPricing[0].InputPrice)
	require.NotSame(t, apiKey.Group.ModelPricing[0].Intervals[0].InputPrice, snapshot.Group.ModelPricing[0].Intervals[0].InputPrice)

	apiKey.Group.ModelPricing[0].Models[0] = "mutated-source"
	*apiKey.Group.ModelPricing[0].InputPrice = 99
	*apiKey.Group.ModelPricing[0].Intervals[0].InputPrice = 101
	require.Equal(t, "grok-4.6", snapshot.Group.ModelPricing[0].Models[0])
	require.InDelta(t, 2e-6, *snapshot.Group.ModelPricing[0].InputPrice, 1e-12)
	require.InDelta(t, 4e-6, *snapshot.Group.ModelPricing[0].Intervals[0].InputPrice, 1e-12)

	// Simulate the Redis L2 JSON boundary before materializing a cache hit.
	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: snapshot})
	require.NoError(t, err)
	var cached APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &cached))

	materialized, used, err := svc.applyAuthCacheEntry(apiKey.Key, &cached)
	require.NoError(t, err)
	require.True(t, used)
	require.NotNil(t, materialized.Group)
	require.True(t, materialized.Group.LongContextPricingEnabled)
	require.Equal(t, snapshot.Group.ModelPricing, materialized.Group.ModelPricing)
	require.NotSame(t, cached.Snapshot.Group.ModelPricing[0].InputPrice, materialized.Group.ModelPricing[0].InputPrice)

	cached.Snapshot.Group.ModelPricing[0].Models[0] = "mutated-cache"
	*cached.Snapshot.Group.ModelPricing[0].InputPrice = 77
	require.Equal(t, "grok-4.6", materialized.Group.ModelPricing[0].Models[0])
	require.InDelta(t, 2e-6, *materialized.Group.ModelPricing[0].InputPrice, 1e-12)

	billing := &BillingService{fallbackPrices: map[string]*ModelPricing{
		"grok-4.6": {InputPricePerToken: 3e-6, OutputPricePerToken: 15e-6},
	}}
	resolver := NewModelPricingResolver(nil, billing)
	resolved := resolver.Resolve(context.Background(), PricingInput{Model: "grok-4.6", Group: materialized.Group})
	require.Equal(t, PricingSourceGroup, resolved.Source)
	require.True(t, resolved.longContextPricingEnabled)
	require.InDelta(t, 2e-6, resolved.BasePricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 6e-6, resolved.BasePricing.OutputPricePerToken, 1e-12)
}
