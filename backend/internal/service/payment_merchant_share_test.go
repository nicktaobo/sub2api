// MERCHANT-SYSTEM v1.0
// Phase 3.10 错误路径测试（RFC v1.13 P2-3）：
//   1. flag 关闭 → 短路 nil（hook 不调任何依赖）
//   2. errors.As 嵌套 wrap 仍能识别 MerchantBlockingError
//   3. MerchantBlockingError 自身的 Error/Unwrap 正确

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// 测试 1：merchant flag 关闭 → 直接 nil 短路（不会去调 repo / strict audit）
func TestApplyMerchantHookForOrder_FlagOff(t *testing.T) {
	ps := &PaymentService{
		merchantCfg: config.MerchantConfig{Enabled: false},
	}
	// 即使传 nil order/repo 也不会崩——因为 flag 关闭时直接 return
	if err := ps.applyMerchantSelfRechargeForOrder(context.Background(), nil); err != nil {
		t.Fatalf("expected nil with flag off, got %v", err)
	}
	if err := ps.applyMerchantHookForOrder(context.Background(), nil); err != nil {
		t.Fatalf("expected nil with flag off (router), got %v", err)
	}
}

// 测试 2：errors.As 必须支持 fmt.Errorf %w 嵌套包装
func TestIsMerchantBlockingError_NestedWrap(t *testing.T) {
	be := &MerchantBlockingError{Stage: "intent_write", Err: errors.New("audit boom")}
	wrapped := fmt.Errorf("payment hook: %w", be)
	doubleWrapped := fmt.Errorf("execute fulfillment: %w", wrapped)
	if !IsMerchantBlockingError(doubleWrapped) {
		t.Errorf("errors.As must traverse nested fmt.Errorf %%w wrap")
	}
	plain := errors.New("plain error")
	if IsMerchantBlockingError(plain) {
		t.Errorf("plain error should not be detected as blocking")
	}
	if IsMerchantBlockingError(nil) {
		t.Errorf("nil should not be blocking")
	}
}

// 测试 3：MerchantBlockingError 自身的 Error/Unwrap 行为
func TestMerchantBlockingError_Methods(t *testing.T) {
	cause := errors.New("inner cause")
	be := &MerchantBlockingError{Stage: "intent_write", Err: cause}
	if be.Error() != "inner cause" {
		t.Errorf("Error() should delegate to wrapped error, got %q", be.Error())
	}
	if !errors.Is(be, cause) {
		t.Errorf("errors.Is should match wrapped cause via Unwrap")
	}
}
