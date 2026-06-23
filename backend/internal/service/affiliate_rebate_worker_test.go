package service

import (
	"testing"
)

func TestAggregateAffOutbox_SumsByPair(t *testing.T) {
	rows := []affOutboxRow{
		{ID: 1, InviterID: 10, InviteeUserID: 100, Amount: 0.5},
		{ID: 2, InviterID: 10, InviteeUserID: 100, Amount: 0.3},
		{ID: 3, InviterID: 10, InviteeUserID: 101, Amount: 1.0},
		{ID: 4, InviterID: 11, InviteeUserID: 100, Amount: 0.2},
		{ID: 5, InviterID: 10, InviteeUserID: 100, Amount: 0.1},
	}
	groups := aggregateAffOutbox(rows)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	// 构造期望 map 以避免顺序依赖（map 迭代顺序未定义）
	got := map[[2]int64]float64{}
	for _, g := range groups {
		got[[2]int64{g.InviterID, g.InviteeUserID}] = g.Amount
	}
	cases := []struct {
		inviter, invitee int64
		want             float64
	}{
		{10, 100, 0.9}, // 0.5 + 0.3 + 0.1
		{10, 101, 1.0},
		{11, 100, 0.2},
	}
	for _, c := range cases {
		v := got[[2]int64{c.inviter, c.invitee}]
		if v < c.want-1e-9 || v > c.want+1e-9 {
			t.Errorf("inviter=%d invitee=%d: want amount=%v, got %v", c.inviter, c.invitee, c.want, v)
		}
	}
}

func TestAggregateAffOutbox_Empty(t *testing.T) {
	groups := aggregateAffOutbox(nil)
	if len(groups) != 0 {
		t.Errorf("expected empty groups for nil input, got %d", len(groups))
	}
	groups = aggregateAffOutbox([]affOutboxRow{})
	if len(groups) != 0 {
		t.Errorf("expected empty groups for empty input, got %d", len(groups))
	}
}
