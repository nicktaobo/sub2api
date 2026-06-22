package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// newOverridePricingService 构造一个带指定汇率的最小 PricingService（不触发远程同步/磁盘），
// 仅用于覆盖表相关单测。pricingData 默认空表，使内置 ¥ 表/JSON 回退按需补充。
func newOverridePricingService(rate float64) *PricingService {
	return &PricingService{
		cfg:         &config.Config{Pricing: config.PricingConfig{CNYToUSDRate: rate}},
		pricingData: map[string]*LiteLLMModelPricing{},
	}
}

// writableSettingRepo 是支持 Set 的最小测试替身（包内已有的 fakeSettingRepo 的 Set 会 panic）。
type writableSettingRepo struct {
	store    map[string]string
	forceErr error // 非 nil 时 GetValue 返回此错误（模拟 DB 瞬时故障）
}

func (w *writableSettingRepo) Get(_ context.Context, _ string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (w *writableSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if w.forceErr != nil {
		return "", w.forceErr
	}
	if v, ok := w.store[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}
func (w *writableSettingRepo) Set(_ context.Context, key, value string) error {
	if w.store == nil {
		w.store = map[string]string{}
	}
	w.store[key] = value
	return nil
}
func (w *writableSettingRepo) GetMultiple(_ context.Context, _ []string) (map[string]string, error) {
	return nil, nil
}
func (w *writableSettingRepo) SetMultiple(_ context.Context, _ map[string]string) error { return nil }
func (w *writableSettingRepo) GetAll(_ context.Context) (map[string]string, error)      { return nil, nil }
func (w *writableSettingRepo) Delete(_ context.Context, key string) error {
	delete(w.store, key)
	return nil
}

func TestModelPricingOverride_GetModelPricing(t *testing.T) {
	svc := newOverridePricingService(1.0) // rate = 1.0
	repo := &writableSettingRepo{store: map[string]string{
		SettingKeyModelPricingOverrides: `[
			{"model":"qwen-plus","currency":"CNY","input":999,"output":888,"cache":100,"has_cache":true,"enabled":true},
			{"model":"glm","currency":"USD","input":5,"output":15,"enabled":true},
			{"model":"glm-4.6","currency":"CNY","input":1,"output":2,"enabled":true},
			{"model":"kimi-k2.6","currency":"CNY","input":1,"output":1,"enabled":false}
		]`,
	}}
	svc.SetSettingRepository(repo)

	// 精确匹配 + CNY 折算（rate=1）+ cache
	p := svc.GetModelPricing("qwen-plus")
	require.NotNil(t, p)
	require.InDelta(t, 999.0/1e6, p.InputCostPerToken, 1e-12)
	require.InDelta(t, 888.0/1e6, p.OutputCostPerToken, 1e-12)
	require.InDelta(t, 100.0/1e6, p.CacheReadInputTokenCost, 1e-12)
	require.True(t, p.SupportsPromptCaching)
	require.Equal(t, "override", p.LiteLLMProvider)
	require.Equal(t, CurrencyCNY, ModelPriceCurrency("qwen-plus"))

	// 前缀回退：qwen-plus-2025-xx → qwen-plus
	require.InDelta(t, 999.0/1e6, svc.GetModelPricing("qwen-plus-2025-09-11").InputCostPerToken, 1e-12)

	// 精确优先于前缀：glm-4.6 命中 glm-4.6(CNY 1) 而非 glm(USD 5)
	require.InDelta(t, 1.0/1e6, svc.GetModelPricing("glm-4.6").InputCostPerToken, 1e-12)

	// 前缀：glm-5.2 → glm(USD 5)，USD 不除汇率
	require.InDelta(t, 5.0/1e6, svc.GetModelPricing("glm-5.2").InputCostPerToken, 1e-12)
	require.Equal(t, CurrencyUSD, ModelPriceCurrency("glm-5.2"))

	// enabled=false 不命中：kimi-k2.6 回退内置 ¥ 表(输入 6.5)
	require.InDelta(t, 6.5/1e6, svc.GetModelPricing("kimi-k2.6").InputCostPerToken, 1e-12)

	// 失效后即时重载
	repo.store[SettingKeyModelPricingOverrides] = `[{"model":"qwen-plus","currency":"CNY","input":1,"output":1,"enabled":true}]`
	svc.InvalidateOverrideCache()
	require.InDelta(t, 1.0/1e6, svc.GetModelPricing("qwen-plus").InputCostPerToken, 1e-12)
}

func TestModelPricingOverride_CurrencyConversion(t *testing.T) {
	repo := &writableSettingRepo{store: map[string]string{
		SettingKeyModelPricingOverrides: `[
			{"model":"x-cny","currency":"CNY","input":700,"output":700,"enabled":true},
			{"model":"x-usd","currency":"USD","input":100,"output":100,"enabled":true}
		]`,
	}}
	svc := newOverridePricingService(7.0) // rate = 7
	svc.SetSettingRepository(repo)

	require.InDelta(t, 700.0/7.0/1e6, svc.GetModelPricing("x-cny").InputCostPerToken, 1e-12) // CNY ÷ 7
	require.InDelta(t, 100.0/1e6, svc.GetModelPricing("x-usd").InputCostPerToken, 1e-12)     // USD 不除
}

func TestModelPricingOverride_ProviderPrefix(t *testing.T) {
	// 上游模型带 provider 前缀（z-ai/glm-5.1），用户用短名/无前缀覆盖也应命中（lastSegment 对齐内置 ¥ 表）。
	repo := &writableSettingRepo{store: map[string]string{
		SettingKeyModelPricingOverrides: `[
			{"model":"glm","currency":"CNY","input":3,"output":6,"enabled":true},
			{"model":"deepseek-v4","currency":"CNY","input":1,"output":2,"enabled":true}
		]`,
	}}
	svc := newOverridePricingService(1.0)
	svc.SetSettingRepository(repo)

	// 前缀回退 + lastSegment：z-ai/glm-5.1 → 去前缀 glm-5.1 → 前缀命中 "glm"
	require.InDelta(t, 3.0/1e6, svc.GetModelPricing("z-ai/glm-5.1").InputCostPerToken, 1e-12)
	// deepseek/deepseek-v4-pro → 去前缀 deepseek-v4-pro → 前缀命中 "deepseek-v4"
	require.InDelta(t, 1.0/1e6, svc.GetModelPricing("deepseek/deepseek-v4-pro").InputCostPerToken, 1e-12)
}

func TestModelPricingOverride_TransientErrorKeepsCache(t *testing.T) {
	repo := &writableSettingRepo{store: map[string]string{
		SettingKeyModelPricingOverrides: `[{"model":"foo","currency":"CNY","input":50,"output":50,"enabled":true}]`,
	}}
	svc := newOverridePricingService(1.0)
	svc.SetSettingRepository(repo)

	// 先正常加载一次
	require.InDelta(t, 50.0/1e6, svc.GetModelPricing("foo").InputCostPerToken, 1e-12)

	// 模拟 DB 瞬时故障 + 缓存失效：应保留上一份覆盖，而非回退（少收费）
	repo.forceErr = errors.New("db timeout")
	svc.InvalidateOverrideCache()
	require.InDelta(t, 50.0/1e6, svc.GetModelPricing("foo").InputCostPerToken, 1e-12)

	// 故障恢复 + 失效后能读到新值
	repo.forceErr = nil
	repo.store[SettingKeyModelPricingOverrides] = `[{"model":"foo","currency":"CNY","input":1,"output":1,"enabled":true}]`
	svc.InvalidateOverrideCache()
	require.InDelta(t, 1.0/1e6, svc.GetModelPricing("foo").InputCostPerToken, 1e-12)
}

func TestModelPricingOverride_NilRepoSafe(t *testing.T) {
	svc := newOverridePricingService(1.0) // 不注入 settingRepo
	require.NotPanics(t, func() { _ = svc.GetModelPricing("qwen-plus") })
	_, ok := svc.matchOverride("anything")
	require.False(t, ok)
}

func TestModelPricingOverride_UpdateRoundTrip(t *testing.T) {
	svc := newOverridePricingService(1.0)
	svc.SetSettingRepository(&writableSettingRepo{store: map[string]string{}})

	out, err := svc.UpdateModelPricingOverrides(context.Background(), &ModelPricingOverridesDTO{Entries: []modelPricingOverride{
		{Model: "  foo  ", Currency: "CNY", InputPerM: 1, OutputPerM: 2, Enabled: true},
		{Model: "", Currency: "USD", InputPerM: 9, Enabled: true},     // 空模型名被丢弃
		{Model: "bar", Currency: "xx", InputPerM: -5, Enabled: false}, // 非法币种→CNY，负价→0
	}})
	require.NoError(t, err)
	require.Len(t, out.Entries, 2)
	require.Equal(t, "foo", out.Entries[0].Model) // 已 trim
	require.Equal(t, CurrencyCNY, out.Entries[1].Currency)
	require.Zero(t, out.Entries[1].InputPerM)

	// 写后即时生效
	require.InDelta(t, 1.0/1e6, svc.GetModelPricing("foo").InputCostPerToken, 1e-12)
}
