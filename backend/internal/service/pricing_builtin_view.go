package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
)

// BuiltinPricingEntry 是「内置全量价目表」的一条只读记录（每百万 token）。
// 它把系统真实计费用的两层内置价——② 国产¥硬编码表 与 ③ LiteLLM 全量 JSON——
// 摊平成与覆盖表同口径的形状,供「模型定价」页只读展示(让 Claude/GPT/Gemini/国产 默认价可见)。
type BuiltinPricingEntry struct {
	Model      string  `json:"model"`
	Currency   string  `json:"currency"` // CurrencyCNY | CurrencyUSD
	InputPerM  float64 `json:"input"`
	OutputPerM float64 `json:"output"`
	CachePerM  float64 `json:"cache"` // 缓存读取价
	HasCache   bool    `json:"has_cache"`
	Source     string  `json:"source"` // cny:<plat> | litellm | <litellm_provider>
}

// ModelPricingViewDTO 是「模型定价」页 List 接口的载荷:
// entries = 可编辑的用户覆盖表(settings),builtin = 只读内置全量表。
type ModelPricingViewDTO struct {
	Entries []modelPricingOverride `json:"entries"`
	Builtin []BuiltinPricingEntry  `json:"builtin"`
}

// GetModelPricingView 返回覆盖表 + 内置全量表(供 admin「模型定价」页)。
func (s *PricingService) GetModelPricingView(ctx context.Context) (*ModelPricingViewDTO, error) {
	ov, err := s.GetModelPricingOverrides(ctx)
	if err != nil {
		return nil, err
	}
	entries := ov.Entries
	if entries == nil {
		entries = []modelPricingOverride{}
	}
	return &ModelPricingViewDTO{Entries: entries, Builtin: s.ListBuiltinPricing()}, nil
}

// ListBuiltinPricing 摊平内置两层价目表为只读列表(每百万 token)。
// ② 国产¥表优先(真实计费先于 ③ JSON 命中它们),③ LiteLLM 同名不重复加入。
// 仅做 map 遍历 + 算术,RLock 内构造切片后立即解锁,不在锁内做慢操作(沿用计费热路径约束)。
func (s *PricingService) ListBuiltinPricing() []BuiltinPricingEntry {
	const perM = 1_000_000.0
	seen := make(map[string]struct{})
	out := make([]BuiltinPricingEntry, 0, 256)

	addCNY := func(table map[string]cnyModelPricing, source string) {
		for model, p := range table {
			key := strings.ToLower(strings.TrimSpace(model))
			if key == "" {
				continue
			}
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, BuiltinPricingEntry{
				Model:      model,
				Currency:   CurrencyCNY,
				InputPerM:  p.inputCNY,
				OutputPerM: p.outputCNY,
				CachePerM:  p.cacheReadCNY,
				HasCache:   p.hasCache,
				Source:     source,
			})
		}
	}
	addCNY(kimiMoonshotPricingTable, "cny:moonshot")
	addCNY(deepSeekPricingTable, "cny:deepseek")
	addCNY(qwenPricingTable, "cny:qwen")

	s.mu.RLock()
	for model, p := range s.pricingData {
		if p == nil {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(model))
		if key == "" {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		source := strings.TrimSpace(p.LiteLLMProvider)
		if source == "" {
			source = "litellm"
		}
		out = append(out, BuiltinPricingEntry{
			Model:      model,
			Currency:   CurrencyUSD,
			InputPerM:  p.InputCostPerToken * perM,
			OutputPerM: p.OutputCostPerToken * perM,
			CachePerM:  p.CacheReadInputTokenCost * perM,
			HasCache:   p.SupportsPromptCaching || p.CacheReadInputTokenCost > 0,
			Source:     source,
		})
	}
	s.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool { return out[i].Model < out[j].Model })
	return out
}

// pricingPreviewMaxChanges 限制 diff 预览返回的明细条数(总计数不受限,仅截断明细)。
const pricingPreviewMaxChanges = 800

// PricingChange 是一条价格变更(每百万 token,USD 口径——内置 ③ 层全 USD)。
// 含 input/output/cache-read 三项:缓存读取价(CacheReadInputTokenCost)也进真实计费
// (billing_service 取用),故 diff 必须覆盖它,否则只改缓存价时预览失真、与 apply 不一致。
type PricingChange struct {
	Model     string  `json:"model"`
	Kind      string  `json:"kind"` // added | removed | changed
	OldInput  float64 `json:"old_input"`
	NewInput  float64 `json:"new_input"`
	OldOutput float64 `json:"old_output"`
	NewOutput float64 `json:"new_output"`
	OldCache  float64 `json:"old_cache"`
	NewCache  float64 `json:"new_cache"`
}

// PricingRefreshPreview 是「一键拉取最新价」的 diff 预览(不落库)。
type PricingRefreshPreview struct {
	RemoteURL    string          `json:"remote_url"`
	CurrentCount int             `json:"current_count"`
	RemoteCount  int             `json:"remote_count"`
	Added        int             `json:"added"`
	Removed      int             `json:"removed"`
	Changed      int             `json:"changed"`
	Changes      []PricingChange `json:"changes"`
	Truncated    bool            `json:"truncated"`
}

// fetchRemoteParsed 拉取并解析远程价表,但**不落盘、不改内存**(仅供预览)。
// 复用 validatePricingURL / remoteClient / parsePricingData,不触碰 downloadPricingData 同步链路。
func (s *PricingService) fetchRemoteParsed(ctx context.Context) (map[string]*LiteLLMModelPricing, string, error) {
	remoteURL, err := s.validatePricingURL(s.cfg.Pricing.RemoteURL)
	if err != nil {
		return nil, "", err
	}
	body, err := s.remoteClient.FetchPricingJSON(ctx, remoteURL)
	if err != nil {
		return nil, remoteURL, fmt.Errorf("download failed: %w", err)
	}
	data, err := s.parsePricingData(body)
	if err != nil {
		return nil, remoteURL, fmt.Errorf("parse pricing data: %w", err)
	}
	return data, remoteURL, nil
}

// PreviewRemotePricing 拉远程价表与当前内置表做 diff,返回预览(不落库)。
// 应用走 ForceUpdate(同步链路),与 10min syncWithRemote 天然一致。
func (s *PricingService) PreviewRemotePricing(ctx context.Context) (*PricingRefreshPreview, error) {
	remote, remoteURL, err := s.fetchRemoteParsed(ctx)
	if err != nil {
		return nil, err
	}

	const perM = 1_000_000.0
	// name 保留原始大小写(展示用);diff 比对 input/output/cache-read 三项。
	type pm struct {
		name           string
		in, out, cache float64
	}
	snap := func(name string, p *LiteLLMModelPricing) pm {
		return pm{name: name, in: p.InputCostPerToken * perM, out: p.OutputCostPerToken * perM, cache: p.CacheReadInputTokenCost * perM}
	}

	s.mu.RLock()
	cur := make(map[string]pm, len(s.pricingData))
	for k, p := range s.pricingData {
		if p == nil {
			continue
		}
		cur[strings.ToLower(k)] = snap(k, p)
	}
	currentCount := len(s.pricingData)
	s.mu.RUnlock()

	rem := make(map[string]pm, len(remote))
	for k, p := range remote {
		if p == nil {
			continue
		}
		rem[strings.ToLower(k)] = snap(k, p)
	}

	const eps = 1e-9
	preview := &PricingRefreshPreview{RemoteURL: remoteURL, CurrentCount: currentCount, RemoteCount: len(remote)}
	changes := make([]PricingChange, 0, 64)
	push := func(c PricingChange) {
		if len(changes) < pricingPreviewMaxChanges {
			changes = append(changes, c)
		} else {
			preview.Truncated = true
		}
	}

	for k, rp := range rem {
		cp, ok := cur[k]
		if !ok {
			preview.Added++
			push(PricingChange{Model: rp.name, Kind: "added", NewInput: rp.in, NewOutput: rp.out, NewCache: rp.cache})
			continue
		}
		if math.Abs(cp.in-rp.in) > eps || math.Abs(cp.out-rp.out) > eps || math.Abs(cp.cache-rp.cache) > eps {
			preview.Changed++
			push(PricingChange{Model: rp.name, Kind: "changed",
				OldInput: cp.in, NewInput: rp.in,
				OldOutput: cp.out, NewOutput: rp.out,
				OldCache: cp.cache, NewCache: rp.cache})
		}
	}
	for k, cp := range cur {
		if _, ok := rem[k]; !ok {
			preview.Removed++
			push(PricingChange{Model: cp.name, Kind: "removed", OldInput: cp.in, OldOutput: cp.out, OldCache: cp.cache})
		}
	}

	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Kind != changes[j].Kind {
			return changes[i].Kind < changes[j].Kind
		}
		return changes[i].Model < changes[j].Model
	})
	preview.Changes = changes
	return preview, nil
}
