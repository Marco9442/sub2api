package accountsim

import (
	"testing"
	"time"
)

func TestEvolveCreatesAccountsUpToWaterline(t *testing.T) {
	state := NewState()
	cfg := DefaultConfig()
	at := time.Unix(1794713258+3*86400+10*3600, 0)

	Evolve(state, at, cfg)

	want := IDWaterline(at, cfg)
	if got := state.LastID; got != want {
		t.Fatalf("演进后最大 ID 应为 %d，实际 %d", want, got)
	}
}

func TestEvolveIdempotent(t *testing.T) {
	cfg := DefaultConfig()
	at := time.Unix(1794713258+5*86400, 0)

	first := NewState()
	Evolve(first, at, cfg)
	second := NewState()
	Evolve(second, at, cfg)

	if first.LastID != second.LastID {
		t.Fatalf("同一时刻两次演进结果不同: %d != %d", first.LastID, second.LastID)
	}
	if len(first.Accounts) != len(second.Accounts) {
		t.Fatalf("账号数量不同: %d != %d", len(first.Accounts), len(second.Accounts))
	}
}

func TestEvolveDoesNotRegress(t *testing.T) {
	state := NewState()
	cfg := DefaultConfig()
	base := time.Unix(1794713258, 0)

	Evolve(state, base.Add(24*time.Hour), cfg)
	mid := state.LastID
	Evolve(state, base.Add(48*time.Hour), cfg)
	if state.LastID < mid {
		t.Fatalf("再次演进后 ID 倒退: %d < %d", state.LastID, mid)
	}
}

func TestEvolveTotalsMonotonic(t *testing.T) {
	state := NewState()
	cfg := DefaultConfig()
	base := time.Unix(1794713258+2*86400, 0)

	Evolve(state, base, cfg)
	before := map[int64]float64{}
	for id, account := range state.Accounts {
		before[id] = account.TotalCost
	}
	Evolve(state, base.Add(6*time.Hour), cfg)
	for id, cost := range before {
		account, ok := state.Accounts[id]
		if ok && account.TotalCost < cost {
			t.Fatalf("账号 %d 累计费用回退: %v < %v", id, account.TotalCost, cost)
		}
	}
}

func TestEvolveRetiresExpiredAccounts(t *testing.T) {
	state := NewState()
	cfg := DefaultConfig()
	cfg.LifetimeDaysMin = 1
	cfg.LifetimeDaysMax = 1
	cfg.LingerDaysMin = 1
	cfg.LingerDaysMax = 1
	base := time.Unix(1794713258, 0)

	Evolve(state, base.Add(24*time.Hour), cfg)
	created := len(state.Accounts)
	if created == 0 {
		t.Fatal("没有生成账号")
	}
	Evolve(state, base.Add(5*24*time.Hour), cfg)
	for _, account := range state.Accounts {
		if account.CreatedAt.Before(base.Add(24 * time.Hour)) {
			t.Fatalf("寿命 1 天、滞留 1 天的账号在第 4 天仍存在: %d", account.ID)
		}
	}
}

func TestEvolveSkipsManualAccounts(t *testing.T) {
	state := NewState()
	cfg := DefaultConfig()
	at := time.Unix(1794713258+2*86400, 0)
	Evolve(state, at, cfg)

	var target *Account
	for _, account := range state.Accounts {
		if account.RetiredAt == nil {
			target = account
			break
		}
	}
	target.Status = StatusInactive
	target.Name = "手动改过"
	target.Pin("status", "name")
	cost := target.TotalCost

	Evolve(state, at.Add(20*24*time.Hour), cfg)
	kept, ok := state.Accounts[target.ID]
	if !ok {
		t.Fatal("手动接管的账号被自动淘汰")
	}
	if kept.Status != StatusInactive || kept.Name != "手动改过" || kept.TotalCost != cost {
		t.Fatalf("手动修改被自动运营覆盖: status=%s name=%s cost=%v", kept.Status, kept.Name, kept.TotalCost)
	}
}

func TestEvolveCombinationsSupported(t *testing.T) {
	state := NewState()
	cfg := DefaultConfig()
	Evolve(state, time.Unix(1794713258+2*86400, 0), cfg)
	for _, account := range state.Accounts {
		if !Supported(account.Platform, account.Type, account.Tier) {
			t.Fatalf("生成了平台不支持的组合: %s/%s/%s", account.Platform, account.Type, account.Tier)
		}
	}
}

func TestEvolveStatsNotBeforeCreation(t *testing.T) {
	state := NewState()
	cfg := DefaultConfig()
	Evolve(state, time.Unix(1794713258+3*86400, 0), cfg)
	for _, account := range state.Accounts {
		created := account.CreatedAt.In(cst).Format(time.DateOnly)
		for _, day := range account.Days {
			if day.Date < created {
				t.Fatalf("账号 %d 在创建前就有用量: %s < %s", account.ID, day.Date, created)
			}
		}
		if account.LastUsedAt != nil && account.LastUsedAt.Before(account.CreatedAt) {
			t.Fatalf("账号 %d 最近使用早于创建", account.ID)
		}
	}
}

func TestEvolveBackfillsThirtyDays(t *testing.T) {
	state := NewState()
	cfg := DefaultConfig()
	at := time.Unix(1794713258, 0)
	Evolve(state, at, cfg)

	oldest := at
	for _, account := range state.Accounts {
		if account.CreatedAt.Before(oldest) {
			oldest = account.CreatedAt
		}
	}
	if at.Sub(oldest) < 10*24*time.Hour {
		t.Fatalf("首次使用应回填历史，最早账号仅在 %v 前", at.Sub(oldest))
	}
	if at.Sub(oldest) > 30*24*time.Hour {
		t.Fatalf("回填超过 30 天: %v", at.Sub(oldest))
	}
	if state.LastID != 6511 {
		t.Fatalf("基准时刻回填后最大 ID 应为 6511，实际 %d", state.LastID)
	}
}

func TestEvolveClearedPoolIsNotFirstUse(t *testing.T) {
	state := NewState()
	cfg := DefaultConfig()
	at := time.Unix(1794713258, 0)
	Evolve(state, at, cfg)
	last := state.LastID
	state.Accounts = map[int64]*Account{}

	Evolve(state, at.Add(time.Hour), cfg)
	for _, account := range state.Accounts {
		if account.ID <= last {
			t.Fatalf("手动清空后又回填了旧账号 %d", account.ID)
		}
	}
}

func TestEvolveRecoversAfterDowntime(t *testing.T) {
	cfg := DefaultConfig()
	base := time.Unix(1794713258, 0)
	end := base.Add(36 * time.Hour)

	resumed := NewState()
	Evolve(resumed, base, cfg)
	Evolve(resumed, end, cfg)
	if want := IDWaterline(end, cfg); resumed.LastID != want {
		t.Fatalf("中断后补齐的最大 ID 应为 %d，实际 %d", want, resumed.LastID)
	}
	ids := map[int64]bool{}
	for id := range resumed.Accounts {
		if ids[id] {
			t.Fatalf("重复生成账号 %d", id)
		}
		ids[id] = true
	}
}

func TestEvolveWindowsReset(t *testing.T) {
	state := NewState()
	cfg := DefaultConfig()
	at := time.Unix(1794713258+86400, 0)
	Evolve(state, at, cfg)
	for _, account := range state.Accounts {
		if !account.Window5h.Start.IsZero() && at.Sub(account.Window5h.Start) >= 5*time.Hour+Step {
			t.Fatalf("账号 %d 的 5 小时窗口未重置: start=%v", account.ID, account.Window5h.Start)
		}
	}
}

func TestEvolveGroupsFollowPool(t *testing.T) {
	state := NewState()
	cfg := DefaultConfig()
	Evolve(state, time.Unix(1794713258, 0), cfg)
	if len(state.Groups) == 0 || len(state.Proxies) == 0 {
		t.Fatal("没有生成分组或代理")
	}
	groups := map[int64]Group{}
	for _, group := range state.Groups {
		groups[group.ID] = group
	}
	for _, account := range state.Accounts {
		for _, id := range account.GroupIDs {
			group, ok := groups[id]
			if !ok {
				t.Fatalf("账号 %d 指向不存在的分组 %d", account.ID, id)
			}
			if group.Platform != account.Platform {
				t.Fatalf("账号 %d 的平台 %s 与分组平台 %s 不一致", account.ID, account.Platform, group.Platform)
			}
		}
	}
}
