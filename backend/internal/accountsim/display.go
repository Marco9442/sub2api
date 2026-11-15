package accountsim

import (
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// displayCredentials 生成前端展示订阅等级等信息所需的非敏感凭据字段。
// 至少包含一个键，避免 lite 列表的 omitempty 把整段 credentials 丢掉。
func displayCredentials(a *Account) map[string]any {
	out := map[string]any{}
	if email := accountEmail(a); email != "" {
		out["email"] = email
	}
	switch a.Platform {
	case "anthropic":
		if a.Type == "oauth" || a.Type == "setup-token" {
			out["plan_type"] = a.Tier
		}
	case "openai":
		if a.Type == "oauth" {
			out["plan_type"] = a.Tier
			out["auth_mode"] = "chatgpt"
		}
	case "gemini":
		switch a.Type {
		case "oauth":
			out["oauth_type"] = "google_one"
			out["tier_id"] = geminiTierID(a.Tier)
		case "apikey":
			out["tier_id"] = "aistudio_paid"
			out["base_url"] = "https://generativelanguage.googleapis.com"
		}
	case "grok":
		out["subscription_tier"] = grokTierLabel(a.Tier)
	case "antigravity":
		// 等级在 extra.load_code_assist，凭据只保留邮箱。
	}
	if a.Type == "apikey" {
		if mode := strings.TrimSpace(a.Tier); mode != "" {
			out["account_mode"] = mode
		}
		if url := officialBaseURL(a.Platform, a.Tier); url != "" {
			out["base_url"] = url
		}
	}
	if len(out) == 0 {
		if a.Tier != "" {
			out["plan_type"] = a.Tier
		} else {
			out["account_mode"] = "payg"
		}
	}
	return out
}

// displayExtra 生成前端展示邮箱、额度快照等信息所需的 extra 字段。
func displayExtra(a *Account, now time.Time) map[string]any {
	out := map[string]any{}
	if email := accountEmail(a); email != "" {
		out["email"] = email
	}
	switch a.Platform {
	case "grok":
		label := grokTierLabel(a.Tier)
		out["grok_billing_snapshot"] = map[string]any{"plan": label}
		out["grok_usage_snapshot"] = map[string]any{"subscription_tier": label}
	case "antigravity":
		out["load_code_assist"] = map[string]any{
			"paidTier": map[string]any{"id": antigravityTierID(a.Tier)},
		}
	}
	if showsCNQuota(a) {
		fillCNQuota(out, a, now)
	}
	if showsCNBalance(a) {
		fillCNBalance(out, a)
	}
	if len(out) == 0 {
		out["display_name"] = a.Name
	}
	return out
}

func accountEmail(a *Account) string {
	if strings.Contains(a.Name, "@") {
		return a.Name
	}
	return ""
}

func grokTierLabel(tier string) string {
	if tier == "heavy" {
		return "SuperGrok Heavy"
	}
	return "SuperGrok"
}

func geminiTierID(tier string) string {
	if tier == "ultra" {
		return "google_ai_ultra"
	}
	return "google_ai_pro"
}

func antigravityTierID(tier string) string {
	if tier == "ultra" {
		return "g1-ultra-tier"
	}
	return "g1-pro-tier"
}

func officialBaseURL(platform, tier string) string {
	switch platform {
	case "kimi":
		if tier == "coding" {
			return "https://api.kimi.com/coding/v1"
		}
		return "https://api.moonshot.cn/v1"
	case "zhipu":
		if tier == "coding" {
			return "https://open.bigmodel.cn/api/coding/paas/v4"
		}
		return "https://open.bigmodel.cn/api/paas/v4"
	case "deepseek":
		return "https://api.deepseek.com"
	case "minimax":
		return "https://api.minimaxi.com/v1"
	case "opencode_go":
		if tier == "zen" {
			return "https://opencode.ai/zen/v1"
		}
		return "https://opencode.ai/zen/go/v1"
	case "command_code":
		return "https://api.commandcode.ai/provider/v1"
	case "cline":
		return "https://api.cline.bot/api/v1"
	default:
		return ""
	}
}

func showsCNQuota(a *Account) bool {
	if a.Type != "apikey" {
		return false
	}
	switch a.Platform {
	case "kimi", "zhipu", "minimax":
		return a.Tier == "coding"
	case "opencode_go":
		return a.Tier != "zen"
	case "command_code", "cline":
		return true
	default:
		return false
	}
}

func showsCNBalance(a *Account) bool {
	if a.Type != "apikey" {
		return false
	}
	switch a.Platform {
	case "kimi", "deepseek":
		return a.Tier != "coding"
	case "command_code", "cline":
		return true
	default:
		return false
	}
}

func fillCNQuota(out map[string]any, a *Account, now time.Time) {
	prof := profileOf(a.Platform, a.Type, a.Tier)
	p := a.Platform
	out[p+"_5h_used_percent"] = utilPercent(a.Window5h.Cost, prof.Cap5h)
	out[p+"_weekly_used_percent"] = utilPercent(a.Window7d.Cost, prof.Cap7d)
	out[p+"_monthly_used_percent"] = utilPercent(monthCost(a), monthlyCap(prof))
	out[p+"_5h_reset_at"] = resetAt(a.Window5h.Start, window5h, now).Format(time.RFC3339)
	out[p+"_weekly_reset_at"] = resetAt(a.Window7d.Start, window7d, now).Format(time.RFC3339)
	out[p+"_monthly_reset_at"] = monthReset(now).Format(time.RFC3339)
	out[p+"_usage_updated_at"] = now.Format(time.RFC3339)
}

func fillCNBalance(out map[string]any, a *Account) {
	remain := remainingBalance(a)
	p := a.Platform
	if p == "deepseek" {
		out[p+"_balances"] = []map[string]any{
			{"currency": "CNY", "balance": remain},
			{"currency": "USD", "balance": math.Round(remain*0.14*100) / 100},
		}
		return
	}
	out[p+"_balances"] = []map[string]any{{"currency": "CNY", "balance": remain}}
}

func remainingBalance(a *Account) float64 {
	prof := profileOf(a.Platform, a.Type, a.Tier)
	remain := prof.Balance - a.TotalCost
	if remain < 0 {
		remain = 0
	}
	return math.Round(remain*100) / 100
}

func monthCost(a *Account) float64 {
	var cost float64
	for _, day := range a.Days {
		cost += day.Cost
	}
	return cost
}

func monthlyCap(prof profile) float64 {
	if prof.Cap7d > 0 {
		return prof.Cap7d * 4
	}
	if prof.CapDaily > 0 {
		return prof.CapDaily * 30
	}
	if prof.Cap5h > 0 {
		return prof.Cap5h * 24
	}
	return 0
}

func monthReset(now time.Time) time.Time {
	local := now.In(cst)
	return time.Date(local.Year(), local.Month()+1, 1, 0, 0, 0, 0, cst)
}

func utilPercent(cost, cap float64) float64 {
	if cap <= 0 || cost <= 0 {
		return 0
	}
	p := cost / cap * 100
	if p > 100 {
		p = 100
	}
	return math.Round(p*10) / 10
}

func resetAt(start time.Time, dur time.Duration, now time.Time) *time.Time {
	end := now.Add(dur)
	if !start.IsZero() {
		end = start.Add(dur)
		if !end.After(now) {
			end = now.Add(dur)
		}
	}
	return &end
}

func windowStatsOf(w Window, rate float64) *service.WindowStats {
	if rate <= 0 {
		rate = 1
	}
	return &service.WindowStats{
		Requests:     w.Requests,
		Tokens:       w.Tokens,
		Cost:         w.Cost * rate,
		StandardCost: w.Cost,
		UserCost:     w.Cost * rate,
	}
}

func windowProgress(w Window, dur time.Duration, cap float64, now time.Time, rate float64) *service.UsageProgress {
	reset := resetAt(w.Start, dur, now)
	remaining := int(reset.Sub(now).Seconds())
	if remaining < 0 {
		remaining = 0
	}
	return &service.UsageProgress{
		Utilization:      utilPercent(w.Cost, cap),
		ResetsAt:         reset,
		RemainingSeconds: remaining,
		WindowStats:      windowStatsOf(w, rate),
	}
}

// usageInfo 用本地窗口生成与真实用量接口相同的结构，不访问上游。
func usageInfo(a *Account, now time.Time) *service.UsageInfo {
	prof := profileOf(a.Platform, a.Type, a.Tier)
	rate := a.RateMultiplier
	info := &service.UsageInfo{}
	five := windowProgress(a.Window5h, window5h, prof.Cap5h, now, rate)
	seven := windowProgress(a.Window7d, window7d, prof.Cap7d, now, rate)
	switch {
	case a.Platform == "gemini":
		info.GeminiSharedDaily = dayProgress(a, now, prof.CapDaily, rate)
	case a.Platform == "antigravity":
		info.FiveHour = five
		info.AntigravityQuotaGroups = []*service.AntigravityQuotaGroup{{
			Key:  "gemini",
			Name: "Gemini",
			Windows: []*service.AntigravityQuotaWindow{
				{Window: "5h", Utilization: utilPercent(a.Window5h.Cost, prof.Cap5h), ResetsAt: resetAt(a.Window5h.Start, window5h, now)},
				{Window: "weekly", Utilization: utilPercent(a.Window7d.Cost, prof.Cap7d), ResetsAt: resetAt(a.Window7d.Start, window7d, now)},
			},
		}}
	case a.Platform == "grok":
		pct := utilPercent(a.Window7d.Cost, prof.Cap7d)
		info.GrokBilling = &xai.BillingSummary{
			PeriodType:   "weekly",
			UsagePercent: &pct,
			PeriodEnd:    resetAt(a.Window7d.Start, window7d, now).Format(time.RFC3339),
			Plan:         grokTierLabel(a.Tier),
			StatusCode:   200,
		}
		info.GrokLocalUsage7d = windowStatsOf(a.Window7d, rate)
		info.GrokQuotaSnapshotState = "observed"
	default:
		info.FiveHour = five
		info.SevenDay = seven
	}
	return info
}

func dayProgress(a *Account, now time.Time, cap, rate float64) *service.UsageProgress {
	day := todayUsage(a, now)
	reset := monthReset(now)
	if cap > 0 {
		next := now.In(cst)
		reset = time.Date(next.Year(), next.Month(), next.Day()+1, 0, 0, 0, 0, cst)
	}
	remaining := int(reset.Sub(now).Seconds())
	if remaining < 0 {
		remaining = 0
	}
	return &service.UsageProgress{
		Utilization:      utilPercent(day.Cost, cap),
		ResetsAt:         &reset,
		RemainingSeconds: remaining,
		WindowStats: &service.WindowStats{
			Requests:     day.Requests,
			Tokens:       day.Tokens(),
			Cost:         day.Cost * nonZero(rate),
			StandardCost: day.Cost,
			UserCost:     day.ActualCost,
		},
	}
}

func todayStats(a *Account, now time.Time) service.WindowStats {
	day := todayUsage(a, now)
	rate := nonZero(a.RateMultiplier)
	return service.WindowStats{
		Requests:     day.Requests,
		Tokens:       day.Tokens(),
		Cost:         day.Cost * rate,
		StandardCost: day.Cost,
		UserCost:     day.ActualCost,
	}
}

func todayUsage(a *Account, now time.Time) DayUsage {
	date := now.In(cst).Format(time.DateOnly)
	for i := range a.Days {
		if a.Days[i].Date == date {
			return a.Days[i]
		}
	}
	return DayUsage{}
}

func nonZero(rate float64) float64 {
	if rate <= 0 {
		return 1
	}
	return rate
}
