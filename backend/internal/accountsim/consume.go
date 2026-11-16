package accountsim

import (
	"math"
	"time"
)

const (
	window5h = 5 * time.Hour
	window7d = 7 * 24 * time.Hour
	// maxModelsPerStep 限制每段时间抽样的模型数，用量按抽样请求放大，计算量与请求数无关。
	maxModelsPerStep = 3
)

// resetWindows 让额度窗口按真实周期重置：窗口从第一次使用开始计时，到期清零。
func resetWindows(account *Account, t time.Time) {
	if !account.Window5h.Start.IsZero() && !t.Before(account.Window5h.Start.Add(window5h)) {
		account.Window5h = Window{}
	}
	if !account.Window7d.Start.IsZero() && !t.Before(account.Window7d.Start.Add(window7d)) {
		account.Window7d = Window{}
	}
}

// consume 生成 [t, t+Step) 的用量，累计到窗口、日统计和总量。额度用尽时限流到窗口重置。
func consume(account *Account, t time.Time, r *rng, cfg Config) {
	prof := profileOf(account.Platform, account.Type, account.Tier)
	target := prof.HourlyCost * account.Intensity * cfg.UsageLevel * hourFactor(t, cfg) *
		(0.5 + r.float()) * float64(Step) / float64(time.Hour)
	if target <= 0 {
		return
	}
	day := dayUsage(account, t)
	var requests, tokens int64
	var cost float64
	for i := 0; i < maxModelsPerStep && cost < target; i++ {
		model := pickModel(r, account.Platform)
		in, out, cacheRead, cacheWrite, each := requestShape(r, model, prof.CacheHeavy)
		n := int64(math.Ceil((target - cost) / each / float64(maxModelsPerStep-i)))
		if n < 1 {
			n = 1
		}
		requests += n
		tokens += n * (in + out + cacheRead + cacheWrite)
		cost += float64(n) * each
		day.Requests += n
		day.InputTokens += n * in
		day.OutputTokens += n * out
		day.CacheReadTokens += n * cacheRead
		day.CacheCreationTokens += n * cacheWrite
		if day.Models == nil {
			day.Models = map[string]int64{}
		}
		day.Models[model.Name] += n
	}
	actual := cost * account.RateMultiplier
	day.Cost += cost
	day.ActualCost += actual
	account.TotalRequests += requests
	account.TotalTokens += tokens
	account.TotalCost += cost
	account.TotalActualCost += actual

	usedAt := t.Add(time.Duration(r.intn(int(Step/time.Second))) * time.Second)
	account.LastUsedAt = &usedAt
	addWindow(&account.Window5h, t, requests, tokens, cost, actual)
	addWindow(&account.Window7d, t, requests, tokens, cost, actual)
	limitOnCap(account, t, prof)
}

func addWindow(w *Window, t time.Time, requests, tokens int64, cost, actual float64) {
	if w.Start.IsZero() {
		w.Start = t
	}
	w.Requests += requests
	w.Tokens += tokens
	w.Cost += cost
	w.ActualCost += actual
}

func limitOnCap(account *Account, t time.Time, prof profile) {
	switch {
	case prof.Cap5h > 0 && account.Window5h.Cost >= prof.Cap5h:
		rateLimit(account, t, account.Window5h.Start.Add(window5h))
	case prof.Cap7d > 0 && account.Window7d.Cost >= prof.Cap7d:
		rateLimit(account, t, account.Window7d.Start.Add(window7d))
	case prof.CapDaily > 0 && dayUsage(account, t).Cost >= prof.CapDaily:
		rateLimit(account, t, dayStart(dayIndex(t)+1))
	}
}

// hourFactor 是该小时权重相对当天最大权重的比例，取值 0～1。
func hourFactor(t time.Time, cfg Config) float64 {
	local := t.In(cst)
	weights := cfg.hourWeights(local.Weekday())
	peak := 0
	for _, weight := range weights {
		peak = max(peak, weight)
	}
	if peak == 0 {
		return 0
	}
	return float64(weights[local.Hour()]) / float64(peak)
}

// dayUsage 返回 t 所在自然日的统计，没有时追加，并只保留最近 30 天。
func dayUsage(account *Account, t time.Time) *DayUsage {
	date := t.In(cst).Format(time.DateOnly)
	if n := len(account.Days); n > 0 && account.Days[n-1].Date == date {
		return &account.Days[n-1]
	}
	account.Days = append(account.Days, DayUsage{Date: date})
	if keep := int(backfillWindow / (24 * time.Hour)); len(account.Days) > keep {
		account.Days = append([]DayUsage(nil), account.Days[len(account.Days)-keep:]...)
	}
	return &account.Days[len(account.Days)-1]
}
