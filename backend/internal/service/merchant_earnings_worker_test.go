// MERCHANT-SYSTEM v1.0
package service

import (
	"strings"
	"testing"
)

func TestSplitOutboxBySource(t *testing.T) {
	rows := []outboxRow{
		{ID: 1, MerchantID: 100, Source: MerchantSourceUserMarkupShare, Amount: 0.5},
		{ID: 2, MerchantID: 100, Source: MerchantSourceUserMarkupShare, Amount: 1.5},
		{ID: 3, MerchantID: 200, Source: MerchantSourceUserRechargeShare, Amount: 10},
		{ID: 4, MerchantID: 100, Source: MerchantSourceSelfRecharge, Amount: 100, RefID: 50},
		{ID: 5, MerchantID: 100, Source: MerchantSourceSelfRecharge, Amount: 200, RefID: 51},
	}

	agg, perRow := splitOutboxBySource(rows)
	if len(agg) != 2 {
		t.Fatalf("expected 2 aggregated groups, got %d", len(agg))
	}
	if len(perRow) != 2 {
		t.Fatalf("expected 2 per-row entries (self_recharge), got %d", len(perRow))
	}

	for _, g := range agg {
		switch g.Source {
		case MerchantSourceUserMarkupShare:
			if g.MerchantID != 100 || g.Sum != 2.0 || len(g.Rows) != 2 {
				t.Errorf("markup_share group wrong: %+v", g)
			}
		case MerchantSourceUserRechargeShare:
			if g.MerchantID != 200 || g.Sum != 10 || len(g.Rows) != 1 {
				t.Errorf("recharge_share group wrong: %+v", g)
			}
		default:
			t.Errorf("unexpected source in agg: %s", g.Source)
		}
	}
	for _, r := range perRow {
		if r.Source != MerchantSourceSelfRecharge {
			t.Errorf("perRow entry should be self_recharge, got %s", r.Source)
		}
	}
}

func TestDeterministicOutboxBatchKey(t *testing.T) {
	// 两次相同的 outbox 行集合必须生成相同的 key（重启重试场景）
	g1 := outboxGroup{
		MerchantID: 100,
		Source:     MerchantSourceUserMarkupShare,
		Rows: []outboxRow{
			{ID: 1}, {ID: 5}, {ID: 3},
		},
	}
	g2 := outboxGroup{
		MerchantID: 100,
		Source:     MerchantSourceUserMarkupShare,
		Rows: []outboxRow{
			// 顺序不同
			{ID: 5}, {ID: 1}, {ID: 3},
		},
	}
	k1 := deterministicOutboxBatchKey(g1)
	k2 := deterministicOutboxBatchKey(g2)
	if k1 != k2 {
		t.Errorf("expected deterministic key: %q vs %q", k1, k2)
	}
	// 改变 count → 不同 key
	g3 := outboxGroup{
		MerchantID: 100,
		Source:     MerchantSourceUserMarkupShare,
		Rows:       []outboxRow{{ID: 1}, {ID: 5}, {ID: 3}, {ID: 4}},
	}
	if deterministicOutboxBatchKey(g3) == k1 {
		t.Errorf("different count should produce different keys")
	}
	if !strings.Contains(k1, "outbox_batch:100:user_markup_share:") {
		t.Errorf("key should contain merchant id and source, got %s", k1)
	}
}

func TestIsMerchantBlockingError(t *testing.T) {
	if IsMerchantBlockingError(nil) {
		t.Error("nil should not be blocking")
	}
	if IsMerchantBlockingError(ErrUserNotFound) {
		t.Error("plain error should not be blocking")
	}
	be := &MerchantBlockingError{Stage: "intent_write", Err: ErrUserNotFound}
	if !IsMerchantBlockingError(be) {
		t.Error("MerchantBlockingError should be detected")
	}
	wrapped := wrapErrorf("audit failed: %w", be)
	if !IsMerchantBlockingError(wrapped) {
		t.Error("errors.As must traverse fmt.Errorf %w wrap")
	}
}

// helper for nested wrap
func wrapErrorf(format string, args ...any) error {
	return &wrappedErr{format: format, args: args}
}

type wrappedErr struct {
	format string
	args   []any
}

func (w *wrappedErr) Error() string { return w.format }
func (w *wrappedErr) Unwrap() error {
	for _, a := range w.args {
		if err, ok := a.(error); ok {
			return err
		}
	}
	return nil
}
