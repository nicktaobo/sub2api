// Package service - FX rate service.
//
// 提供 CNY ↔ USD 汇率，用于"模型定价"展示页计算"等效美元价"。
//
// 工作方式：
//   - 启动时同步拉一次外部公开汇率 API（带超时）
//   - 之后每小时刷一次（goroutine）
//   - 外部不可达时降级到 fallbackCNYPerUSD 常量（6.8）
//
// 仅用于"展示"，不参与计费——计费仍按 USD 进行。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	// fallbackCNYPerUSD 当外部 API 不可达时的默认值；按当前汇率水平大致合理。
	fallbackCNYPerUSD = 6.8

	fxRateRefreshInterval = time.Hour
	fxRateFetchTimeout    = 5 * time.Second

	// 公开汇率 API（无需 key）。多 source 用 fallback：
	//   - open.er-api.com 是 exchangerate-api.com 的免费镜像
	fxRateURL = "https://open.er-api.com/v6/latest/USD"
)

// FXRateService 维护 CNY/USD 汇率快照（内存）。
type FXRateService struct {
	rate        atomic.Value // float64
	lastUpdated atomic.Value // time.Time

	httpClient *http.Client
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// NewFXRateService 创建汇率服务，启动时不立即拉外部 API（Initialize 时拉）。
func NewFXRateService() *FXRateService {
	s := &FXRateService{
		httpClient: &http.Client{Timeout: fxRateFetchTimeout},
		stopCh:     make(chan struct{}),
	}
	s.rate.Store(float64(fallbackCNYPerUSD))
	s.lastUpdated.Store(time.Time{})
	return s
}

// Initialize 同步拉一次（失败用默认值），并启动后台 goroutine 定时刷新。
func (s *FXRateService) Initialize(ctx context.Context) {
	if err := s.refresh(ctx); err != nil {
		logger.LegacyPrintf("service.fx_rate", "[FXRate] initial fetch failed (%v), using fallback %.4f",
			err, fallbackCNYPerUSD)
	}
	s.wg.Add(1)
	go s.loop()
}

// Stop 停止后台刷新。
func (s *FXRateService) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

// CNYPerUSD 返回当前 ¥ / $ 汇率（6.8 表示 ¥6.8 = $1）。
func (s *FXRateService) CNYPerUSD() float64 {
	v, _ := s.rate.Load().(float64)
	if v <= 0 {
		return fallbackCNYPerUSD
	}
	return v
}

// LastUpdated 返回上次刷新成功的时间（零值表示从未成功刷新过，仍在用 fallback）。
func (s *FXRateService) LastUpdated() time.Time {
	v, _ := s.lastUpdated.Load().(time.Time)
	return v
}

func (s *FXRateService) loop() {
	defer s.wg.Done()
	ticker := time.NewTicker(fxRateRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), fxRateFetchTimeout)
			if err := s.refresh(ctx); err != nil {
				logger.LegacyPrintf("service.fx_rate", "[FXRate] refresh failed: %v", err)
			}
			cancel()
		}
	}
}

type erAPIResponse struct {
	Result string             `json:"result"`
	Rates  map[string]float64 `json:"rates"`
}

func (s *FXRateService) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fxRateURL, nil)
	if err != nil {
		return err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return errors.New("fx rate api returned non-200 status")
	}

	var body erAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return err
	}
	if body.Result != "success" {
		return errors.New("fx rate api returned non-success result")
	}
	cny, ok := body.Rates["CNY"]
	if !ok || cny <= 0 {
		return errors.New("fx rate api response missing CNY")
	}
	s.rate.Store(cny)
	s.lastUpdated.Store(time.Now())
	return nil
}
