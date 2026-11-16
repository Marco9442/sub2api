package accountsim

import (
	"testing"
	"time"
)

func TestIDWaterlineStable(t *testing.T) {
	at := time.Unix(1794713258+10*86400, 0)
	cfg := DefaultConfig()

	first := IDWaterline(at, cfg)
	second := IDWaterline(at, cfg)
	if first != second {
		t.Fatalf("同一时刻重复计算得到不同水位: %d != %d", first, second)
	}
	if first < 6511 {
		t.Fatalf("水位低于基准 ID: %d", first)
	}
}

func TestIDWaterlineMonotonic(t *testing.T) {
	cfg := DefaultConfig()
	base := time.Unix(1794713258, 0)
	previous := IDWaterline(base, cfg)
	for day := 1; day <= 60; day++ {
		current := IDWaterline(base.AddDate(0, 0, day), cfg)
		if current < previous {
			t.Fatalf("第 %d 天水位倒退: %d < %d", day, current, previous)
		}
		previous = current
	}
}

func TestIDWaterlineDailyGrowthInRange(t *testing.T) {
	cfg := DefaultConfig()
	first := dayIndex(time.Unix(1794713258, 0))
	for day := first; day < first+40; day++ {
		growth := IDWaterline(dayStart(day+1), cfg) - IDWaterline(dayStart(day), cfg)
		if growth < cfg.DailyNewMin || growth > cfg.DailyNewMax {
			t.Fatalf("第 %d 天新增 %d 超出配置范围 %d~%d", day-first, growth, cfg.DailyNewMin, cfg.DailyNewMax)
		}
	}
}

func TestIDWaterlineRespectsFloor(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IDFloor = 9000
	at := time.Unix(1794713258, 0)
	if got := IDWaterline(at, cfg); got < 9000 {
		t.Fatalf("水位低于下限: %d", got)
	}
}

func TestIDWaterlineAnchor(t *testing.T) {
	cfg := DefaultConfig()
	at := time.Unix(1794713258, 0)
	if got := IDWaterline(at, cfg); got != 6511 {
		t.Fatalf("基准时刻水位应为 6511，实际 %d", got)
	}
}
