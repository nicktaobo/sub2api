package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

var (
	openAIModelDatePattern     = regexp.MustCompile(`-\d{8}$`)
	openAIModelBasePattern     = regexp.MustCompile(`^(gpt-\d+(?:\.\d+)?)(?:-|$)`)
	openAIGPT54FallbackPricing = &LiteLLMModelPricing{
		InputCostPerToken:               2.5e-06, // $2.5 per MTok
		OutputCostPerToken:              1.5e-05, // $15 per MTok
		CacheReadInputTokenCost:         2.5e-07, // $0.25 per MTok
		LongContextInputTokenThreshold:  272000,
		LongContextInputCostMultiplier:  2.0,
		LongContextOutputCostMultiplier: 1.5,
		LiteLLMProvider:                 "openai",
		Mode:                            "chat",
		SupportsPromptCaching:           true,
	}
	openAIGPT56SolFallbackPricing = &LiteLLMModelPricing{
		InputCostPerToken:                   5e-06,
		InputCostPerTokenPriority:           1e-05,
		OutputCostPerToken:                  3e-05,
		OutputCostPerTokenPriority:          6e-05,
		CacheCreationInputTokenCost:         6.25e-06,
		CacheCreationInputTokenCostPriority: 1.25e-05,
		CacheReadInputTokenCost:             5e-07,
		CacheReadInputTokenCostPriority:     1e-06,
		LongContextInputTokenThreshold:      openAIGPT54LongContextInputThreshold,
		LongContextInputCostMultiplier:      openAIGPT54LongContextInputMultiplier,
		LongContextOutputCostMultiplier:     openAIGPT54LongContextOutputMultiplier,
		SupportsServiceTier:                 true,
		LiteLLMProvider:                     "openai",
		Mode:                                "chat",
		SupportsPromptCaching:               true,
	}
	openAIGPT56TerraFallbackPricing = &LiteLLMModelPricing{
		InputCostPerToken:                   2.5e-06,
		InputCostPerTokenPriority:           5e-06,
		OutputCostPerToken:                  1.5e-05,
		OutputCostPerTokenPriority:          3e-05,
		CacheCreationInputTokenCost:         3.125e-06,
		CacheCreationInputTokenCostPriority: 6.25e-06,
		CacheReadInputTokenCost:             2.5e-07,
		CacheReadInputTokenCostPriority:     5e-07,
		LongContextInputTokenThreshold:      openAIGPT54LongContextInputThreshold,
		LongContextInputCostMultiplier:      openAIGPT54LongContextInputMultiplier,
		LongContextOutputCostMultiplier:     openAIGPT54LongContextOutputMultiplier,
		SupportsServiceTier:                 true,
		LiteLLMProvider:                     "openai",
		Mode:                                "chat",
		SupportsPromptCaching:               true,
	}
	openAIGPT56LunaFallbackPricing = &LiteLLMModelPricing{
		InputCostPerToken:                   1e-06,
		InputCostPerTokenPriority:           2e-06,
		OutputCostPerToken:                  6e-06,
		OutputCostPerTokenPriority:          1.2e-05,
		CacheCreationInputTokenCost:         1.25e-06,
		CacheCreationInputTokenCostPriority: 2.5e-06,
		CacheReadInputTokenCost:             1e-07,
		CacheReadInputTokenCostPriority:     2e-07,
		LongContextInputTokenThreshold:      openAIGPT54LongContextInputThreshold,
		LongContextInputCostMultiplier:      openAIGPT54LongContextInputMultiplier,
		LongContextOutputCostMultiplier:     openAIGPT54LongContextOutputMultiplier,
		SupportsServiceTier:                 true,
		LiteLLMProvider:                     "openai",
		Mode:                                "chat",
		SupportsPromptCaching:               true,
	}
	openAIGPT54MiniFallbackPricing = &LiteLLMModelPricing{
		InputCostPerToken:       7.5e-07,
		OutputCostPerToken:      4.5e-06,
		CacheReadInputTokenCost: 7.5e-08,
		LiteLLMProvider:         "openai",
		Mode:                    "chat",
		SupportsPromptCaching:   true,
	}
	openAIGPT54NanoFallbackPricing = &LiteLLMModelPricing{
		InputCostPerToken:       2e-07,
		OutputCostPerToken:      1.25e-06,
		CacheReadInputTokenCost: 2e-08,
		LiteLLMProvider:         "openai",
		Mode:                    "chat",
		SupportsPromptCaching:   true,
	}
	// Claude Fable 5 官方定价（$10/$50 per MTok，缓存写 1.25x / 1h 写 2x / 读 0.1x）。
	// 远程定价库尚未收录该模型时的静态兜底，避免同步覆盖本地条目后计费归零。
	claudeFable5FallbackPricing = &LiteLLMModelPricing{
		InputCostPerToken:                   1e-05,
		OutputCostPerToken:                  5e-05,
		CacheCreationInputTokenCost:         1.25e-05,
		CacheCreationInputTokenCostAbove1hr: 2e-05,
		CacheReadInputTokenCost:             1e-06,
		LiteLLMProvider:                     "anthropic",
		Mode:                                "chat",
		SupportsPromptCaching:               true,
	}
)

// Kimi / Moonshot 官方定价（人民币，每 100 万 token），来源：
// https://platform.kimi.com/docs/pricing/chat
// 官方仅公布人民币价，系统统一按美元计价，故在查找时用可配置汇率折算成美元。
// defaultCNYToUSDRate 仅在配置缺失/非法时兜底；正常由 pricing.cny_to_usd_rate 提供。
// 系统采用 1 CNY = 1 USD 余额的充值模型，因此默认汇率为 1.0（不换算）。
const defaultCNYToUSDRate = 1.0

// cnyModelPricing 定义一个模型的人民币官方定价（Kimi/Moonshot、DeepSeek 等）。
type cnyModelPricing struct {
	inputCNY     float64 // 输入（未命中缓存），¥/1M tokens
	cacheReadCNY float64 // 输入（命中缓存），¥/1M tokens；0 表示不支持缓存
	outputCNY    float64 // 输出，¥/1M tokens
	hasCache     bool    // 是否支持 prompt caching
}

// kimiMoonshotPricingTable 官方定价表（人民币/每百万 token）。
// kimi-for-coding 自动跟随最新模型（目前等价 k2.6）。
var kimiMoonshotPricingTable = map[string]cnyModelPricing{
	"kimi-k2.6":        {inputCNY: 6.5, cacheReadCNY: 1.1, outputCNY: 27.0, hasCache: true},
	"kimi-for-coding":  {inputCNY: 6.5, cacheReadCNY: 1.1, outputCNY: 27.0, hasCache: true},
	"kimi-k2.5":        {inputCNY: 4.0, cacheReadCNY: 0.7, outputCNY: 21.0, hasCache: true},
	"moonshot-v1-8k":   {inputCNY: 2.0, outputCNY: 10.0},
	"moonshot-v1-32k":  {inputCNY: 5.0, outputCNY: 20.0},
	"moonshot-v1-128k": {inputCNY: 10.0, outputCNY: 30.0},
}

// DeepSeek V4 官方定价（人民币，每 100 万 token），来源：
// https://api-docs.deepseek.com/quick_start/pricing/
// 用法同 Kimi/Moonshot：CNY 价格通过可配置汇率折算（默认 1:1）。
var deepSeekPricingTable = map[string]cnyModelPricing{
	"deepseek-v4-flash": {inputCNY: 1.0, cacheReadCNY: 0.02, outputCNY: 2.0, hasCache: true},
	"deepseek-v4-pro":   {inputCNY: 3.0, cacheReadCNY: 0.025, outputCNY: 6.0, hasCache: true},
}

// qwenPricingTable 阿里云百炼(DashScope)通义千问官方价（人民币/每百万 token，
// 国内区、基础档 0-32K、标准价，非 Batch/限时）。缓存命中按官方隐式缓存=输入价 20%。
// 来源 help.aliyun.com/zh/model-studio/model-pricing 及各模型价格页，2026-06 核对。
//   - qwen-plus / qwen-turbo / qwen3-max / qwen3-coder-plus：官方明确价
//   - qwen-max：当前分档价（旧版 ¥20/¥60 已降）；qwen-flash 输出、qwen-long、
//     qwen3-coder-flash 为合理估值，如与控制台不符可在此微调（站长可改）。
var qwenPricingTable = map[string]cnyModelPricing{
	"qwen3-max":         {inputCNY: 6.0, cacheReadCNY: 1.2, outputCNY: 24.0, hasCache: true},
	"qwen-max":          {inputCNY: 2.4, cacheReadCNY: 0.48, outputCNY: 9.6, hasCache: true},
	"qwen-plus":         {inputCNY: 0.8, cacheReadCNY: 0.16, outputCNY: 2.0, hasCache: true},
	"qwen-flash":        {inputCNY: 0.15, cacheReadCNY: 0.03, outputCNY: 1.5, hasCache: true},
	"qwen-turbo":        {inputCNY: 0.3, cacheReadCNY: 0.06, outputCNY: 0.6, hasCache: true},
	"qwen-long":         {inputCNY: 0.5, cacheReadCNY: 0.1, outputCNY: 2.0, hasCache: true},
	"qwen3-coder-plus":  {inputCNY: 4.0, cacheReadCNY: 0.8, outputCNY: 16.0, hasCache: true},
	"qwen3-coder-flash": {inputCNY: 1.5, cacheReadCNY: 0.3, outputCNY: 6.0, hasCache: true},
}

// LiteLLMModelPricing LiteLLM价格数据结构
// 只保留我们需要的字段，使用指针来处理可能缺失的值
type LiteLLMModelPricing struct {
	InputCostPerToken                   float64 `json:"input_cost_per_token"`
	InputCostPerTokenPriority           float64 `json:"input_cost_per_token_priority"`
	OutputCostPerToken                  float64 `json:"output_cost_per_token"`
	OutputCostPerTokenPriority          float64 `json:"output_cost_per_token_priority"`
	CacheCreationInputTokenCost         float64 `json:"cache_creation_input_token_cost"`
	CacheCreationInputTokenCostPriority float64 `json:"cache_creation_input_token_cost_priority"`
	CacheCreationInputTokenCostAbove1hr float64 `json:"cache_creation_input_token_cost_above_1hr"`
	CacheReadInputTokenCost             float64 `json:"cache_read_input_token_cost"`
	CacheReadInputTokenCostPriority     float64 `json:"cache_read_input_token_cost_priority"`
	LongContextInputTokenThreshold      int     `json:"long_context_input_token_threshold,omitempty"`
	LongContextInputCostMultiplier      float64 `json:"long_context_input_cost_multiplier,omitempty"`
	LongContextOutputCostMultiplier     float64 `json:"long_context_output_cost_multiplier,omitempty"`
	SupportsServiceTier                 bool    `json:"supports_service_tier"`
	LiteLLMProvider                     string  `json:"litellm_provider"`
	Mode                                string  `json:"mode"`
	SupportsPromptCaching               bool    `json:"supports_prompt_caching"`
	OutputCostPerImage                  float64 `json:"output_cost_per_image"`       // 图片生成模型每张图片价格
	OutputCostPerImageToken             float64 `json:"output_cost_per_image_token"` // 图片输出 token 价格
	InputCostPerImageToken              float64 `json:"input_cost_per_image_token"`  // 图片输入 token 价格（如 gpt-image-2 图片编辑）
	MaxOutputTokens                     int     `json:"max_output_tokens"`           // 模型 output token 物理上限(计费前封顶防上游伪造,2026-06 gegemini 事件)

	// TokenPricingAbsent 表示源数据中 input/output token 价格均缺失（仅有图片价）。
	// 此类条目只可用于图片计费，token 计费必须回退到 fallback 或 fail-closed，
	// 否则 token 流量会被按 $0 计费。零值（false）表示条目具备 token 价格。
	TokenPricingAbsent bool `json:"-"`
}

// PricingRemoteClient 远程价格数据获取接口
type PricingRemoteClient interface {
	FetchPricingJSON(ctx context.Context, url string) ([]byte, error)
	FetchHashText(ctx context.Context, url string) (string, error)
}

// LiteLLMRawEntry 用于解析原始JSON数据
type LiteLLMRawEntry struct {
	InputCostPerToken                   *float64 `json:"input_cost_per_token"`
	InputCostPerTokenPriority           *float64 `json:"input_cost_per_token_priority"`
	OutputCostPerToken                  *float64 `json:"output_cost_per_token"`
	OutputCostPerTokenPriority          *float64 `json:"output_cost_per_token_priority"`
	CacheCreationInputTokenCost         *float64 `json:"cache_creation_input_token_cost"`
	CacheCreationInputTokenCostPriority *float64 `json:"cache_creation_input_token_cost_priority"`
	CacheCreationInputTokenCostAbove1hr *float64 `json:"cache_creation_input_token_cost_above_1hr"`
	CacheReadInputTokenCost             *float64 `json:"cache_read_input_token_cost"`
	CacheReadInputTokenCostPriority     *float64 `json:"cache_read_input_token_cost_priority"`
	LongContextInputTokenThreshold      *int     `json:"long_context_input_token_threshold"`
	LongContextInputCostMultiplier      *float64 `json:"long_context_input_cost_multiplier"`
	LongContextOutputCostMultiplier     *float64 `json:"long_context_output_cost_multiplier"`
	SupportsServiceTier                 bool     `json:"supports_service_tier"`
	LiteLLMProvider                     string   `json:"litellm_provider"`
	Mode                                string   `json:"mode"`
	SupportsPromptCaching               bool     `json:"supports_prompt_caching"`
	OutputCostPerImage                  *float64 `json:"output_cost_per_image"`
	OutputCostPerImageToken             *float64 `json:"output_cost_per_image_token"`
	InputCostPerImageToken              *float64 `json:"input_cost_per_image_token"`
	MaxOutputTokens                     *int     `json:"max_output_tokens"`
}

// PricingService 动态价格服务
type PricingService struct {
	cfg          *config.Config
	remoteClient PricingRemoteClient
	mu           sync.RWMutex
	pricingData  map[string]*LiteLLMModelPricing
	lastUpdated  time.Time
	localHash    string

	// 用户自定义价格覆盖表（UI 配置，存 settings）。独立锁，绝不复用 s.mu，
	// 因为 GetModelPricing 全程持 s.mu.RLock 并在其中查覆盖，复用会重入死锁。
	settingRepo       SettingRepository
	overrideMu        sync.RWMutex
	overrideCache     []modelPricingOverride // 原始顺序（供 DTO 展示）
	overrideMatchList []modelPricingOverride // 仅 enabled、按 Model 长度降序（供前缀匹配，免每请求 sort）
	overrideLoadedAt  int64                  // unix nano；0=未加载/已失效
	overrideSF        singleflight.Group     // 缓存过期时合并并发 DB 读，防惊群

	// 停止信号
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewPricingService 创建价格服务
func NewPricingService(cfg *config.Config, remoteClient PricingRemoteClient) *PricingService {
	s := &PricingService{
		cfg:          cfg,
		remoteClient: remoteClient,
		pricingData:  make(map[string]*LiteLLMModelPricing),
		stopCh:       make(chan struct{}),
	}
	return s
}

// Initialize 初始化价格服务
func (s *PricingService) Initialize() error {
	// 确保数据目录存在
	if err := os.MkdirAll(s.cfg.Pricing.DataDir, 0755); err != nil {
		logger.LegacyPrintf("service.pricing", "[Pricing] Failed to create data directory: %v", err)
	}

	// 首次加载价格数据
	if err := s.checkAndUpdatePricing(); err != nil {
		logger.LegacyPrintf("service.pricing", "[Pricing] Initial load failed, using fallback: %v", err)
		if err := s.useFallbackPricing(); err != nil {
			return fmt.Errorf("failed to load pricing data: %w", err)
		}
	}

	// 启动定时更新
	s.startUpdateScheduler()

	logger.LegacyPrintf("service.pricing", "[Pricing] Service initialized with %d models", len(s.pricingData))
	return nil
}

// Stop 停止价格服务
func (s *PricingService) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	logger.LegacyPrintf("service.pricing", "%s", "[Pricing] Service stopped")
}

// startUpdateScheduler 启动定时更新调度器
func (s *PricingService) startUpdateScheduler() {
	// 定期检查哈希更新
	hashInterval := time.Duration(s.cfg.Pricing.HashCheckIntervalMinutes) * time.Minute
	if hashInterval < time.Minute {
		hashInterval = 10 * time.Minute
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(hashInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := s.syncWithRemote(); err != nil {
					logger.LegacyPrintf("service.pricing", "[Pricing] Sync failed: %v", err)
				}
			case <-s.stopCh:
				return
			}
		}
	}()

	logger.LegacyPrintf("service.pricing", "[Pricing] Update scheduler started (check every %v)", hashInterval)
}

// checkAndUpdatePricing 检查并更新价格数据
func (s *PricingService) checkAndUpdatePricing() error {
	pricingFile := s.getPricingFilePath()

	// 检查本地文件是否存在
	if _, err := os.Stat(pricingFile); os.IsNotExist(err) {
		logger.LegacyPrintf("service.pricing", "%s", "[Pricing] Local pricing file not found, downloading...")
		return s.downloadPricingData()
	}

	// 先加载本地文件（确保服务可用），再检查是否需要更新
	if err := s.loadPricingData(pricingFile); err != nil {
		logger.LegacyPrintf("service.pricing", "[Pricing] Failed to load local file, downloading: %v", err)
		return s.downloadPricingData()
	}

	// 如果配置了哈希URL，通过远程哈希检查是否有更新
	if s.cfg.Pricing.HashURL != "" {
		remoteHash, err := s.fetchRemoteHash()
		if err != nil {
			logger.LegacyPrintf("service.pricing", "[Pricing] Failed to fetch remote hash on startup: %v", err)
			return nil // 已加载本地文件，哈希获取失败不影响启动
		}

		s.mu.RLock()
		localHash := s.localHash
		s.mu.RUnlock()

		if localHash == "" || remoteHash != localHash {
			logger.LegacyPrintf("service.pricing", "[Pricing] Remote hash differs on startup (local=%s remote=%s), downloading...",
				localHash[:min(8, len(localHash))], remoteHash[:min(8, len(remoteHash))])
			if err := s.downloadPricingData(); err != nil {
				logger.LegacyPrintf("service.pricing", "[Pricing] Download failed, using existing file: %v", err)
			}
		}
		return nil
	}

	// 没有哈希URL时，基于文件年龄检查
	info, err := os.Stat(pricingFile)
	if err != nil {
		return nil // 已加载本地文件
	}

	fileAge := time.Since(info.ModTime())
	maxAge := time.Duration(s.cfg.Pricing.UpdateIntervalHours) * time.Hour

	if fileAge > maxAge {
		logger.LegacyPrintf("service.pricing", "[Pricing] Local file is %v old, updating...", fileAge.Round(time.Hour))
		if err := s.downloadPricingData(); err != nil {
			logger.LegacyPrintf("service.pricing", "[Pricing] Download failed, using existing file: %v", err)
		}
	}

	return nil
}

// syncWithRemote 与远程同步（基于哈希校验）
func (s *PricingService) syncWithRemote() error {
	// 如果配置了哈希URL，从远程获取哈希进行比对
	if s.cfg.Pricing.HashURL != "" {
		remoteHash, err := s.fetchRemoteHash()
		if err != nil {
			logger.LegacyPrintf("service.pricing", "[Pricing] Failed to fetch remote hash: %v", err)
			return nil // 哈希获取失败不影响正常使用
		}

		s.mu.RLock()
		localHash := s.localHash
		s.mu.RUnlock()

		if localHash == "" || remoteHash != localHash {
			logger.LegacyPrintf("service.pricing", "[Pricing] Remote hash differs (local=%s remote=%s), downloading new version...",
				localHash[:min(8, len(localHash))], remoteHash[:min(8, len(remoteHash))])
			return s.downloadPricingData()
		}
		logger.LegacyPrintf("service.pricing", "%s", "[Pricing] Hash check passed, no update needed")
		return nil
	}

	// 没有哈希URL时，基于时间检查
	pricingFile := s.getPricingFilePath()
	info, err := os.Stat(pricingFile)
	if err != nil {
		return s.downloadPricingData()
	}

	fileAge := time.Since(info.ModTime())
	maxAge := time.Duration(s.cfg.Pricing.UpdateIntervalHours) * time.Hour

	if fileAge > maxAge {
		logger.LegacyPrintf("service.pricing", "[Pricing] File is %v old, downloading...", fileAge.Round(time.Hour))
		return s.downloadPricingData()
	}

	return nil
}

// downloadPricingData 从远程下载价格数据
func (s *PricingService) downloadPricingData() error {
	remoteURL, err := s.validatePricingURL(s.cfg.Pricing.RemoteURL)
	if err != nil {
		return err
	}
	logger.LegacyPrintf("service.pricing", "[Pricing] Downloading from %s", remoteURL)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 获取远程哈希（用于同步锚点，不作为完整性校验）
	var remoteHash string
	if strings.TrimSpace(s.cfg.Pricing.HashURL) != "" {
		remoteHash, err = s.fetchRemoteHash()
		if err != nil {
			logger.LegacyPrintf("service.pricing", "[Pricing] Failed to fetch remote hash (continuing): %v", err)
		}
	}

	body, err := s.remoteClient.FetchPricingJSON(ctx, remoteURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// 哈希校验：不匹配时仅告警，不阻止更新
	// 远程哈希文件可能与数据文件不同步（如维护者更新了数据但未更新哈希文件）
	dataHash := sha256.Sum256(body)
	dataHashStr := hex.EncodeToString(dataHash[:])
	if remoteHash != "" && !strings.EqualFold(remoteHash, dataHashStr) {
		logger.LegacyPrintf("service.pricing", "[Pricing] Hash mismatch warning: remote=%s data=%s (hash file may be out of sync)",
			remoteHash[:min(8, len(remoteHash))], dataHashStr[:8])
	}

	// 解析JSON数据（使用灵活的解析方式）
	data, err := s.parsePricingData(body)
	if err != nil {
		return fmt.Errorf("parse pricing data: %w", err)
	}
	data = s.mergeFallbackPricingData(data)

	// 保存到本地文件
	pricingFile := s.getPricingFilePath()
	if err := os.WriteFile(pricingFile, body, 0644); err != nil {
		logger.LegacyPrintf("service.pricing", "[Pricing] Failed to save file: %v", err)
	}

	// 使用远程哈希作为同步锚点，防止重复下载
	// 当远程哈希不可用时，回退到数据本身的哈希
	syncHash := dataHashStr
	if remoteHash != "" {
		syncHash = remoteHash
	}
	hashFile := s.getHashFilePath()
	if err := os.WriteFile(hashFile, []byte(syncHash+"\n"), 0644); err != nil {
		logger.LegacyPrintf("service.pricing", "[Pricing] Failed to save hash: %v", err)
	}

	// 更新内存数据
	s.mu.Lock()
	s.pricingData = data
	s.lastUpdated = time.Now()
	s.localHash = syncHash
	s.mu.Unlock()

	logger.LegacyPrintf("service.pricing", "[Pricing] Downloaded %d models successfully", len(data))
	return nil
}

// parsePricingData 解析价格数据（处理各种格式）
func (s *PricingService) parsePricingData(body []byte) (map[string]*LiteLLMModelPricing, error) {
	// 首先解析为 map[string]json.RawMessage
	var rawData map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("parse raw JSON: %w", err)
	}

	result := make(map[string]*LiteLLMModelPricing)
	skipped := 0

	for modelName, rawEntry := range rawData {
		// 跳过 sample_spec 等文档条目
		if modelName == "sample_spec" {
			continue
		}

		// 尝试解析每个条目
		var entry LiteLLMRawEntry
		if err := json.Unmarshal(rawEntry, &entry); err != nil {
			skipped++
			continue
		}

		// 只保留有有效价格的条目
		if entry.InputCostPerToken == nil && entry.OutputCostPerToken == nil && entry.OutputCostPerImage == nil && entry.OutputCostPerImageToken == nil && entry.InputCostPerImageToken == nil {
			continue
		}

		pricing := &LiteLLMModelPricing{
			LiteLLMProvider:       entry.LiteLLMProvider,
			Mode:                  entry.Mode,
			SupportsPromptCaching: entry.SupportsPromptCaching,
			SupportsServiceTier:   entry.SupportsServiceTier,
			TokenPricingAbsent:    entry.InputCostPerToken == nil && entry.OutputCostPerToken == nil,
		}

		if entry.MaxOutputTokens != nil {
			pricing.MaxOutputTokens = *entry.MaxOutputTokens
		}

		if entry.InputCostPerToken != nil {
			pricing.InputCostPerToken = *entry.InputCostPerToken
		}
		if entry.InputCostPerTokenPriority != nil {
			pricing.InputCostPerTokenPriority = *entry.InputCostPerTokenPriority
		}
		if entry.OutputCostPerToken != nil {
			pricing.OutputCostPerToken = *entry.OutputCostPerToken
		}
		if entry.OutputCostPerTokenPriority != nil {
			pricing.OutputCostPerTokenPriority = *entry.OutputCostPerTokenPriority
		}
		if entry.CacheCreationInputTokenCost != nil {
			pricing.CacheCreationInputTokenCost = *entry.CacheCreationInputTokenCost
		}
		if entry.CacheCreationInputTokenCostPriority != nil {
			pricing.CacheCreationInputTokenCostPriority = *entry.CacheCreationInputTokenCostPriority
		}
		if entry.CacheCreationInputTokenCostAbove1hr != nil {
			pricing.CacheCreationInputTokenCostAbove1hr = *entry.CacheCreationInputTokenCostAbove1hr
		}
		if entry.CacheReadInputTokenCost != nil {
			pricing.CacheReadInputTokenCost = *entry.CacheReadInputTokenCost
		}
		if entry.CacheReadInputTokenCostPriority != nil {
			pricing.CacheReadInputTokenCostPriority = *entry.CacheReadInputTokenCostPriority
		}
		if entry.LongContextInputTokenThreshold != nil {
			pricing.LongContextInputTokenThreshold = *entry.LongContextInputTokenThreshold
		}
		if entry.LongContextInputCostMultiplier != nil {
			pricing.LongContextInputCostMultiplier = *entry.LongContextInputCostMultiplier
		}
		if entry.LongContextOutputCostMultiplier != nil {
			pricing.LongContextOutputCostMultiplier = *entry.LongContextOutputCostMultiplier
		}
		if entry.OutputCostPerImage != nil {
			pricing.OutputCostPerImage = *entry.OutputCostPerImage
		}
		if entry.OutputCostPerImageToken != nil {
			pricing.OutputCostPerImageToken = *entry.OutputCostPerImageToken
		}
		if entry.InputCostPerImageToken != nil {
			pricing.InputCostPerImageToken = *entry.InputCostPerImageToken
		}

		result[modelName] = pricing
	}

	if skipped > 0 {
		logger.LegacyPrintf("service.pricing", "[Pricing] Skipped %d invalid entries", skipped)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no valid pricing entries found")
	}

	return result, nil
}

// loadPricingData 从本地文件加载价格数据
func (s *PricingService) loadPricingData(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file failed: %w", err)
	}

	// 使用灵活的解析方式
	pricingData, err := s.parsePricingData(data)
	if err != nil {
		return fmt.Errorf("parse pricing data: %w", err)
	}
	pricingData = s.mergeFallbackPricingData(pricingData)

	// 计算哈希
	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])

	s.mu.Lock()
	s.pricingData = pricingData
	s.localHash = hashStr

	info, _ := os.Stat(filePath)
	if info != nil {
		s.lastUpdated = info.ModTime()
	} else {
		s.lastUpdated = time.Now()
	}
	s.mu.Unlock()

	logger.LegacyPrintf("service.pricing", "[Pricing] Loaded %d models from %s", len(pricingData), filePath)
	return nil
}

func (s *PricingService) mergeFallbackPricingData(data map[string]*LiteLLMModelPricing) map[string]*LiteLLMModelPricing {
	if data == nil {
		data = make(map[string]*LiteLLMModelPricing)
	}
	if s == nil || s.cfg == nil || strings.TrimSpace(s.cfg.Pricing.FallbackFile) == "" {
		return data
	}
	fallbackBody, err := os.ReadFile(s.cfg.Pricing.FallbackFile)
	if err != nil {
		logger.LegacyPrintf("service.pricing", "[Pricing] Fallback merge skipped: %v", err)
		return data
	}
	fallbackData, err := s.parsePricingData(fallbackBody)
	if err != nil {
		logger.LegacyPrintf("service.pricing", "[Pricing] Fallback merge parse skipped: %v", err)
		return data
	}
	merged := 0
	for modelName, pricing := range fallbackData {
		if _, ok := data[modelName]; ok {
			continue
		}
		data[modelName] = pricing
		merged++
	}
	if merged > 0 {
		logger.LegacyPrintf("service.pricing", "[Pricing] Merged %d fallback-only models", merged)
	}
	return data
}

// useFallbackPricing 使用回退价格文件
func (s *PricingService) useFallbackPricing() error {
	fallbackFile := s.cfg.Pricing.FallbackFile

	if _, err := os.Stat(fallbackFile); os.IsNotExist(err) {
		return fmt.Errorf("fallback file not found: %s", fallbackFile)
	}

	logger.LegacyPrintf("service.pricing", "[Pricing] Using fallback file: %s", fallbackFile)

	// 复制到数据目录
	data, err := os.ReadFile(fallbackFile)
	if err != nil {
		return fmt.Errorf("read fallback failed: %w", err)
	}

	pricingFile := s.getPricingFilePath()
	if err := os.WriteFile(pricingFile, data, 0644); err != nil {
		logger.LegacyPrintf("service.pricing", "[Pricing] Failed to copy fallback: %v", err)
	}

	return s.loadPricingData(fallbackFile)
}

// fetchRemoteHash 从远程获取哈希值
func (s *PricingService) fetchRemoteHash() (string, error) {
	hashURL, err := s.validatePricingURL(s.cfg.Pricing.HashURL)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	hash, err := s.remoteClient.FetchHashText(ctx, hashURL)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(hash), nil
}

func (s *PricingService) validatePricingURL(raw string) (string, error) {
	if s.cfg != nil && !s.cfg.Security.URLAllowlist.Enabled {
		normalized, err := urlvalidator.ValidateURLFormat(raw, s.cfg.Security.URLAllowlist.AllowInsecureHTTP)
		if err != nil {
			return "", fmt.Errorf("invalid pricing url: %w", err)
		}
		return normalized, nil
	}
	normalized, err := urlvalidator.ValidateHTTPSURL(raw, urlvalidator.ValidationOptions{
		AllowedHosts:     s.cfg.Security.URLAllowlist.PricingHosts,
		RequireAllowlist: true,
		AllowPrivate:     s.cfg.Security.URLAllowlist.AllowPrivateHosts,
	})
	if err != nil {
		return "", fmt.Errorf("invalid pricing url: %w", err)
	}
	return normalized, nil
}

// cnyToUSDRate 返回当前生效的人民币→美元汇率；配置缺失或非法时回退到兜底值。
func (s *PricingService) cnyToUSDRate() float64 {
	if s.cfg != nil && s.cfg.Pricing.CNYToUSDRate > 0 {
		return s.cfg.Pricing.CNYToUSDRate
	}
	return defaultCNYToUSDRate
}

// Currency codes for usage-log price display.
const (
	CurrencyUSD = "USD"
	CurrencyCNY = "CNY"
)

// ModelPriceCurrency reports the currency a model's usage cost is denominated in.
// Returns CurrencyCNY when the model is priced via the official-RMB pricing tables
// (the Kimi/Moonshot/DeepSeek overrides GetModelPricing applies before any USD source),
// otherwise CurrencyUSD. It reuses the exact membership those overrides use, so the
// displayed currency never drifts from how the cost was actually computed.
// 注意：GLM/MiniMax 等走美元口径 JSON/fallback 的模型按 USD 返回（其成本数值本就是美元口径）。
func ModelPriceCurrency(model string) string {
	ml := strings.ToLower(strings.TrimSpace(model))
	if ml == "" {
		return CurrencyUSD
	}
	// 用户自定义覆盖优先：命中则按其币种展示，与实际计费口径一致。
	if ps := currentPricingService.Load(); ps != nil {
		if ov, ok := ps.matchOverride(ml); ok {
			return ov.Currency
		}
	}
	if _, ok := matchKimiMoonshotCNY(ml); ok {
		return CurrencyCNY
	}
	if _, ok := matchDeepSeekCNY(ml); ok {
		return CurrencyCNY
	}
	if _, ok := matchQwenCNY(ml); ok {
		return CurrencyCNY
	}
	return CurrencyUSD
}

// matchKimiMoonshotCNY reports whether modelLower resolves to an official-RMB
// Kimi/Moonshot price, returning the matched CNY pricing. Pure membership logic
// shared by kimiMoonshotPricingOverride and ModelPriceCurrency (single source of truth).
func matchKimiMoonshotCNY(modelLower string) (cnyModelPricing, bool) {
	m := strings.ReplaceAll(modelLower, "_", "-")
	m = strings.ReplaceAll(m, " ", "")
	// Strip provider prefix: "moonshotai/kimi-k2.6" -> "kimi-k2.6"
	m = lastSegment(m)

	if cny, found := kimiMoonshotPricingTable[m]; found {
		return cny, true
	}
	// kimi-k2.6 variants: kimi-k2-6 / kimi-k26
	if strings.Contains(m, "kimi-k2-6") || strings.Contains(m, "kimi-k26") {
		return kimiMoonshotPricingTable["kimi-k2.6"], true
	}
	// vision-preview variants reuse the base model price
	if strings.HasSuffix(m, "-vision-preview") {
		if cny, found := kimiMoonshotPricingTable[strings.TrimSuffix(m, "-vision-preview")]; found {
			return cny, true
		}
	}
	// 兜底：所有 kimi-* 模型（含未来新模型）统一按 kimi-k2.6 计费
	if strings.HasPrefix(m, "kimi-") {
		return kimiMoonshotPricingTable["kimi-k2.6"], true
	}
	return cnyModelPricing{}, false
}

func (s *PricingService) kimiMoonshotPricingOverride(modelLower string) *LiteLLMModelPricing {
	cny, found := matchKimiMoonshotCNY(modelLower)
	if !found {
		return nil
	}

	rate := s.cnyToUSDRate()
	const perToken = 1.0 / 1_000_000.0
	p := &LiteLLMModelPricing{
		InputCostPerToken:  cny.inputCNY / rate * perToken,
		OutputCostPerToken: cny.outputCNY / rate * perToken,
		LiteLLMProvider:    "moonshot",
		Mode:               "chat",
	}
	if cny.hasCache {
		p.CacheReadInputTokenCost = cny.cacheReadCNY / rate * perToken
		p.SupportsPromptCaching = true
	}
	return p
}

// matchDeepSeekCNY reports whether modelLower resolves to an official-RMB DeepSeek
// price. Pure membership logic shared by deepSeekPricingOverride and ModelPriceCurrency.
func matchDeepSeekCNY(modelLower string) (cnyModelPricing, bool) {
	m := lastSegment(modelLower) // Strip provider prefix: "deepseek/deepseek-v4-flash" -> "deepseek-v4-flash"
	if !strings.HasPrefix(m, "deepseek") {
		return cnyModelPricing{}, false
	}
	// Exact match first
	if cny, found := deepSeekPricingTable[m]; found {
		return cny, true
	}
	// Pattern match: v4-pro takes precedence over v4 (flash)
	switch {
	case strings.Contains(m, "v4-pro"):
		return deepSeekPricingTable["deepseek-v4-pro"], true
	case strings.Contains(m, "v4"):
		return deepSeekPricingTable["deepseek-v4-flash"], true
	}
	return cnyModelPricing{}, false
}

func (s *PricingService) deepSeekPricingOverride(modelLower string) *LiteLLMModelPricing {
	cny, found := matchDeepSeekCNY(modelLower)
	if !found {
		return nil
	}

	rate := s.cnyToUSDRate()
	const perToken = 1.0 / 1_000_000.0
	p := &LiteLLMModelPricing{
		InputCostPerToken:  cny.inputCNY / rate * perToken,
		OutputCostPerToken: cny.outputCNY / rate * perToken,
		LiteLLMProvider:    "deepseek",
		Mode:               "chat",
	}
	if cny.hasCache {
		p.CacheReadInputTokenCost = cny.cacheReadCNY / rate * perToken
		p.SupportsPromptCaching = true
	}
	return p
}

// matchQwenCNY reports whether modelLower resolves to an official-RMB Qwen
// (通义千问/DashScope) price. Pure membership logic shared by qwenPricingOverride
// and ModelPriceCurrency. 精确匹配优先，其次按 coder/flash/turbo/long/max 模式回退，
// 最后所有 qwen*/qwq*/qvq* 兜底按 qwen-plus 计费（避免新模型 $0）。
func matchQwenCNY(modelLower string) (cnyModelPricing, bool) {
	m := lastSegment(modelLower) // Strip provider prefix: "qwen/qwen3-max" -> "qwen3-max"
	if !strings.HasPrefix(m, "qwen") && !strings.HasPrefix(m, "qwq") && !strings.HasPrefix(m, "qvq") {
		return cnyModelPricing{}, false
	}
	if cny, found := qwenPricingTable[m]; found {
		return cny, true
	}
	switch {
	case strings.Contains(m, "coder"):
		if strings.Contains(m, "flash") {
			return qwenPricingTable["qwen3-coder-flash"], true
		}
		return qwenPricingTable["qwen3-coder-plus"], true
	case strings.Contains(m, "flash"):
		return qwenPricingTable["qwen-flash"], true
	case strings.Contains(m, "turbo"):
		return qwenPricingTable["qwen-turbo"], true
	case strings.Contains(m, "long"):
		return qwenPricingTable["qwen-long"], true
	case strings.Contains(m, "max"):
		if strings.Contains(m, "qwen3") || strings.Contains(m, "qwen-3") {
			return qwenPricingTable["qwen3-max"], true
		}
		return qwenPricingTable["qwen-max"], true
	}
	return qwenPricingTable["qwen-plus"], true
}

func (s *PricingService) qwenPricingOverride(modelLower string) *LiteLLMModelPricing {
	cny, found := matchQwenCNY(modelLower)
	if !found {
		return nil
	}

	rate := s.cnyToUSDRate()
	const perToken = 1.0 / 1_000_000.0
	p := &LiteLLMModelPricing{
		InputCostPerToken:  cny.inputCNY / rate * perToken,
		OutputCostPerToken: cny.outputCNY / rate * perToken,
		LiteLLMProvider:    "dashscope",
		Mode:               "chat",
	}
	if cny.hasCache {
		p.CacheReadInputTokenCost = cny.cacheReadCNY / rate * perToken
		p.SupportsPromptCaching = true
	}
	return p
}

// matchByPlatformFallback 为 DeepSeek / Moonshot (Kimi) / GLM 等独立平台模型提供
// 回退定价：当精确匹配和模糊匹配都找不到时，根据模型名前缀回退到该平台已知的
// 基础模型定价。这样新模型上线后即使定价 JSON 尚未更新，也能按同平台最接近的
// 价格计费，而非返回 $0。
func (s *PricingService) matchByPlatformFallback(model string) *LiteLLMModelPricing {
	// ---- 通用平台回退：按前缀匹配远程定价数据中的已知模型 ----
	type platformRule struct {
		prefixes  []string // 用于判断模型是否属于该平台
		fallbacks []string // 按优先级尝试的回退模型名
	}

	rules := []platformRule{
		{prefixes: []string{"deepseek"}, fallbacks: []string{"deepseek-chat", "deepseek-reasoner"}},
		{prefixes: []string{"kimi", "moonshot"}, fallbacks: []string{"kimi-k2.6", "kimi-k2.5"}},
		{prefixes: []string{"glm"}, fallbacks: []string{"glm-5.1"}},
	}

	for _, rule := range rules {
		matched := false
		for _, prefix := range rule.prefixes {
			if strings.HasPrefix(model, prefix) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		for _, fb := range rule.fallbacks {
			if pricing, ok := s.pricingData[fb]; ok {
				logger.LegacyPrintf("service.pricing",
					"[Pricing] Platform fallback matched %s -> %s", model, fb)
				return pricing
			}
		}
	}
	return nil
}

// GetModelMaxOutputTokens 返回模型 output token 物理上限(来自 LiteLLM 价目文件,
// 复用 GetModelPricing 的归一化/别名/快照匹配)。无上限数据返回 (0,false)。
// 用于计费前对上游上报 output 封顶,防伪造灌水(2026-06 gegemini 事件)。
func (s *PricingService) GetModelMaxOutputTokens(modelName string) (int, bool) {
	p := s.GetModelPricing(modelName)
	if p == nil || p.MaxOutputTokens <= 0 {
		return 0, false
	}
	return p.MaxOutputTokens, true
}

// GetModelPricing 获取模型价格（带模糊匹配）
func (s *PricingService) GetModelPricing(modelName string) *LiteLLMModelPricing {
	if modelName == "" {
		return nil
	}

	// 标准化模型名称（同时兼容 "models/xxx"、VertexAI 资源名等前缀）
	modelLower := strings.ToLower(strings.TrimSpace(modelName))

	// 用户自定义价格覆盖表（UI 配置，优先级最高，先于内置 ¥ 表与 JSON 同步源）。
	// 放在 s.mu.RLock 之前：matchOverride 走独立 overrideMu，缓存 miss 时会做 DB 读，
	// 绝不能在 s.mu.RLock 持有期内进行，否则慢 DB 调用会阻塞 pricing 周期刷新的写锁与
	// 整条计费热路径（s.mu 是 writer-preferring）。
	if ov, ok := s.matchOverride(modelLower); ok {
		return s.overrideToLiteLLM(ov)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Kimi / Moonshot：官方人民币计价，用官方价 + 可配置汇率折算成美元覆盖，
	// 确保按官方价计费（汇率见 pricing.cny_to_usd_rate 配置）。
	if pricing := s.kimiMoonshotPricingOverride(modelLower); pricing != nil {
		return pricing
	}

	// DeepSeek V4：同上，人民币官方计价，运行时汇率折算。
	if pricing := s.deepSeekPricingOverride(modelLower); pricing != nil {
		return pricing
	}

	// Qwen（通义千问/DashScope）：同上，阿里云百炼官方人民币计价，运行时汇率折算。
	if pricing := s.qwenPricingOverride(modelLower); pricing != nil {
		return pricing
	}

	lookupCandidates := s.buildModelLookupCandidates(modelLower)

	// 1. 精确匹配
	for _, candidate := range lookupCandidates {
		if candidate == "" {
			continue
		}
		if pricing, ok := s.pricingData[candidate]; ok {
			return pricing
		}
	}

	// 2. 处理常见的模型名称变体
	// claude-opus-4-5-20251101 -> claude-opus-4.5-20251101
	for _, candidate := range lookupCandidates {
		normalized := strings.ReplaceAll(candidate, "-4-5-", "-4.5-")
		if pricing, ok := s.pricingData[normalized]; ok {
			return pricing
		}
	}

	// 3. 尝试模糊匹配（去掉版本号后缀）
	// claude-opus-4-5-20251101 -> claude-opus-4.5
	baseName := s.extractBaseName(lookupCandidates[0])
	for key, pricing := range s.pricingData {
		keyBase := s.extractBaseName(strings.ToLower(key))
		if keyBase == baseName {
			return pricing
		}
	}

	// 4. 基于模型系列匹配（Claude）
	if pricing := s.matchByModelFamily(lookupCandidates[0]); pricing != nil {
		return pricing
	}

	// 5. OpenAI 模型回退策略
	if strings.HasPrefix(lookupCandidates[0], "gpt-") {
		return s.matchOpenAIModel(lookupCandidates[0])
	}

	// 6. 平台级回退（DeepSeek / Moonshot / GLM）
	if pricing := s.matchByPlatformFallback(lookupCandidates[0]); pricing != nil {
		return pricing
	}

	return nil
}

func (s *PricingService) buildModelLookupCandidates(modelLower string) []string {
	// Prefer canonical model name first (this also improves billing compatibility with "models/xxx").
	candidates := []string{
		normalizeModelNameForPricing(modelLower),
		modelLower,
	}
	candidates = append(candidates,
		strings.TrimPrefix(modelLower, "models/"),
		lastSegment(modelLower),
		lastSegment(strings.TrimPrefix(modelLower, "models/")),
	)

	seen := make(map[string]struct{}, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	if len(out) == 0 {
		return []string{modelLower}
	}
	return out
}

func normalizeModelNameForPricing(model string) string {
	// Common Gemini/VertexAI forms:
	// - models/gemini-2.0-flash-exp
	// - publishers/google/models/gemini-2.5-pro
	// - projects/.../locations/.../publishers/google/models/gemini-2.5-pro
	model = strings.TrimSpace(model)
	model = strings.TrimLeft(model, "/")
	model = strings.TrimPrefix(model, "models/")
	model = strings.TrimPrefix(model, "publishers/google/models/")

	if idx := strings.LastIndex(model, "/publishers/google/models/"); idx != -1 {
		model = model[idx+len("/publishers/google/models/"):]
	}
	if idx := strings.LastIndex(model, "/models/"); idx != -1 {
		model = model[idx+len("/models/"):]
	}

	model = strings.TrimLeft(model, "/")
	if canonical := canonicalizeOpenAIModelAliasSpelling(model); canonical != "" {
		if canonical == "gpt-5.6" {
			return "gpt-5.6-sol"
		}
		if suffix, ok := strings.CutPrefix(canonical, "gpt-5.6-"); ok && (suffix == "max" || isKnownCodexModelSuffix(suffix)) {
			return "gpt-5.6-sol"
		}
		return canonical
	}
	return model
}

func lastSegment(model string) string {
	if idx := strings.LastIndex(model, "/"); idx != -1 {
		return model[idx+1:]
	}
	return model
}

// extractBaseName 提取基础模型名称（去掉日期版本号）
func (s *PricingService) extractBaseName(model string) string {
	// 移除日期后缀 (如 -20251101, -20241022)
	parts := strings.Split(model, "-")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		// 跳过看起来像日期的部分（8位数字）
		if len(part) == 8 && isNumeric(part) {
			continue
		}
		// 跳过版本号（如 v1:0）
		if strings.Contains(part, ":") {
			continue
		}
		result = append(result, part)
	}
	return strings.Join(result, "-")
}

// matchByModelFamily 基于模型系列匹配
func (s *PricingService) matchByModelFamily(model string) *LiteLLMModelPricing {
	// modelFamily 定义一个模型系列的匹配和定价查找规则。
	type modelFamily struct {
		name    string   // 系列名称
		match   []string // 用于将模型归类到此系列的模式（strings.Contains 匹配）
		pricing []string // 用于在定价数据中查找价格的模式（nil 则复用 match；可包含低版本 fallback）
	}

	// 按特异性降序排列：高版本号在前，避免 "claude-opus-4"（opus-4 系列）
	// 因子串关系误匹配 "claude-opus-4-7"（opus-4.7 系列）。
	// 注意：原 map 实现存在 Go map 迭代随机性导致的同类 bug，此处改为有序切片修复。
	families := []modelFamily{
		{name: "fable-5", match: []string{"claude-fable-5", "claude-fable"}},
		{name: "opus-4.7", match: []string{"claude-opus-4-7", "claude-opus-4.7"}, pricing: []string{"claude-opus-4-7", "claude-opus-4.7", "claude-opus-4-6"}},
		{name: "opus-4.6", match: []string{"claude-opus-4-6", "claude-opus-4.6"}},
		{name: "opus-4.5", match: []string{"claude-opus-4-5", "claude-opus-4.5"}},
		{name: "opus-4", match: []string{"claude-opus-4", "claude-3-opus"}},
		{name: "sonnet-4.5", match: []string{"claude-sonnet-4-5", "claude-sonnet-4.5"}},
		{name: "sonnet-4", match: []string{"claude-sonnet-4", "claude-3-5-sonnet"}},
		{name: "sonnet-3.5", match: []string{"claude-3-5-sonnet", "claude-3.5-sonnet"}},
		{name: "sonnet-3", match: []string{"claude-3-sonnet"}},
		{name: "haiku-3.5", match: []string{"claude-3-5-haiku", "claude-3.5-haiku"}},
		{name: "haiku-3", match: []string{"claude-3-haiku"}},
	}

	// Phase 1: 按有序切片归类（最具体的系列优先匹配）
	var matched *modelFamily
	for i := range families {
		for _, pattern := range families[i].match {
			if strings.Contains(model, pattern) || strings.Contains(model, strings.ReplaceAll(pattern, "-", "")) {
				matched = &families[i]
				break
			}
		}
		if matched != nil {
			break
		}
	}

	// Phase 2: 二次兜底——当模型 ID 不含已知模式串时，按关键字粗分
	if matched == nil {
		var fallbackName string
		switch {
		case strings.Contains(model, "fable"):
			fallbackName = "fable-5"
		case strings.Contains(model, "opus"):
			switch {
			case strings.Contains(model, "4.7") || strings.Contains(model, "4-7"):
				fallbackName = "opus-4.7"
			case strings.Contains(model, "4.6") || strings.Contains(model, "4-6"):
				fallbackName = "opus-4.6"
			case strings.Contains(model, "4.5") || strings.Contains(model, "4-5"):
				fallbackName = "opus-4.5"
			default:
				fallbackName = "opus-4"
			}
		case strings.Contains(model, "sonnet"):
			switch {
			case strings.Contains(model, "4.5") || strings.Contains(model, "4-5"):
				fallbackName = "sonnet-4.5"
			case strings.Contains(model, "3-5") || strings.Contains(model, "3.5"):
				fallbackName = "sonnet-3.5"
			default:
				fallbackName = "sonnet-4"
			}
		case strings.Contains(model, "haiku"):
			switch {
			case strings.Contains(model, "3-5") || strings.Contains(model, "3.5"):
				fallbackName = "haiku-3.5"
			default:
				fallbackName = "haiku-3"
			}
		}
		if fallbackName != "" {
			for i := range families {
				if families[i].name == fallbackName {
					matched = &families[i]
					break
				}
			}
		}
	}

	if matched == nil {
		return nil
	}

	// Phase 3: 在定价数据中查找该系列的价格
	lookups := matched.pricing
	if lookups == nil {
		lookups = matched.match
	}
	for _, pattern := range lookups {
		for key, pricing := range s.pricingData {
			keyLower := strings.ToLower(key)
			if strings.Contains(keyLower, pattern) {
				logger.LegacyPrintf("service.pricing", "[Pricing] Fuzzy matched %s -> %s", model, key)
				return pricing
			}
		}
	}

	// Fable 5 系列：定价数据未收录时按官方静态价计费，避免返回 $0
	if matched.name == "fable-5" {
		logger.LegacyPrintf("service.pricing", "[Pricing] Fable fallback matched %s -> claude-fable-5(static)", model)
		return claudeFable5FallbackPricing
	}

	return nil
}

// matchOpenAIModel OpenAI 模型回退匹配策略
// 回退顺序：
// 1. gpt-5.3-codex-spark* -> gpt-5.1-codex（按业务要求固定计费）
// 2. gpt-5.2-codex -> gpt-5.2（去掉后缀如 -codex, -mini, -max 等）
// 3. gpt-5.2-20251222 -> gpt-5.2（去掉日期版本号）
// 4. gpt-5.3-codex -> gpt-5.2-codex
// 5. gpt-5.4* -> 业务静态兜底价
// 6. 最终回退到 DefaultTestModel (gpt-5.1-codex)
func (s *PricingService) matchOpenAIModel(model string) *LiteLLMModelPricing {
	if strings.HasPrefix(model, "gpt-5.3-codex-spark") {
		if pricing, ok := s.pricingData["gpt-5.1-codex"]; ok {
			logger.LegacyPrintf("service.pricing", "[Pricing][SparkBilling] %s -> %s billing", model, "gpt-5.1-codex")
			logger.With(zap.String("component", "service.pricing")).
				Info(fmt.Sprintf("[Pricing] OpenAI fallback matched %s -> %s", model, "gpt-5.1-codex"))
			return pricing
		}
	}

	// 尝试的回退变体
	variants := s.generateOpenAIModelVariants(model, openAIModelDatePattern)

	for _, variant := range variants {
		if pricing, ok := s.pricingData[variant]; ok {
			logger.With(zap.String("component", "service.pricing")).
				Info(fmt.Sprintf("[Pricing] OpenAI fallback matched %s -> %s", model, variant))
			return pricing
		}
	}

	if strings.HasPrefix(model, "gpt-5.3-codex") {
		if pricing, ok := s.pricingData["gpt-5.2-codex"]; ok {
			logger.With(zap.String("component", "service.pricing")).
				Info(fmt.Sprintf("[Pricing] OpenAI fallback matched %s -> %s", model, "gpt-5.2-codex"))
			return pricing
		}
	}

	if strings.HasPrefix(model, "gpt-5.6-sol") {
		logger.With(zap.String("component", "service.pricing")).
			Info(fmt.Sprintf("[Pricing] OpenAI fallback matched %s -> %s", model, "gpt-5.6-sol(static)"))
		return openAIGPT56SolFallbackPricing
	}
	if strings.HasPrefix(model, "gpt-5.6-terra") {
		logger.With(zap.String("component", "service.pricing")).
			Info(fmt.Sprintf("[Pricing] OpenAI fallback matched %s -> %s", model, "gpt-5.6-terra(static)"))
		return openAIGPT56TerraFallbackPricing
	}
	if strings.HasPrefix(model, "gpt-5.6-luna") {
		logger.With(zap.String("component", "service.pricing")).
			Info(fmt.Sprintf("[Pricing] OpenAI fallback matched %s -> %s", model, "gpt-5.6-luna(static)"))
		return openAIGPT56LunaFallbackPricing
	}

	// GPT-5.5 回退到 GPT-5.4 定价
	if strings.HasPrefix(model, "gpt-5.5") {
		logger.With(zap.String("component", "service.pricing")).
			Info(fmt.Sprintf("[Pricing] OpenAI fallback matched %s -> %s", model, "gpt-5.4(static)"))
		return openAIGPT54FallbackPricing
	}

	if strings.HasPrefix(model, "gpt-5.4-mini") {
		logger.With(zap.String("component", "service.pricing")).
			Info(fmt.Sprintf("[Pricing] OpenAI fallback matched %s -> %s", model, "gpt-5.4-mini(static)"))
		return openAIGPT54MiniFallbackPricing
	}

	if strings.HasPrefix(model, "gpt-5.4-nano") {
		logger.With(zap.String("component", "service.pricing")).
			Info(fmt.Sprintf("[Pricing] OpenAI fallback matched %s -> %s", model, "gpt-5.4-nano(static)"))
		return openAIGPT54NanoFallbackPricing
	}

	if strings.HasPrefix(model, "gpt-5.4") {
		logger.With(zap.String("component", "service.pricing")).
			Info(fmt.Sprintf("[Pricing] OpenAI fallback matched %s -> %s", model, "gpt-5.4(static)"))
		return openAIGPT54FallbackPricing
	}

	if isOpenAIImageGenerationModel(model) {
		for _, candidate := range []string{"gpt-image-2", "gpt-image-1.5", "gpt-image-1"} {
			if pricing, ok := s.pricingData[candidate]; ok {
				logger.LegacyPrintf("service.pricing", "[Pricing] OpenAI image fallback matched %s -> %s", model, candidate)
				return pricing
			}
		}
		return nil
	}

	// 最终回退到 DefaultTestModel
	defaultModel := strings.ToLower(openai.DefaultTestModel)
	if pricing, ok := s.pricingData[defaultModel]; ok {
		logger.LegacyPrintf("service.pricing", "[Pricing] OpenAI fallback to default model %s -> %s", model, defaultModel)
		return pricing
	}

	return nil
}

// generateOpenAIModelVariants 生成 OpenAI 模型的回退变体列表
func (s *PricingService) generateOpenAIModelVariants(model string, datePattern *regexp.Regexp) []string {
	seen := make(map[string]bool)
	var variants []string

	addVariant := func(v string) {
		if v != model && !seen[v] {
			seen[v] = true
			variants = append(variants, v)
		}
	}

	// 1. 去掉日期版本号: gpt-5.2-20251222 -> gpt-5.2
	withoutDate := datePattern.ReplaceAllString(model, "")
	if withoutDate != model {
		addVariant(withoutDate)
	}

	// 2. 提取基础版本号: gpt-5.2-codex -> gpt-5.2
	// 只匹配纯数字版本号格式 gpt-X 或 gpt-X.Y，不匹配 gpt-4o 这种带字母后缀的
	if matches := openAIModelBasePattern.FindStringSubmatch(model); len(matches) > 1 {
		addVariant(matches[1])
	}

	// 3. 同时去掉日期后再提取基础版本号
	if withoutDate != model {
		if matches := openAIModelBasePattern.FindStringSubmatch(withoutDate); len(matches) > 1 {
			addVariant(matches[1])
		}
	}

	return variants
}

// LiteLLMModelEntry 列表返回项：模型名 + 价格数据（用于 admin 批量填充渠道定价）。
type LiteLLMModelEntry struct {
	Model   string
	Pricing *LiteLLMModelPricing
}

// ListAll 返回 LiteLLM 价格表中所有模型，按字母序排序。
//
//   - providerFilter 非空时按 litellm_provider 大小写不敏感匹配过滤
//   - 仅返回 mode == "chat" 或为空（兼容老条目）的模型；embedding/image_generation
//     等非 chat 模式不参与渠道定价批量填充
func (s *PricingService) ListAll(providerFilter string) []LiteLLMModelEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	filter := strings.ToLower(strings.TrimSpace(providerFilter))
	out := make([]LiteLLMModelEntry, 0, len(s.pricingData))
	for name, p := range s.pricingData {
		if p == nil {
			continue
		}
		if filter != "" && !strings.EqualFold(p.LiteLLMProvider, filter) {
			continue
		}
		if p.Mode != "" && p.Mode != "chat" {
			continue
		}
		out = append(out, LiteLLMModelEntry{Model: name, Pricing: p})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Model < out[j].Model })
	return out
}

// GetStatus 获取服务状态
func (s *PricingService) GetStatus() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]any{
		"model_count":  len(s.pricingData),
		"last_updated": s.lastUpdated,
		"local_hash":   s.localHash[:min(8, len(s.localHash))],
	}
}

// ForceUpdate 强制更新
func (s *PricingService) ForceUpdate() error {
	return s.downloadPricingData()
}

// getPricingFilePath 获取价格文件路径
func (s *PricingService) getPricingFilePath() string {
	return filepath.Join(s.cfg.Pricing.DataDir, "model_pricing.json")
}

// getHashFilePath 获取哈希文件路径
func (s *PricingService) getHashFilePath() string {
	return filepath.Join(s.cfg.Pricing.DataDir, "model_pricing.sha256")
}

// ListModelNamesByProvider returns all model names in the catalog whose
// LiteLLMProvider matches the given provider string (case-insensitive).
// The returned slice is sorted alphabetically.
func (s *PricingService) ListModelNamesByProvider(provider string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	provider = strings.ToLower(strings.TrimSpace(provider))
	names := make([]string, 0)
	for name, p := range s.pricingData {
		if strings.ToLower(p.LiteLLMProvider) == provider {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// isNumeric 检查字符串是否为纯数字
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
