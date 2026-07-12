//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// CalculateCostUnified
// ---------------------------------------------------------------------------

func TestCalculateCostUnified_NilResolver_FallsBackToOldPath(t *testing.T) {
	svc := newTestBillingService()

	tokens := UsageTokens{InputTokens: 1000, OutputTokens: 500}
	input := CostInput{
		Model:          "claude-sonnet-4",
		Tokens:         tokens,
		RateMultiplier: 1.0,
		Resolver:       nil, // no resolver
	}
	cost, err := svc.CalculateCostUnified(input)
	require.NoError(t, err)

	// Should match the old-path result exactly
	expected, err := svc.calculateCostInternal("claude-sonnet-4", tokens, 1.0, "", nil)
	require.NoError(t, err)
	require.InDelta(t, expected.TotalCost, cost.TotalCost, 1e-10)
	require.InDelta(t, expected.ActualCost, cost.ActualCost, 1e-10)
	// BillingMode is NOT set by old path through CalculateCostUnified (resolver == nil)
	require.Empty(t, cost.BillingMode)
}

func TestCalculateCostUnified_TokenMode(t *testing.T) {
	bs := newTestBillingService()
	resolver := NewModelPricingResolver(nil, bs)

	tokens := UsageTokens{InputTokens: 1000, OutputTokens: 500}
	input := CostInput{
		Ctx:            context.Background(),
		Model:          "claude-sonnet-4",
		Tokens:         tokens,
		RateMultiplier: 1.5,
		Resolver:       resolver,
	}
	cost, err := bs.CalculateCostUnified(input)
	require.NoError(t, err)
	require.NotNil(t, cost)

	// Verify token billing: Input: 1000*3e-6=0.003, Output: 500*15e-6=0.0075
	expectedTotal := 1000*3e-6 + 500*15e-6
	require.InDelta(t, expectedTotal, cost.TotalCost, 1e-10)
	require.InDelta(t, expectedTotal*1.5, cost.ActualCost, 1e-10)
	require.Equal(t, string(BillingModeToken), cost.BillingMode)
}

func TestCalculateCostUnified_TokenModeAppliesRateMultiplierToImageTokens(t *testing.T) {
	bs := newTestBillingService()
	resolver := NewModelPricingResolver(nil, bs)

	tokens := UsageTokens{InputTokens: 1000, OutputTokens: 600, ImageOutputTokens: 100}
	cost, err := bs.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "claude-sonnet-4",
		Tokens:         tokens,
		RateMultiplier: 3.0,
		Resolver:       resolver,
	})
	require.NoError(t, err)

	textInput := 1000 * 3e-6
	textOutput := 500 * 15e-6
	imageOutput := 100 * 15e-6
	require.InDelta(t, textInput+textOutput+imageOutput, cost.TotalCost, 1e-10)
	require.InDelta(t, (textInput+textOutput+imageOutput)*3.0, cost.ActualCost, 1e-10)
	require.InDelta(t, imageOutput, cost.ImageOutputCost, 1e-10)
}

func TestCalculateCostUnified_PerRequestMode(t *testing.T) {
	// Set up a ChannelService with a per-request pricing channel
	cs := newTestChannelServiceWithCache(t, &channelCache{
		pricingByGroupModel: map[channelModelKey]*ChannelModelPricing{
			{groupID: 1, model: "claude-sonnet-4"}: {
				BillingMode:     BillingModePerRequest,
				PerRequestPrice: testPtrFloat64(0.05),
			},
		},
		channelByGroupID: map[int64]*Channel{
			1: {ID: 1, Status: StatusActive},
		},
		groupPlatform:           map[int64]string{1: ""},
		wildcardByGroupPlatform: map[channelGroupPlatformKey][]*wildcardPricingEntry{},
		mappingByGroupModel:     map[channelModelKey]string{},
		wildcardMappingByGP:     map[channelGroupPlatformKey][]*wildcardMappingEntry{},
		byID:                    map[int64]*Channel{},
	})

	bs := newTestBillingService()
	resolver := NewModelPricingResolver(cs, bs)
	groupID := int64(1)

	input := CostInput{
		Ctx:            context.Background(),
		Model:          "claude-sonnet-4",
		GroupID:        &groupID,
		Tokens:         UsageTokens{InputTokens: 100, OutputTokens: 50},
		RequestCount:   3,
		RateMultiplier: 2.0,
		Resolver:       resolver,
	}
	cost, err := bs.CalculateCostUnified(input)
	require.NoError(t, err)
	require.NotNil(t, cost)

	// 3 requests * $0.05 = $0.15
	require.InDelta(t, 0.15, cost.TotalCost, 1e-10)
	// ActualCost = 0.15 * 2.0 = 0.30
	require.InDelta(t, 0.30, cost.ActualCost, 1e-10)
	require.Equal(t, string(BillingModePerRequest), cost.BillingMode)
}

func TestCalculateCostUnified_ImageMode(t *testing.T) {
	cs := newTestChannelServiceWithCache(t, &channelCache{
		pricingByGroupModel: map[channelModelKey]*ChannelModelPricing{
			{groupID: 2, model: "gemini-image"}: {
				BillingMode:     BillingModeImage,
				PerRequestPrice: testPtrFloat64(0.10),
			},
		},
		channelByGroupID: map[int64]*Channel{
			2: {ID: 2, Status: StatusActive},
		},
		groupPlatform:           map[int64]string{2: ""},
		wildcardByGroupPlatform: map[channelGroupPlatformKey][]*wildcardPricingEntry{},
		mappingByGroupModel:     map[channelModelKey]string{},
		wildcardMappingByGP:     map[channelGroupPlatformKey][]*wildcardMappingEntry{},
		byID:                    map[int64]*Channel{},
	})

	bs := &BillingService{
		cfg:            &config.Config{},
		fallbackPrices: map[string]*ModelPricing{},
	}
	resolver := NewModelPricingResolver(cs, bs)
	groupID := int64(2)

	input := CostInput{
		Ctx:            context.Background(),
		Model:          "gemini-image",
		GroupID:        &groupID,
		Tokens:         UsageTokens{},
		RequestCount:   2,
		RateMultiplier: 1.0,
		Resolver:       resolver,
	}
	cost, err := bs.CalculateCostUnified(input)
	require.NoError(t, err)
	require.NotNil(t, cost)

	// 2 * $0.10 = $0.20
	require.InDelta(t, 0.20, cost.TotalCost, 1e-10)
	require.InDelta(t, 0.20, cost.ActualCost, 1e-10)
	require.Equal(t, string(BillingModeImage), cost.BillingMode)
}

// TestCalculateCostUnified_RateMultiplierZeroProducesZero 锁定新行为：
// 保存时强制 > 0；若 0 仍泄漏到计费层，按 0 计费（而非历史上的 1.0）。
func TestCalculateCostUnified_RateMultiplierZeroProducesZero(t *testing.T) {
	bs := newTestBillingService()
	resolver := NewModelPricingResolver(nil, bs)

	tokens := UsageTokens{InputTokens: 1000, OutputTokens: 500}

	cost, err := bs.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "claude-sonnet-4",
		Tokens:         tokens,
		RateMultiplier: 0,
		Resolver:       resolver,
	})
	require.NoError(t, err)
	require.Greater(t, cost.TotalCost, 0.0)
	require.InDelta(t, 0.0, cost.ActualCost, 1e-10)
}

// TestCalculateCostUnified_NegativeRateMultiplierClampedToZero 锁定新行为：
// 负数倍率按 0 计费，避免历史的 <=0 → 1.0 把配置异常静默按标准价扣费。
func TestCalculateCostUnified_NegativeRateMultiplierClampedToZero(t *testing.T) {
	bs := newTestBillingService()
	resolver := NewModelPricingResolver(nil, bs)

	tokens := UsageTokens{InputTokens: 1000}

	cost, err := bs.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "claude-sonnet-4",
		Tokens:         tokens,
		RateMultiplier: -5.0,
		Resolver:       resolver,
	})
	require.NoError(t, err)
	require.Greater(t, cost.TotalCost, 0.0)
	require.InDelta(t, 0.0, cost.ActualCost, 1e-10)
}

func TestCalculateCostUnified_BillingModeFieldFilled(t *testing.T) {
	bs := newTestBillingService()
	resolver := NewModelPricingResolver(nil, bs)

	cost, err := bs.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "claude-sonnet-4",
		Tokens:         UsageTokens{InputTokens: 100},
		RateMultiplier: 1.0,
		Resolver:       resolver,
	})
	require.NoError(t, err)
	require.Equal(t, "token", cost.BillingMode)
}

func TestCalculateCostUnified_UsesPreResolvedPricing(t *testing.T) {
	bs := newTestBillingService()
	resolver := NewModelPricingResolver(nil, bs)

	// Pre-resolve with per_request mode to verify it's used instead of re-resolving
	preResolved := &ResolvedPricing{
		Mode:                   BillingModePerRequest,
		DefaultPerRequestPrice: 0.07,
	}

	cost, err := bs.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "claude-sonnet-4",
		Tokens:         UsageTokens{InputTokens: 100},
		RequestCount:   2,
		RateMultiplier: 1.0,
		Resolver:       resolver,
		Resolved:       preResolved,
	})
	require.NoError(t, err)
	require.NotNil(t, cost)

	// 2 * $0.07 = $0.14
	require.InDelta(t, 0.14, cost.TotalCost, 1e-10)
	require.Equal(t, string(BillingModePerRequest), cost.BillingMode)
}

// ---------------------------------------------------------------------------
// GPT-5.6 长上下文默认倍率 × 渠道约定价（fork 定制回归，见 calculateTokenCost）
// ---------------------------------------------------------------------------

// 渠道 flat 约定价的 GPT-5.6：总上下文超过 272K 默认阈值时，不得叠加默认
// 2x/1.5x 长上下文倍率（渠道价是与客户约定的固定单价）。
// 回归场景：合并上游后 applyModelSpecificPricingPolicy 扩到 GPT-5.6 +
// BasePricing 继承内置长上下文字段，导致渠道 flat 价超 272K 被静默按 2x/1.5x 多收。
func TestCalculateCostUnified_GPT56ChannelFlatPricingSkipsDefaultLongContext(t *testing.T) {
	cs := newTestChannelServiceWithTokenPricing(t, "gpt-5.6-sol", &ChannelModelPricing{
		BillingMode:    BillingModeToken,
		InputPrice:     testPtrFloat64(1e-6),
		OutputPrice:    testPtrFloat64(2e-6),
		CacheReadPrice: testPtrFloat64(1e-7),
	})
	bs := newTestBillingService()
	resolver := NewModelPricingResolver(cs, bs)
	groupID := int64(1)

	// 总上下文 = 100000 input + 200000 cache_read = 300000 > 272000
	tokens := UsageTokens{InputTokens: 100000, CacheReadTokens: 200000, OutputTokens: 1000}
	cost, err := bs.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "gpt-5.6-sol",
		GroupID:        &groupID,
		Tokens:         tokens,
		RateMultiplier: 1.0,
		Resolver:       resolver,
	})
	require.NoError(t, err)

	expectedInput := 100000 * 1e-6
	expectedCacheRead := 200000 * 1e-7
	expectedOutput := 1000 * 2e-6
	require.InDelta(t, expectedInput, cost.InputCost, 1e-10,
		"channel flat input price must not be multiplied by default long-context policy")
	require.InDelta(t, expectedCacheRead, cost.CacheReadCost, 1e-10,
		"channel flat cache-read price must not be multiplied by default long-context policy")
	require.InDelta(t, expectedOutput, cost.OutputCost, 1e-10,
		"channel flat output price must not be multiplied by default long-context policy")
	require.InDelta(t, expectedInput+expectedCacheRead+expectedOutput, cost.TotalCost, 1e-10)
}

// 渠道显式配置了长上下文分层（区间定价）的 GPT-5.6：区间价照常生效。
func TestCalculateCostUnified_GPT56ChannelIntervalPricingStillApplies(t *testing.T) {
	maxTokens := 272000
	cs := newTestChannelServiceWithTokenPricing(t, "gpt-5.6-sol", &ChannelModelPricing{
		BillingMode: BillingModeToken,
		Intervals: []PricingInterval{
			{MinTokens: 0, MaxTokens: &maxTokens, InputPrice: testPtrFloat64(1e-6), OutputPrice: testPtrFloat64(2e-6)},
			{MinTokens: 272000, InputPrice: testPtrFloat64(2e-6), OutputPrice: testPtrFloat64(3e-6)},
		},
	})
	bs := newTestBillingService()
	resolver := NewModelPricingResolver(cs, bs)
	groupID := int64(1)

	cost, err := bs.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "gpt-5.6-sol",
		GroupID:        &groupID,
		Tokens:         UsageTokens{InputTokens: 300000, OutputTokens: 1000},
		RateMultiplier: 1.0,
		Resolver:       resolver,
	})
	require.NoError(t, err)

	// 300000 > 272000 命中第二区间；区间价已含上下文分层，不再叠加长上下文倍率
	expected := 300000*2e-6 + 1000*3e-6
	require.InDelta(t, expected, cost.TotalCost, 1e-10)
}

// 内置/兜底定价的 GPT-5.6：默认长上下文倍率维持上游行为（对齐官方价）。
func TestCalculateCostUnified_GPT56BuiltinPricingKeepsDefaultLongContext(t *testing.T) {
	bs := newTestBillingService()
	resolver := NewModelPricingResolver(nil, bs)

	cost, err := bs.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "gpt-5.6-sol",
		Tokens:         UsageTokens{InputTokens: 300000, OutputTokens: 1000},
		RateMultiplier: 1.0,
		Resolver:       resolver,
	})
	require.NoError(t, err)

	// fallback gpt-5.6-sol: input 5e-6, output 30e-6；超 272K 后 input×2.0 / output×1.5
	expectedInput := 300000 * 5e-6 * 2.0
	expectedOutput := 1000 * 30e-6 * 1.5
	require.InDelta(t, expectedInput, cost.InputCost, 1e-10)
	require.InDelta(t, expectedOutput, cost.OutputCost, 1e-10)
}

// gpt-5.4 / gpt-5.5 渠道 flat 价：合并前长上下文倍率即对渠道价生效，
// 本次 fork 定制只针对 GPT-5.6，存量计费口径必须保持不变。
func TestCalculateCostUnified_GPT54And55ChannelFlatPricingKeepsLongContext(t *testing.T) {
	for _, model := range []string{"gpt-5.4", "gpt-5.5"} {
		t.Run(model, func(t *testing.T) {
			cs := newTestChannelServiceWithTokenPricing(t, model, &ChannelModelPricing{
				BillingMode: BillingModeToken,
				InputPrice:  testPtrFloat64(1e-6),
				OutputPrice: testPtrFloat64(2e-6),
			})
			bs := newTestBillingService()
			resolver := NewModelPricingResolver(cs, bs)
			groupID := int64(1)

			cost, err := bs.CalculateCostUnified(CostInput{
				Ctx:            context.Background(),
				Model:          model,
				GroupID:        &groupID,
				Tokens:         UsageTokens{InputTokens: 300000, OutputTokens: 1000},
				RateMultiplier: 1.0,
				Resolver:       resolver,
			})
			require.NoError(t, err)

			// 既有口径：超 272K 后渠道价也叠加 input×2.0 / output×1.5
			expectedInput := 300000 * 1e-6 * 2.0
			expectedOutput := 1000 * 2e-6 * 1.5
			require.InDelta(t, expectedInput, cost.InputCost, 1e-10)
			require.InDelta(t, expectedOutput, cost.OutputCost, 1e-10)
		})
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// newTestChannelServiceWithTokenPricing 构造带单条 token 模式渠道定价的 ChannelService
// （groupID 固定为 1）。
func newTestChannelServiceWithTokenPricing(t *testing.T, model string, pricing *ChannelModelPricing) *ChannelService {
	t.Helper()
	return newTestChannelServiceWithCache(t, &channelCache{
		pricingByGroupModel: map[channelModelKey]*ChannelModelPricing{
			{groupID: 1, model: model}: pricing,
		},
		channelByGroupID: map[int64]*Channel{
			1: {ID: 1, Status: StatusActive},
		},
		groupPlatform:           map[int64]string{1: ""},
		wildcardByGroupPlatform: map[channelGroupPlatformKey][]*wildcardPricingEntry{},
		mappingByGroupModel:     map[channelModelKey]string{},
		wildcardMappingByGP:     map[channelGroupPlatformKey][]*wildcardMappingEntry{},
		byID:                    map[int64]*Channel{},
	})
}

// newTestChannelServiceWithCache creates a ChannelService with a pre-populated
// cache snapshot, bypassing the repository layer entirely.
func newTestChannelServiceWithCache(t *testing.T, cache *channelCache) *ChannelService {
	t.Helper()
	cs := &ChannelService{}
	cache.loadedAt = time.Now()
	cs.cache.Store(cache)
	return cs
}
