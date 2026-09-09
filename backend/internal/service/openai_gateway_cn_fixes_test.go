//go:build unit

package service

// 国产供应商功能修复回归测试：
//  1. CN 分组的 /v1/messages 调度级模型映射不得回落 openai 的 gpt-5.x 默认值
//     （本 fork 改为按平台兜底，见下方用例注释）；
//  2. 计费候选链对 CN 账号过滤 claude-* 兜底候选（防按 Claude 原价误计 CN 流量）；
//  3. 空候选按 ErrModelPricingUnavailable 处理（零成本落账而非丢弃 usage 记录）；
//  4. Responses×anthropic 流式转换器客户端断开后继续排水、usage 汇总完整。

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 由上游同名用例改写（上游名 ...NoDispatchMapping）。上游断言 CN 分组一律返回空，
// 理由是「openai 的 gpt-5.x 默认值发给 CN 上游必错」。这个理由本 fork 同样成立，
// 但解法不同：上游假定 CN 账号配 api_protocol=anthropic 原生直通、模型改写全交给
// 账号级 model_mapping；本 fork 合并前的既有行为是分组级按平台兜底映射，CN 分组的
// Claude Code 流量（claude-* 模型名）靠它翻成各家自己的型号。改成返回空会让 claude-*
// 原样透传到国产上游：轻则选号/上游 400，重则上游接受后被
// filterCNProviderBillingModelCandidates 滤空候选 → 零成本落账。
// 所以这里守的是「不得吃到 openai 专属型号」，达成方式是按平台各自兜底。
//
// 关于 MiniMax（本轮上游新增的 CN 平台，上游把它加进了同名用例的平台列表）：
// 这里**故意**不纳入 want 表。defaultMessagesDispatchModels 目前没有 minimax 分支，
// 本仓也没有确凿的 MiniMax 调度兜底型号可用——handler 侧的
// defaultCodexModelIDsForPlatform / 前端白名单里的 MiniMax-M3 / M2.7 / M2.5
// 是 /v1/models 展示列表口径，不等于调度兜底口径（且 haiku 档该落哪一档也无据可依）。
// 随手编一个会把错误型号钉进回归基线。待人工确认兜底型号后，再同时补
// defaultMessagesDispatchModels 的 minimax 分支和这里的 want 条目；在那之前
// minimax 分组仍会落到 gpt-5.x 默认值，属已知缺口（见合并 followups）。
func TestResolveMessagesDispatchModel_CNProvidersUsePlatformDefaults(t *testing.T) {
	want := map[string]string{
		PlatformKimi:     "kimi-k2.6",
		PlatformZhipu:    "glm-4.6",
		PlatformDeepseek: "deepseek-v4-pro",
	}
	for platform, expected := range want {
		g := &Group{Platform: platform}
		for _, model := range []string{"claude-sonnet-4-5", "claude-opus-4-1"} {
			got := g.ResolveMessagesDispatchModel(model)
			require.Equal(t, expected, got, "CN 分组(%s) 应按平台兜底: model=%s", platform, model)
			require.NotContains(t, got, "gpt-",
				"CN 分组(%s)不得返回 openai 专属型号: model=%s", platform, model)
		}
	}
	// 非回归：openai 分组保持原有默认映射行为。
	openaiGroup := &Group{Platform: PlatformOpenAI}
	require.NotEmpty(t, openaiGroup.ResolveMessagesDispatchModel("claude-sonnet-4-5"),
		"openai 分组的调度默认映射不应受 CN 修复影响")
}

func TestFilterCNProviderBillingModelCandidates(t *testing.T) {
	svc := &OpenAIGatewayService{} // resolver 为 nil → 无显式分组/渠道定价
	apiKey := &APIKey{Group: &Group{ID: 1, Platform: PlatformKimi}}

	cnAccount := &Account{ID: 1, Platform: PlatformKimi}
	filtered := svc.filterCNProviderBillingModelCandidates(context.Background(), cnAccount, apiKey,
		[]string{"kimi-k2-0905-preview", "claude-sonnet-4-5", "moonshot-v1-8k"})
	require.Equal(t, []string{"kimi-k2-0905-preview", "moonshot-v1-8k"}, filtered,
		"无显式定价时 claude-* 候选必须被过滤")

	allClaude := svc.filterCNProviderBillingModelCandidates(context.Background(), cnAccount, apiKey,
		[]string{"claude-sonnet-4-5", "claude-sonnet-4-5"})
	require.Empty(t, allClaude, "全 claude 候选应被清空（上层走零成本+告警落账）")

	// 非 CN 账号完全不受影响。
	openaiAccount := &Account{ID: 2, Platform: PlatformOpenAI}
	passthrough := svc.filterCNProviderBillingModelCandidates(context.Background(), openaiAccount, apiKey,
		[]string{"claude-sonnet-4-5", "gpt-5.4"})
	require.Equal(t, []string{"claude-sonnet-4-5", "gpt-5.4"}, passthrough)

	require.Nil(t, svc.filterCNProviderBillingModelCandidates(context.Background(), nil, apiKey, nil))
}

func TestCalculateOpenAIRecordUsageCost_EmptyCandidatesIsPricingUnavailable(t *testing.T) {
	svc := &OpenAIGatewayService{}
	apiKey := &APIKey{Group: &Group{ID: 1, Platform: PlatformKimi}}

	_, err := svc.calculateOpenAIRecordUsageCost(
		context.Background(), nil, apiKey, nil,
		1.0, 1.0, 1.0, 1.0, UsageTokens{InputTokens: 100}, "", nil, time.Time{},
	)
	require.Error(t, err)
	require.True(t, isUsagePricingUnavailableError(err),
		"空候选必须按无价可循处理（上层零成本落账），而不是丢弃整条 usage 记录: %v", err)
}

func TestResponsesStreamingFromNativeAnthropic_ClientDisconnectDrainsUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newNativeAnthropicHangTestService(5)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	// failAfter=0：首次写出即失败，模拟客户端断开（复用测试包既有 failingGinWriter）。
	failWriter := &failingGinWriter{ResponseWriter: c.Writer, failAfter: 0}
	c.Writer = failWriter

	resp, pr, pw := newHangingUpstreamResponse()
	go func() {
		// 首事件触发客户端写失败后，末尾 message_delta 才携带最终 output_tokens：
		// 断开即弃会把整段生成记成 1 token。
		_, _ = pw.Write([]byte(miniAnthropicSSEStream()))
		_ = pw.Close()
	}()
	defer func() { _ = pr.Close() }()

	res, err := svc.handleResponsesStreamingFromNativeAnthropic(
		resp, c, "glm-4.7", "glm-4.7", "glm-4.7", nil, time.Now(), apicompat.ResponsesClientToolMapping{})

	require.NoError(t, err, "断开排水至上游自然结束应返回 nil error（usage 走成功路径落账）")
	require.NotNil(t, res)
	require.True(t, res.ClientDisconnect)
	require.Equal(t, 10, res.Usage.InputTokens, "input_tokens 应来自 message_start")
	require.Equal(t, 5, res.Usage.OutputTokens,
		"output_tokens 必须来自排水读到的末尾 message_delta（断开即弃时会是 1）")
}

func TestHandle403_CNProviderHTMLBodySkipsAccountPenalty(t *testing.T) {
	for _, platform := range []string{PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax} {
		repo := &rateLimitAccountRepoStub{}
		service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := &Account{ID: 401, Platform: platform, Type: AccountTypeAPIKey}

		shouldDisable := service.HandleUpstreamError(
			context.Background(),
			account,
			http.StatusForbidden,
			http.Header{},
			[]byte("<html><body>Access denied by CDN</body></html>"),
		)

		require.False(t, shouldDisable, "%s: HTML 403（CDN/代理拦截页）不得作为账号失效证据", platform)
		require.Equal(t, 0, repo.setErrorCalls, "%s: 不得永久禁用账号", platform)
		require.Equal(t, 0, repo.tempCalls, "%s: 不得临时停调账号", platform)
	}
}

func TestHandle403_CNProviderStructured403TempUnschedulableFirstHit(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	counter := &openAI403CounterCacheStub{counts: []int64{1}}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service.SetOpenAI403CounterCache(counter)
	account := &Account{ID: 402, Platform: PlatformKimi, Type: AccountTypeAPIKey}

	shouldDisable := service.HandleUpstreamError(
		context.Background(),
		account,
		http.StatusForbidden,
		http.Header{},
		[]byte(`{"error":{"message":"forbidden"}}`),
	)

	require.True(t, shouldDisable)
	require.Equal(t, 0, repo.setErrorCalls, "首次结构化 403 应临时停调而非永久禁用")
	require.Equal(t, 1, repo.tempCalls)
	require.Contains(t, repo.lastTempReason, "(1/3)")
}

func TestIsCNProviderConcurrencyLimit403_ExactClassification(t *testing.T) {
	kimi := &Account{Platform: PlatformKimi}

	require.True(t, isCNProviderConcurrencyLimit403(kimi, kimiConcurrentRequestLimitMessage))
	require.True(t, isCNProviderConcurrencyLimit403(kimi, "  "+kimiConcurrentRequestLimitMessage+"\n"))

	for name, tc := range map[string]struct {
		account *Account
		message string
	}{
		"permission denied":              {kimi, "You do not have permission to access this resource."},
		"generic concurrency wording":    {kimi, "concurrent request limit reached"},
		"near match missing punctuation": {kimi, "You've reached your concurrent request limit. Please wait for your ongoing requests to finish and try again"},
		"other CN provider":              {&Account{Platform: PlatformZhipu}, kimiConcurrentRequestLimitMessage},
		"non CN provider":                {&Account{Platform: PlatformOpenAI}, kimiConcurrentRequestLimitMessage},
		"nil account":                    {nil, kimiConcurrentRequestLimitMessage},
	} {
		t.Run(name, func(t *testing.T) {
			require.False(t, isCNProviderConcurrencyLimit403(tc.account, tc.message))
		})
	}
}

func TestHandle403_OtherCNProviderWithKimiConcurrencyMessageUsesNormalPolicy(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	counter := &openAI403CounterCacheStub{counts: []int64{openAI403DisableThreshold}}
	blocker := &runtimeBlockRecorder{}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service.SetOpenAI403CounterCache(counter)
	service.SetAccountRuntimeBlocker(blocker)
	account := &Account{ID: 405, Platform: PlatformZhipu, Type: AccountTypeAPIKey}

	shouldDisable := service.HandleUpstreamError(
		context.Background(), account, http.StatusForbidden, http.Header{},
		[]byte(`{"error":{"message":"You've reached your concurrent request limit. Please wait for your ongoing requests to finish and try again."}}`),
	)

	require.True(t, shouldDisable)
	require.Equal(t, 1, repo.setErrorCalls, "non-Kimi CN provider must retain the normal permanent-error policy")
	require.Equal(t, 0, repo.tempCalls)
	require.Empty(t, counter.counts, "normal CN 403 policy must consume the counter result")
	require.Equal(t, []string{"auth_error"}, blocker.reasons, "the Kimi-specific runtime block must not apply")
}

func TestHandle403_CNProviderConcurrencyLimitAlwaysUsesTemporaryCooldown(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	counter := &openAI403CounterCacheStub{counts: []int64{openAI403DisableThreshold}}
	blocker := &runtimeBlockRecorder{}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service.SetOpenAI403CounterCache(counter)
	service.SetAccountRuntimeBlocker(blocker)
	account := &Account{ID: 403, Platform: PlatformKimi, Type: AccountTypeAPIKey}

	shouldDisable := service.HandleUpstreamError(
		context.Background(), account, http.StatusForbidden, http.Header{},
		[]byte(`{"error":{"message":"You've reached your concurrent request limit. Please wait for your ongoing requests to finish and try again."}}`),
	)

	require.True(t, shouldDisable, "the request must still fail over to another account")
	require.Equal(t, 0, repo.setErrorCalls)
	require.Equal(t, 1, repo.tempCalls)
	require.Contains(t, repo.lastTempReason, cnConcurrencyLimitReasonPrefix)
	require.Equal(t, []int64{openAI403DisableThreshold}, counter.counts, "transient concurrency 403 must bypass the permanent-error counter")
	require.Len(t, blocker.accounts, 1)
	require.Equal(t, cnConcurrencyLimitReasonPrefix, blocker.reasons[0])
	require.True(t, blocker.until[0].After(time.Now()))
}

func TestHandle403_KimiConcurrencyLimitRepositoryFailureKeepsRuntimeBlock(t *testing.T) {
	repo := &rateLimitAccountRepoStub{tempErr: errors.New("repository unavailable")}
	counter := &openAI403CounterCacheStub{counts: []int64{openAI403DisableThreshold}}
	blocker := &runtimeBlockRecorder{}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service.SetOpenAI403CounterCache(counter)
	service.SetAccountRuntimeBlocker(blocker)
	account := &Account{ID: 406, Platform: PlatformKimi, Type: AccountTypeAPIKey}

	shouldDisable := service.HandleUpstreamError(
		context.Background(), account, http.StatusForbidden, http.Header{},
		[]byte(`{"error":{"message":"You've reached your concurrent request limit. Please wait for your ongoing requests to finish and try again."}}`),
	)

	require.True(t, shouldDisable, "the current request must fail over even when persistence fails")
	require.Equal(t, 1, repo.tempCalls, "the temporary cooldown should still be persisted when possible")
	require.Equal(t, 0, repo.setErrorCalls, "persistence failure must not fall back to permanent account error")
	require.Equal(t, []int64{openAI403DisableThreshold}, counter.counts, "persistence failure must not enter the permanent-error counter path")
	require.Len(t, blocker.accounts, 1, "the in-memory runtime block must survive repository failure")
	require.Same(t, account, blocker.accounts[0])
	require.Equal(t, cnConcurrencyLimitReasonPrefix, blocker.reasons[0])
	require.True(t, blocker.until[0].After(time.Now()))
}

func TestHandle403_CNProviderNearMatchRetainsNormalPermanentErrorPolicy(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	counter := &openAI403CounterCacheStub{counts: []int64{openAI403DisableThreshold}}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service.SetOpenAI403CounterCache(counter)
	account := &Account{ID: 404, Platform: PlatformKimi, Type: AccountTypeAPIKey}

	shouldDisable := service.HandleUpstreamError(
		context.Background(), account, http.StatusForbidden, http.Header{},
		[]byte(`{"error":{"message":"You've reached your concurrent request limit. Please contact support."}}`),
	)

	require.True(t, shouldDisable)
	require.Equal(t, 1, repo.setErrorCalls, "non-exact 403 must retain existing permission/auth protection")
	require.Equal(t, 0, repo.tempCalls)
}

// 泛化守卫：无论上游以后再加多少个国产平台，CN 分组的调度兜底都不得吐出
// openai 专属型号。上一轮（0.2.4 加 minimax）正是因为只有逐平台 case、
// default 直落 gpt-5.x，导致 MiniMax 分组会把 gpt-5.4 发给 api.minimaxi.com。
// 这条用例按 IsCNProvider 遍历全部国产平台，新增平台会自动纳入，不用改测试。
func TestDefaultMessagesDispatchModels_NoCNPlatformFallsBackToOpenAIModels(t *testing.T) {
	cnPlatforms := []string{}
	for _, p := range AllowedQuotaPlatforms {
		if IsCNProvider(p) {
			cnPlatforms = append(cnPlatforms, p)
		}
	}
	require.NotEmpty(t, cnPlatforms, "AllowedQuotaPlatforms 里应当有国产平台")

	for _, platform := range cnPlatforms {
		g := &Group{Platform: platform}
		opus, sonnet, haiku := g.defaultMessagesDispatchModels()
		for _, got := range []string{opus, sonnet, haiku} {
			require.NotContains(t, got, "gpt-",
				"CN 平台 %s 的调度兜底不得是 openai 专属型号（拿到 %q）；"+
					"新增国产平台要么在 defaultMessagesDispatchModels 里补 case，"+
					"要么让它落到返回空的 IsCNProvider 分支", platform, got)
		}
	}
}

// 泛化守卫：命中 fork 的 CN Anthropic 直通分支的平台，必须都能拼出上游 URL。
// usesLegacyCNAnthropicDirect 的条件是「APIKey 账号 + IsCNProvider + 没写 api_protocol」，
// 上游新增国产平台会自动满足它；buildAnthropicDirectMessagesURL 少一个 case
// 就返回空串、forwardAnthropicDirect 直接报 unsupported platform。
func TestBuildAnthropicDirectMessagesURL_CoversEveryCNPlatform(t *testing.T) {
	for _, platform := range AllowedQuotaPlatforms {
		if !IsCNProvider(platform) {
			continue
		}
		account := &Account{Platform: platform, Type: AccountTypeAPIKey}
		require.True(t, usesLegacyCNAnthropicDirect(account),
			"CN 平台 %s 的 APIKey 账号（未配 api_protocol）应命中直通分支", platform)
		got := buildAnthropicDirectMessagesURL(account)
		require.NotEmpty(t, got,
			"buildAnthropicDirectMessagesURL 缺 %s 分支：会返回空串并让 forwardAnthropicDirect 报 unsupported platform", platform)
		require.True(t, strings.HasSuffix(got, "/messages"),
			"CN 平台 %s 拼出的直通 URL 应以 /messages 结尾，实际 %q", platform, got)
	}
}
