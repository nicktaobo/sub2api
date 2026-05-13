package handler

import (
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AvailableChannelHandler 处理用户侧「可用渠道」查询。
//
// 用户侧接口委托 ChannelService.ListAvailable，并在返回前做三层过滤：
//  1. 行过滤：只保留状态为 Active 且与当前用户可访问分组有交集的渠道；
//  2. 分组过滤：渠道的 Groups 只保留用户可访问的那些；
//  3. 平台过滤：渠道的 SupportedModels 只保留平台在用户可见 Groups 中出现过的模型，
//     防止"渠道同时挂在 antigravity / anthropic 两个平台的分组上，用户只访问
//     antigravity，却看到 anthropic 模型"这类跨平台信息泄漏；
//  4. 字段白名单：仅返回用户需要的字段（省略 BillingModelSource / RestrictModels
//     / 内部 ID / Status 等管理字段）。
type AvailableChannelHandler struct {
	channelService  *service.ChannelService
	apiKeyService   *service.APIKeyService
	settingService  *service.SettingService
	billingService  *service.BillingService
	merchantPricing *service.MerchantPricingService
}

// NewAvailableChannelHandler 创建用户侧可用渠道 handler。
func NewAvailableChannelHandler(
	channelService *service.ChannelService,
	apiKeyService *service.APIKeyService,
	settingService *service.SettingService,
	billingService *service.BillingService,
	merchantPricing *service.MerchantPricingService,
) *AvailableChannelHandler {
	return &AvailableChannelHandler{
		channelService:  channelService,
		apiKeyService:   apiKeyService,
		settingService:  settingService,
		billingService:  billingService,
		merchantPricing: merchantPricing,
	}
}

// featureEnabled 返回 available-channels 开关是否启用。默认关闭（opt-in）。
func (h *AvailableChannelHandler) featureEnabled(c *gin.Context) bool {
	if h.settingService == nil {
		return false
	}
	return h.settingService.GetAvailableChannelsRuntime(c.Request.Context()).Enabled
}

// userAvailableGroup 用户可见的分组概要（白名单字段）。
//
// 前端据此区分专属 vs 公开分组（IsExclusive）、订阅 vs 标准分组（SubscriptionType，
// 订阅视觉加深），并用 RateMultiplier 作为默认倍率；用户专属倍率前端走
// /groups/rates，和 API 密钥页面保持一致。
type userAvailableGroup struct {
	ID               int64   `json:"id"`
	Name             string  `json:"name"`
	Platform         string  `json:"platform"`
	SubscriptionType string  `json:"subscription_type"`
	RateMultiplier   float64 `json:"rate_multiplier"`
	IsExclusive      bool    `json:"is_exclusive"`
}

// userSupportedModelPricing 用户可见的定价字段白名单。
//
// official_* 字段来自 LiteLLM 价格表（单位 USD / per token），用于前端做"本站价
// vs 官方价"对比展示。当模型在 LiteLLM 列表中未找到或价格为 0 时为 nil。
type userSupportedModelPricing struct {
	BillingMode             string                   `json:"billing_mode"`
	InputPrice              *float64                 `json:"input_price"`
	OutputPrice             *float64                 `json:"output_price"`
	CacheWritePrice         *float64                 `json:"cache_write_price"`
	CacheReadPrice          *float64                 `json:"cache_read_price"`
	ImageOutputPrice        *float64                 `json:"image_output_price"`
	PerRequestPrice         *float64                 `json:"per_request_price"`
	Intervals               []userPricingIntervalDTO `json:"intervals"`
	OfficialInputPrice      *float64                 `json:"official_input_price,omitempty"`
	OfficialOutputPrice     *float64                 `json:"official_output_price,omitempty"`
	OfficialCacheWritePrice *float64                 `json:"official_cache_write_price,omitempty"`
	OfficialCacheReadPrice  *float64                 `json:"official_cache_read_price,omitempty"`
}

// userPricingIntervalDTO 定价区间白名单（去掉内部 ID、SortOrder 等前端不渲染的字段）。
type userPricingIntervalDTO struct {
	MinTokens       int      `json:"min_tokens"`
	MaxTokens       *int     `json:"max_tokens"`
	TierLabel       string   `json:"tier_label,omitempty"`
	InputPrice      *float64 `json:"input_price"`
	OutputPrice     *float64 `json:"output_price"`
	CacheWritePrice *float64 `json:"cache_write_price"`
	CacheReadPrice  *float64 `json:"cache_read_price"`
	PerRequestPrice *float64 `json:"per_request_price"`
}

// userSupportedModel 用户可见的支持模型条目。
type userSupportedModel struct {
	Name     string                     `json:"name"`
	Platform string                     `json:"platform"`
	Pricing  *userSupportedModelPricing `json:"pricing"`
}

// userChannelPlatformSection 单渠道内某个平台的子视图：用户可见的分组 + 该平台
// 支持的模型。按 platform 聚合后让前端可以把渠道名作为 row-group 一次渲染，
// 后面的平台行按 sections 顺序铺开。
type userChannelPlatformSection struct {
	Platform        string               `json:"platform"`
	Groups          []userAvailableGroup `json:"groups"`
	SupportedModels []userSupportedModel `json:"supported_models"`
}

// userAvailableChannel 用户可见的渠道条目（白名单字段）。
//
// 每个渠道聚合为一条记录，内嵌 platforms 子数组：每个 section 对应一个平台，
// 包含该平台的 groups 和 supported_models。
type userAvailableChannel struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Platforms   []userChannelPlatformSection `json:"platforms"`
}

// buildVisibleChannels 共享构建逻辑：拉用户可见 groups → 过滤 channels 的 groups
// → 过滤 supported_models（防跨平台泄漏）→ enrich 官方价。
// 不做 feature flag 检查、不做 owner 视角的 sell_rate 替换（调用方按需做）。
func (h *AvailableChannelHandler) buildVisibleChannels(c *gin.Context, userID int64) ([]userAvailableChannel, error) {
	userGroups, err := h.apiKeyService.GetAvailableGroups(c.Request.Context(), userID)
	if err != nil {
		return nil, err
	}
	allowedGroupIDs := make(map[int64]struct{}, len(userGroups))
	for i := range userGroups {
		allowedGroupIDs[userGroups[i].ID] = struct{}{}
	}

	channels, err := h.channelService.ListAvailable(c.Request.Context())
	if err != nil {
		return nil, err
	}

	out := make([]userAvailableChannel, 0, len(channels))
	for _, ch := range channels {
		if ch.Status != service.StatusActive {
			continue
		}
		visibleGroups := filterUserVisibleGroups(ch.Groups, allowedGroupIDs)
		if len(visibleGroups) == 0 {
			continue
		}
		sections := buildPlatformSections(ch, visibleGroups)
		if len(sections) == 0 {
			continue
		}
		out = append(out, userAvailableChannel{
			Name:        ch.Name,
			Description: ch.Description,
			Platforms:   sections,
		})
	}

	h.enrichOfficialPricing(out)
	return out, nil
}

// List 列出当前用户可见的「可用渠道」。
// GET /api/v1/channels/available
func (h *AvailableChannelHandler) List(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	// Feature 未启用时返回空数组（不暴露渠道信息）。检查放在认证之后，
	// 保持与未开关前的 401 行为一致：未登录先 401，登录后再按开关决定。
	if !h.featureEnabled(c) {
		response.Success(c, []userAvailableChannel{})
		return
	}

	out, err := h.buildVisibleChannels(c, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.applyMerchantSellRate(c, subject.UserID, out)

	response.Success(c, out)
}

// userPricingModel 端点（group）下的单条模型；价格字段是 LiteLLM 官方价
// （per token, USD），前端按 group.rate_multiplier / fx_rate 算出本站价。
type userPricingModel struct {
	Name                    string   `json:"name"`
	OfficialInputPrice      *float64 `json:"official_input_price,omitempty"`
	OfficialOutputPrice     *float64 `json:"official_output_price,omitempty"`
	OfficialCacheWritePrice *float64 `json:"official_cache_write_price,omitempty"`
	OfficialCacheReadPrice  *float64 `json:"official_cache_read_price,omitempty"`
}

// userPricingGroup 「模型定价」展示页的端点 = 一个 group。
// 每个端点的"折扣"由 rate_multiplier 自己决定，跟具体上游 channel 无关。
type userPricingGroup struct {
	ID             int64              `json:"id"`
	Name           string             `json:"name"`
	Platform       string             `json:"platform"`
	RateMultiplier float64            `json:"rate_multiplier"`
	IsExclusive    bool               `json:"is_exclusive"`
	Models         []userPricingModel `json:"models"`
}

// PricingGroupList 列出用户可见的「定价端点」——每个端点对应一个 group，
// 模型集合 = 所有 active channel 中关联此 group 的 supported_models（按 platform
// 匹配）并集。完全不暴露 channel 概念给前端。
//
// 不受 available_channels_enabled 开关限制：模型价格的可见性跟"我能用哪些
// group"绑定，跟"是否展示可用渠道列表"独立。
//
// 商户 sub_user 视角下，rate_multiplier 替换为商户配置的 sell_rate（与计费
// 路径一致），让前端展示价跟实际扣款对齐。
//
// GET /api/v1/pricing/groups
func (h *AvailableChannelHandler) PricingGroupList(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	userGroups, err := h.apiKeyService.GetAvailableGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if len(userGroups) == 0 {
		response.Success(c, []userPricingGroup{})
		return
	}

	allowed := make(map[int64]*service.Group, len(userGroups))
	for i := range userGroups {
		g := userGroups[i]
		allowed[g.ID] = &g
	}

	channels, err := h.channelService.ListAvailable(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 扫所有 active channel，按 group_id 收集 (platform-matched) model 名集合（去重）。
	modelsByGroup := make(map[int64]map[string]struct{}, len(allowed))
	for _, ch := range channels {
		if ch.Status != service.StatusActive {
			continue
		}
		for _, gref := range ch.Groups {
			g, ok := allowed[gref.ID]
			if !ok {
				continue
			}
			set := modelsByGroup[g.ID]
			if set == nil {
				set = make(map[string]struct{}, 8)
				modelsByGroup[g.ID] = set
			}
			for _, m := range ch.SupportedModels {
				if m.Platform == g.Platform {
					set[m.Name] = struct{}{}
				}
			}
		}
	}

	// 官方价 lookup 缓存（同个模型可能多次出现）
	priceCache := make(map[string]*service.ModelPricing, 32)
	lookupOfficial := func(model string) *service.ModelPricing {
		if p, ok := priceCache[model]; ok {
			return p
		}
		if h.billingService == nil {
			priceCache[model] = nil
			return nil
		}
		p, err := h.billingService.GetModelPricing(model)
		if err != nil {
			priceCache[model] = nil
			return nil
		}
		priceCache[model] = p
		return p
	}

	out := make([]userPricingGroup, 0, len(userGroups))
	for i := range userGroups {
		g := userGroups[i]
		// 商户 sub_user 视角下的 rate 替换
		rate := g.RateMultiplier
		if h.merchantPricing != nil {
			if sell, ok := h.merchantPricing.LookupSellRateForUser(c.Request.Context(), subject.UserID, g.ID); ok {
				rate = sell
			}
		}

		item := userPricingGroup{
			ID:             g.ID,
			Name:           g.Name,
			Platform:       g.Platform,
			RateMultiplier: rate,
			IsExclusive:    g.IsExclusive,
			Models:         []userPricingModel{},
		}
		if set := modelsByGroup[g.ID]; len(set) > 0 {
			names := make([]string, 0, len(set))
			for n := range set {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, name := range names {
				m := userPricingModel{Name: name}
				if p := lookupOfficial(name); p != nil {
					m.OfficialInputPrice = positiveFloatPtr(p.InputPricePerToken)
					m.OfficialOutputPrice = positiveFloatPtr(p.OutputPricePerToken)
					m.OfficialCacheWritePrice = positiveFloatPtr(p.CacheCreationPricePerToken)
					m.OfficialCacheReadPrice = positiveFloatPtr(p.CacheReadPricePerToken)
				}
				item.Models = append(item.Models, m)
			}
		}
		out = append(out, item)
	}

	response.Success(c, out)
}

// applyMerchantSellRate 商户 sub_user 视角下，把每个 group 的 RateMultiplier
// 替换成商户配置的 sell_rate（商户没配的 group 维持主站价）。
// 这保证前端展示的"我看到的倍率"和实际计费时 sub_user 余额扣的钱一致。
func (h *AvailableChannelHandler) applyMerchantSellRate(c *gin.Context, userID int64, out []userAvailableChannel) {
	if h.merchantPricing == nil || userID <= 0 {
		return
	}
	ctx := c.Request.Context()
	cache := make(map[int64]float64, 8)
	miss := make(map[int64]struct{}, 8)
	lookup := func(groupID int64) (float64, bool) {
		if v, ok := cache[groupID]; ok {
			return v, true
		}
		if _, hit := miss[groupID]; hit {
			return 0, false
		}
		v, ok := h.merchantPricing.LookupSellRateForUser(ctx, userID, groupID)
		if ok {
			cache[groupID] = v
		} else {
			miss[groupID] = struct{}{}
		}
		return v, ok
	}
	for ci := range out {
		for si := range out[ci].Platforms {
			groups := out[ci].Platforms[si].Groups
			for gi := range groups {
				if v, ok := lookup(groups[gi].ID); ok {
					groups[gi].RateMultiplier = v
				}
			}
		}
	}
}

// enrichOfficialPricing 为每个模型补充官方价（来自 LiteLLM 价格表，USD/per token）。
// 查不到或价格为 0 时静默跳过，前端按 nil 展示为"-"。
func (h *AvailableChannelHandler) enrichOfficialPricing(out []userAvailableChannel) {
	if h.billingService == nil {
		return
	}
	cache := make(map[string]*service.ModelPricing, 32)
	lookup := func(model string) *service.ModelPricing {
		if p, ok := cache[model]; ok {
			return p
		}
		p, err := h.billingService.GetModelPricing(model)
		if err != nil {
			cache[model] = nil
			return nil
		}
		cache[model] = p
		return p
	}
	for ci := range out {
		for si := range out[ci].Platforms {
			models := out[ci].Platforms[si].SupportedModels
			for mi := range models {
				if models[mi].Pricing == nil {
					continue
				}
				p := lookup(models[mi].Name)
				if p == nil {
					continue
				}
				models[mi].Pricing.OfficialInputPrice = positiveFloatPtr(p.InputPricePerToken)
				models[mi].Pricing.OfficialOutputPrice = positiveFloatPtr(p.OutputPricePerToken)
				models[mi].Pricing.OfficialCacheWritePrice = positiveFloatPtr(p.CacheCreationPricePerToken)
				models[mi].Pricing.OfficialCacheReadPrice = positiveFloatPtr(p.CacheReadPricePerToken)
			}
		}
	}
}

func positiveFloatPtr(v float64) *float64 {
	if v <= 0 {
		return nil
	}
	return &v
}

// buildPlatformSections 把一个渠道按 visibleGroups 的平台集合拆成有序的 section 列表：
// 每个 section 对应一个平台，只包含该平台的 groups 和 supported_models。
// 输出按 platform 字母序稳定排序，便于前端等效比较与回归测试。
func buildPlatformSections(
	ch service.AvailableChannel,
	visibleGroups []userAvailableGroup,
) []userChannelPlatformSection {
	groupsByPlatform := make(map[string][]userAvailableGroup, 4)
	for _, g := range visibleGroups {
		if g.Platform == "" {
			continue
		}
		groupsByPlatform[g.Platform] = append(groupsByPlatform[g.Platform], g)
	}
	if len(groupsByPlatform) == 0 {
		return nil
	}

	platforms := make([]string, 0, len(groupsByPlatform))
	for p := range groupsByPlatform {
		platforms = append(platforms, p)
	}
	sort.Strings(platforms)

	sections := make([]userChannelPlatformSection, 0, len(platforms))
	for _, platform := range platforms {
		platformSet := map[string]struct{}{platform: {}}
		sections = append(sections, userChannelPlatformSection{
			Platform:        platform,
			Groups:          groupsByPlatform[platform],
			SupportedModels: toUserSupportedModels(ch.SupportedModels, platformSet),
		})
	}
	return sections
}

// filterUserVisibleGroups 仅保留用户可访问的分组。
func filterUserVisibleGroups(
	groups []service.AvailableGroupRef,
	allowed map[int64]struct{},
) []userAvailableGroup {
	visible := make([]userAvailableGroup, 0, len(groups))
	for _, g := range groups {
		if _, ok := allowed[g.ID]; !ok {
			continue
		}
		visible = append(visible, userAvailableGroup{
			ID:               g.ID,
			Name:             g.Name,
			Platform:         g.Platform,
			SubscriptionType: g.SubscriptionType,
			RateMultiplier:   g.RateMultiplier,
			IsExclusive:      g.IsExclusive,
		})
	}
	return visible
}

// toUserSupportedModels 将 service 层支持模型转换为用户 DTO（字段白名单）。
// 仅保留平台在 allowedPlatforms 中的条目，防止跨平台模型信息泄漏。
// allowedPlatforms 为 nil 时不做平台过滤（保留全部，供测试或明确无过滤场景使用）。
func toUserSupportedModels(
	src []service.SupportedModel,
	allowedPlatforms map[string]struct{},
) []userSupportedModel {
	out := make([]userSupportedModel, 0, len(src))
	for i := range src {
		m := src[i]
		if allowedPlatforms != nil {
			if _, ok := allowedPlatforms[m.Platform]; !ok {
				continue
			}
		}
		out = append(out, userSupportedModel{
			Name:     m.Name,
			Platform: m.Platform,
			Pricing:  toUserPricing(m.Pricing),
		})
	}
	return out
}

// toUserPricing 将 service 层定价转换为用户 DTO；入参为 nil 时返回 nil。
func toUserPricing(p *service.ChannelModelPricing) *userSupportedModelPricing {
	if p == nil {
		return nil
	}
	intervals := make([]userPricingIntervalDTO, 0, len(p.Intervals))
	for _, iv := range p.Intervals {
		intervals = append(intervals, userPricingIntervalDTO{
			MinTokens:       iv.MinTokens,
			MaxTokens:       iv.MaxTokens,
			TierLabel:       iv.TierLabel,
			InputPrice:      iv.InputPrice,
			OutputPrice:     iv.OutputPrice,
			CacheWritePrice: iv.CacheWritePrice,
			CacheReadPrice:  iv.CacheReadPrice,
			PerRequestPrice: iv.PerRequestPrice,
		})
	}
	billingMode := string(p.BillingMode)
	if billingMode == "" {
		billingMode = string(service.BillingModeToken)
	}
	return &userSupportedModelPricing{
		BillingMode:      billingMode,
		InputPrice:       p.InputPrice,
		OutputPrice:      p.OutputPrice,
		CacheWritePrice:  p.CacheWritePrice,
		CacheReadPrice:   p.CacheReadPrice,
		ImageOutputPrice: p.ImageOutputPrice,
		PerRequestPrice:  p.PerRequestPrice,
		Intervals:        intervals,
	}
}
