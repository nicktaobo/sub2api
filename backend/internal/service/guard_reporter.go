package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

// GuardSample 上报给 guard(upstream-monitor)/api/agent/audit 的真实流量样本。
// 注意:CompletionTokens 故意送"上游上报的原始 output"(L3 封顶之前的值),这样 guard
// 侧 relay:output_over_max 规则才能检测到伪造灌水;BilledAmount 另送,便于"上报值 vs
// 实计费"对账(2026-06 gegemini:号池伪造 output 计费、超模型上限 18×)。
type GuardSample struct {
	Sub2apiAccountID int64          `json:"sub2api_account_id"`
	Model            string         `json:"model_name"`
	RequestedModel   string         `json:"requested_model,omitempty"`
	ResponseModel    string         `json:"response_model,omitempty"`
	RequestID        string         `json:"request_id,omitempty"`
	HTTPStatus       int            `json:"http_status,omitempty"`
	IsStream         bool           `json:"is_stream"`
	StreamCompleted  bool           `json:"stream_completed"`
	PromptTokens     int            `json:"prompt_tokens"`
	CompletionTokens int            `json:"completion_tokens"` // 原始上游上报 output(未封顶)
	TotalTokens      int            `json:"total_tokens"`
	BilledAmount     float64        `json:"billed_amount"`
	Detail           map[string]any `json:"detail,omitempty"`
}

// GuardReporter fire-and-forget 上报器。所有错误仅 WARN 吞掉,绝不阻塞/影响计费与响应。
type GuardReporter struct {
	cfg    config.GatewayGuardReportConfig
	client *http.Client
	url    string
}

// NewGuardReporter 关闭或无 BaseURL 时返回 nil(调用方判 nil 即低成本 no-op)。
func NewGuardReporter(c config.GatewayGuardReportConfig) *GuardReporter {
	if !c.Enabled || strings.TrimSpace(c.BaseURL) == "" {
		return nil
	}
	t := c.TimeoutSeconds
	if t <= 0 {
		t = 2
	}
	return &GuardReporter{
		cfg:    c,
		client: &http.Client{Timeout: time.Duration(t) * time.Second},
		url:    strings.TrimRight(c.BaseURL, "/") + "/api/agent/audit",
	}
}

var (
	guardReporterOnce sync.Once
	guardReporterInst *GuardReporter
)

// GuardReporterFromConfig 进程内单例(配置稳定),复用单个 http.Client。
func GuardReporterFromConfig(c config.GatewayGuardReportConfig) *GuardReporter {
	guardReporterOnce.Do(func() { guardReporterInst = NewGuardReporter(c) })
	return guardReporterInst
}

// Enqueue 同步执行 POST(调用方已在后台 usage-record worker-pool 任务里,无需再起
// goroutine);用独立 context.Background()+client.Timeout 与请求生命周期解耦,任何错误
// 仅 WARN、不返回。务必在 RecordUsage/计费完成之后调用。
func (r *GuardReporter) Enqueue(s GuardSample) {
	if r == nil {
		return
	}
	if r.cfg.SampleRate > 0 && r.cfg.SampleRate < 100 && rand.IntN(100) >= r.cfg.SampleRate {
		return
	}
	defer func() {
		if rec := recover(); rec != nil {
			logger.L().With(zap.Any("recover", rec)).Warn("guard_report.panic")
		}
	}()
	body, err := json.Marshal(s)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, r.url, bytes.NewReader(body))
	if err != nil {
		logger.L().With(zap.Error(err)).Warn("guard_report.build_failed")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if r.cfg.AgentToken != "" {
		req.Header.Set("X-Agent-Token", r.cfg.AgentToken) // 切勿打印 token
	}
	resp, err := r.client.Do(req)
	if err != nil {
		logger.L().With(zap.Error(err)).Warn("guard_report.post_failed")
		return
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode/100 != 2 {
		logger.L().With(zap.Int("status", resp.StatusCode)).Warn("guard_report.non_2xx")
	}
}
