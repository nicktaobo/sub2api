package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// SettingKeyModelPricingOverrides 是 settings 表里存放“通用模型价格覆盖表”的 key。
const SettingKeyModelPricingOverrides = "model_pricing_overrides"

// modelPricingOverride 是单条用户自定义模型价格覆盖（价格为每百万 token，币种 CNY/USD）。
type modelPricingOverride struct {
	Model      string  `json:"model"`
	Currency   string  `json:"currency"` // CurrencyCNY | CurrencyUSD
	InputPerM  float64 `json:"input"`
	OutputPerM float64 `json:"output"`
	CachePerM  float64 `json:"cache"`
	HasCache   bool    `json:"has_cache"`
	Enabled    bool    `json:"enabled"`
}

// ModelPricingOverridesDTO 是 admin GET/PUT 的载荷（整张表覆盖式读写）。
type ModelPricingOverridesDTO struct {
	Entries []modelPricingOverride `json:"entries"`
}

// currentPricingService 让包级函数 ModelPriceCurrency 也能查覆盖币种（进程内单例，
// 由 SetSettingRepository 登记）。
var currentPricingService atomic.Pointer[PricingService]

// SetSettingRepository 注入 settings 仓储，并把本实例登记为进程级定价服务。
func (s *PricingService) SetSettingRepository(repo SettingRepository) {
	if s == nil {
		return
	}
	s.settingRepo = repo
	currentPricingService.Store(s)
}

// loadOverrides 读取覆盖表，带独立 overrideMu + 60s TTL 进程缓存。
//
// ⚠️ 绝不能复用 s.mu：GetModelPricing 全程持有 s.mu.RLock 并在其中调用本方法，
// 若复用 s.mu 刷新缓存（写锁）会重入死锁。
func (s *PricingService) loadOverrides() []modelPricingOverride {
	if s == nil || s.settingRepo == nil {
		return nil
	}
	now := time.Now().UnixNano()

	s.overrideMu.RLock()
	if s.overrideLoadedAt != 0 && now < s.overrideLoadedAt {
		cached := s.overrideCache
		s.overrideMu.RUnlock()
		return cached
	}
	s.overrideMu.RUnlock()

	// 缓存过期：singleflight 合并并发读，只有一个 goroutine 真打 DB，其余共享结果（防惊群）。
	v, _, _ := s.overrideSF.Do("model_pricing_overrides", func() (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		raw, err := s.settingRepo.GetValue(ctx, SettingKeyModelPricingOverrides)
		if err != nil && !errors.Is(err, ErrSettingNotFound) {
			// 瞬时故障（超时/连接抖动）：保留上一份缓存继续用，短 TTL 后重试，绝不落空表，
			// 否则覆盖价会静默失效、计费回退到内置低价（方向是少收费）。
			s.overrideMu.Lock()
			prev := s.overrideCache
			s.overrideLoadedAt = now + int64(5*time.Second)
			s.overrideMu.Unlock()
			logger.LegacyPrintf("service.pricing", "[Pricing] load model pricing overrides failed, keep previous cache: %v", err)
			return prev, nil
		}
		var list []modelPricingOverride
		if err == nil && strings.TrimSpace(raw) != "" {
			_ = json.Unmarshal([]byte(raw), &list) // NotFound/空串/坏 JSON => 空表（用户可见地清空，非故障）
		}
		s.overrideMu.Lock()
		s.overrideCache = list
		s.overrideMatchList = sortedEnabledOverrides(list)
		s.overrideLoadedAt = now + int64(60*time.Second)
		s.overrideMu.Unlock()
		return list, nil
	})
	if list, ok := v.([]modelPricingOverride); ok {
		return list
	}
	return nil
}

// sortedEnabledOverrides 返回仅 enabled、按 Model 长度降序排序的副本（供 matchOverride 前缀回退，最长前缀优先）。
func sortedEnabledOverrides(list []modelPricingOverride) []modelPricingOverride {
	out := make([]modelPricingOverride, 0, len(list))
	for _, ov := range list {
		if ov.Enabled && strings.TrimSpace(ov.Model) != "" {
			out = append(out, ov)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return len(strings.TrimSpace(out[i].Model)) > len(strings.TrimSpace(out[j].Model))
	})
	return out
}

// InvalidateOverrideCache 置缓存失效，使下次读立即重载（admin 写后调用，即时生效）。
func (s *PricingService) InvalidateOverrideCache() {
	if s == nil {
		return
	}
	s.overrideMu.Lock()
	s.overrideLoadedAt = 0
	s.overrideMu.Unlock()
}

// matchOverride 精确匹配优先，无则按最长前缀回退。modelLower 须已小写。
//
// 同时尝试完整名与去 provider 前缀名（lastSegment），与内置 ¥ 表口径对齐——
// 上游传入的可能是带前缀的 "z-ai/glm-5.1"、"deepseek/deepseek-v4"，用户用短名/无前缀
// 覆盖（如 "glm"）也应能命中。overrideMatchList 已是 enabled + 按长度降序。
func (s *PricingService) matchOverride(modelLower string) (modelPricingOverride, bool) {
	if s == nil || s.settingRepo == nil {
		return modelPricingOverride{}, false
	}
	s.loadOverrides() // 确保缓存新鲜（TTL/singleflight/错误处理均在内部）
	s.overrideMu.RLock()
	list := s.overrideMatchList
	s.overrideMu.RUnlock()
	if len(list) == 0 {
		return modelPricingOverride{}, false
	}

	forms := []string{modelLower}
	if seg := lastSegment(modelLower); seg != modelLower {
		forms = append(forms, seg)
	}
	// 第一遍：精确匹配（任一形态）
	for _, ov := range list {
		om := strings.ToLower(strings.TrimSpace(ov.Model))
		for _, f := range forms {
			if om == f {
				return ov, true
			}
		}
	}
	// 第二遍：最长前缀回退（list 已按 Model 长度降序；任一形态）
	for _, ov := range list {
		om := strings.ToLower(strings.TrimSpace(ov.Model))
		for _, f := range forms {
			if strings.HasPrefix(f, om) {
				return ov, true
			}
		}
	}
	return modelPricingOverride{}, false
}

// overrideToLiteLLM 把覆盖项折算成内部“每 token 美元价”：CNY 按运行时汇率折算，
// USD 直接用（不除汇率）。与 qwenPricingOverride 口径一致。
func (s *PricingService) overrideToLiteLLM(ov modelPricingOverride) *LiteLLMModelPricing {
	const perToken = 1.0 / 1_000_000.0
	rate := 1.0
	if ov.Currency == CurrencyCNY {
		rate = s.cnyToUSDRate()
	}
	p := &LiteLLMModelPricing{LiteLLMProvider: "override", Mode: "chat"}
	p.InputCostPerToken = ov.InputPerM / rate * perToken
	p.OutputCostPerToken = ov.OutputPerM / rate * perToken
	if ov.HasCache {
		p.CacheReadInputTokenCost = ov.CachePerM / rate * perToken
		p.SupportsPromptCaching = true
	}
	return p
}

// GetModelPricingOverrides 返回当前覆盖表（供 admin API）。
func (s *PricingService) GetModelPricingOverrides(_ context.Context) (*ModelPricingOverridesDTO, error) {
	list := s.loadOverrides()
	if list == nil {
		list = []modelPricingOverride{}
	}
	return &ModelPricingOverridesDTO{Entries: list}, nil
}

// UpdateModelPricingOverrides 整表覆盖写入 settings 并即时生效（供 admin API）。
func (s *PricingService) UpdateModelPricingOverrides(ctx context.Context, in *ModelPricingOverridesDTO) (*ModelPricingOverridesDTO, error) {
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("setting repository not initialized")
	}
	if in == nil {
		in = &ModelPricingOverridesDTO{}
	}
	norm := make([]modelPricingOverride, 0, len(in.Entries))
	for _, e := range in.Entries {
		e.Model = strings.TrimSpace(e.Model)
		if e.Model == "" {
			continue
		}
		if e.Currency != CurrencyCNY && e.Currency != CurrencyUSD {
			e.Currency = CurrencyCNY
		}
		if e.InputPerM < 0 {
			e.InputPerM = 0
		}
		if e.OutputPerM < 0 {
			e.OutputPerM = 0
		}
		if e.CachePerM < 0 {
			e.CachePerM = 0
		}
		norm = append(norm, e)
	}
	b, err := json.Marshal(norm)
	if err != nil {
		return nil, err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyModelPricingOverrides, string(b)); err != nil {
		return nil, err
	}
	s.InvalidateOverrideCache()
	return &ModelPricingOverridesDTO{Entries: norm}, nil
}
