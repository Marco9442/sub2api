package accountsim

import "testing"

func TestNormalizeKeepsDefaults(t *testing.T) {
	cfg := DefaultConfig()
	got := cfg.Normalize()
	if got.DailyNewMin != cfg.DailyNewMin || got.LifetimeDaysMax != cfg.LifetimeDaysMax || len(got.TypeWeight) != len(cfg.TypeWeight) {
		t.Fatalf("默认配置经过 Normalize 后改变: %+v", got)
	}
}

func TestNormalizeRepairsInvalidValues(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DailyNewMin, cfg.DailyNewMax = 200, -5
	cfg.LifetimeDaysMin, cfg.LifetimeDaysMax = 0, 0
	cfg.RateLimitPerMille = 5000
	cfg.AuthErrorRecoverPercent = 150
	cfg.AccountsPerGroup = 0
	cfg.GroupNamePattern = " "
	cfg.UsageLevel = -1
	cfg.TypeWeight["anthropic/upstream"] = 9
	cfg.PlatformWeight["unknown"] = 5
	cfg.HourlyWeight[3] = -2

	got := cfg.Normalize()
	if got.DailyNewMin < 0 || got.DailyNewMax < got.DailyNewMin {
		t.Fatalf("每日新增范围无效: %d~%d", got.DailyNewMin, got.DailyNewMax)
	}
	if got.LifetimeDaysMin < 1 || got.LifetimeDaysMax < got.LifetimeDaysMin {
		t.Fatalf("寿命范围无效: %d~%d", got.LifetimeDaysMin, got.LifetimeDaysMax)
	}
	if got.RateLimitPerMille > 1000 || got.AuthErrorRecoverPercent > 100 {
		t.Fatalf("比例越界: %d %d", got.RateLimitPerMille, got.AuthErrorRecoverPercent)
	}
	if got.AccountsPerGroup < 1 || got.GroupNamePattern == " " || got.UsageLevel < 0 || got.HourlyWeight[3] < 0 {
		t.Fatalf("未修正: %+v", got)
	}
	if _, ok := got.TypeWeight["anthropic/upstream"]; ok {
		t.Fatal("保留了平台不支持的认证类型")
	}
	if _, ok := got.PlatformWeight["unknown"]; ok {
		t.Fatal("保留了未知平台")
	}
}

func TestNormalizeFillsMissingMaps(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PlatformWeight, cfg.TypeWeight, cfg.TierWeight = nil, nil, nil
	got := cfg.Normalize()
	if len(got.PlatformWeight) == 0 || len(got.TypeWeight) == 0 || len(got.TierWeight) == 0 {
		t.Fatal("缺失的权重表没有用默认值补齐")
	}
}
