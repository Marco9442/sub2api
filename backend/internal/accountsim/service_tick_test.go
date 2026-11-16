package accountsim

import (
	"context"
	"testing"
	"time"
)

func TestTickReplacesMemoryOnlyAfterSave(t *testing.T) {
	store := &memStore{state: fixtureState(), cfg: DefaultConfig()}
	original := store.state
	account := original.Accounts[6601]
	dayCost := account.Days[0].ActualCost
	now := fixtureNow
	svc := NewService(store, func() time.Time { return now })

	if err := svc.Tick(context.Background()); err != nil || store.saves != 0 || svc.state != original {
		t.Fatalf("水位未前进不应保存: err=%v saves=%d", err, store.saves)
	}

	now = fixtureNow.Add(Step)
	if err := svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.saves != 1 || store.state == original || svc.state != store.state || !store.state.EvolvedUntil.Equal(now) {
		t.Fatalf("保存成功后才应换成新状态: saves=%d until=%v", store.saves, store.state.EvolvedUntil)
	}
	if !original.EvolvedUntil.Equal(fixtureNow) || original.Accounts[6601] != account {
		t.Fatalf("原状态不应被演进: until=%v", original.EvolvedUntil)
	}
	if dayCost == nil || *dayCost != 2 || account.Days[0].ActualCost != dayCost {
		t.Fatal("演进副本不应改原账号的实际费用")
	}
	if store.state.Accounts[6601] == account {
		t.Fatal("新状态应使用账号副本")
	}
}

func TestTickKeepsMemoryWhenSaveFails(t *testing.T) {
	store := &failStore{memStore: memStore{state: fixtureState(), cfg: DefaultConfig()}}
	original := store.state
	now := fixtureNow.Add(Step)
	svc := NewService(store, func() time.Time { return now })

	if err := svc.Tick(context.Background()); err == nil {
		t.Fatal("保存失败应返回错误")
	}
	if store.saves != 1 || svc.state != original || !original.EvolvedUntil.Equal(fixtureNow) {
		t.Fatalf("失败后应保持原水位: saves=%d until=%v", store.saves, original.EvolvedUntil)
	}
	if err := svc.Tick(context.Background()); err == nil || store.saves != 2 || svc.state != original || !original.EvolvedUntil.Equal(fixtureNow) {
		t.Fatalf("同一时段重试仍应保存且不推进水位: saves=%d err=%v until=%v", store.saves, err, original.EvolvedUntil)
	}
}
