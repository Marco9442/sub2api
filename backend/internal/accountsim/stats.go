package accountsim

import (
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/gin-gonic/gin"
)

// statsExactDays 是按天用量真实保留的天数。更长的区间用这些天的日均补上。
const statsExactDays = 30

func (h *Handler) stats(c *gin.Context, p params) {
	id, ok := p.id(c)
	if !ok {
		return
	}
	days := parseStatsDays(c.Query("days"))
	h.read(c, func(v view) {
		account := v.state.Accounts[id]
		if account == nil {
			notFound(c)
			return
		}
		response.Success(c, accountStats(account, v.now, days))
	})
}

// parseStatsDays 与真实 GetStats 一样：缺省 30，只接受 1 到 90。
func parseStatsDays(raw string) int {
	days, err := strconv.Atoi(raw)
	if raw == "" || err != nil || days <= 0 || days > 90 {
		return statsExactDays
	}
	return days
}

func accountStats(account *Account, now time.Time, days int) usagestats.AccountUsageStatsResponse {
	today := dayOf(now)
	start := today.AddDate(0, 0, -(days - 1))
	// 与真实统计接口一致：窗口右端是明天 0 点，天数比参数多 1。
	span := int(today.AddDate(0, 0, 1).Sub(start).Hours()/24) + 1
	exactStart := today.AddDate(0, 0, -(statsExactDays - 1))
	created := dayOf(account.CreatedAt)

	stored := make([]DayUsage, 0, len(account.Days))
	byDate := make(map[string]DayUsage, len(account.Days))
	for _, day := range account.Days {
		at, ok := parseDay(day.Date)
		if !ok || at.Before(created) || day.empty() {
			continue
		}
		stored = append(stored, day)
		byDate[day.Date] = day
	}
	average := meanDay(stored)

	points := make([]DayUsage, 0, days)
	for at := start; !at.After(today); at = at.AddDate(0, 0, 1) {
		if at.Before(created) {
			continue
		}
		date := at.Format(time.DateOnly)
		if day, ok := byDate[date]; ok {
			day.Date = date
			points = append(points, day)
			continue
		}
		if days > statsExactDays && at.Before(exactStart) && len(stored) > 0 {
			fill := average
			fill.Date = date
			points = append(points, fill)
		}
	}

	history := make([]usagestats.AccountUsageHistory, 0, len(points))
	models := map[string]*usagestats.ModelStat{}
	var requests, tokens int64
	for _, day := range points {
		history = append(history, historyFromDay(day))
		addModelShares(models, day)
		requests += day.Requests
		tokens += day.Tokens()
	}

	return usagestats.AccountUsageStatsResponse{
		History:           history,
		Summary:           statsSummary(history, span, today, requests, tokens),
		Models:            modelList(models),
		Endpoints:         []usagestats.EndpointStat{},
		UpstreamEndpoints: []usagestats.EndpointStat{},
	}
}

func (d DayUsage) empty() bool {
	return d.Requests == 0 && d.Tokens() == 0 && d.Cost == 0 && d.ActualCost == 0
}

func dayOf(t time.Time) time.Time {
	local := t.In(cst)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, cst)
}

func parseDay(date string) (time.Time, bool) {
	at, err := time.ParseInLocation(time.DateOnly, date, cst)
	return at, err == nil
}

func meanDay(days []DayUsage) DayUsage {
	if len(days) == 0 {
		return DayUsage{}
	}
	n := float64(len(days))
	var requests, input, output, cacheRead, cacheWrite int64
	var cost, actual float64
	models := map[string]float64{}
	for _, day := range days {
		requests += day.Requests
		input += day.InputTokens
		output += day.OutputTokens
		cacheRead += day.CacheReadTokens
		cacheWrite += day.CacheCreationTokens
		cost += day.Cost
		actual += day.ActualCost
		for name, count := range day.Models {
			models[name] += float64(count)
		}
	}
	out := DayUsage{
		Requests:            int64(math.Round(float64(requests) / n)),
		InputTokens:         int64(math.Round(float64(input) / n)),
		OutputTokens:        int64(math.Round(float64(output) / n)),
		CacheReadTokens:     int64(math.Round(float64(cacheRead) / n)),
		CacheCreationTokens: int64(math.Round(float64(cacheWrite) / n)),
		Cost:                cost / n,
		ActualCost:          actual / n,
		Models:              map[string]int64{},
	}
	for name, count := range models {
		if rounded := int64(math.Round(count / n)); rounded > 0 {
			out.Models[name] = rounded
		}
	}
	return out
}

func historyFromDay(day DayUsage) usagestats.AccountUsageHistory {
	at, _ := parseDay(day.Date)
	cost := storedCost(day.ActualCost, day.Cost)
	return usagestats.AccountUsageHistory{
		Date:       day.Date,
		Label:      at.Format("01/02"),
		Requests:   day.Requests,
		Tokens:     day.Tokens(),
		Cost:       day.Cost,
		ActualCost: cost,
		UserCost:   cost,
	}
}

func statsSummary(history []usagestats.AccountUsageHistory, span int, today time.Time, requests, tokens int64) usagestats.AccountUsageSummary {
	used := len(history)
	if used == 0 {
		used = 1
	}
	var accountCost, userCost, standardCost float64
	var peakCost, peakRequests *usagestats.AccountUsageHistory
	for i := range history {
		item := &history[i]
		accountCost += item.ActualCost
		userCost += item.UserCost
		standardCost += item.Cost
		if peakCost == nil || item.ActualCost > peakCost.ActualCost {
			peakCost = item
		}
		if peakRequests == nil || item.Requests > peakRequests.Requests {
			peakRequests = item
		}
	}
	summary := usagestats.AccountUsageSummary{
		Days:              span,
		ActualDaysUsed:    len(history),
		TotalCost:         accountCost,
		TotalUserCost:     userCost,
		TotalStandardCost: standardCost,
		TotalRequests:     requests,
		TotalTokens:       tokens,
		AvgDailyCost:      accountCost / float64(used),
		AvgDailyUserCost:  userCost / float64(used),
		AvgDailyRequests:  float64(requests) / float64(used),
		AvgDailyTokens:    float64(tokens) / float64(used),
		AvgDurationMs:     averageDuration(requests, tokens),
	}
	todayKey := today.Format(time.DateOnly)
	for i := range history {
		if history[i].Date != todayKey {
			continue
		}
		summary.Today = &struct {
			Date     string  `json:"date"`
			Cost     float64 `json:"cost"`
			UserCost float64 `json:"user_cost"`
			Requests int64   `json:"requests"`
			Tokens   int64   `json:"tokens"`
		}{
			Date:     history[i].Date,
			Cost:     history[i].ActualCost,
			UserCost: history[i].UserCost,
			Requests: history[i].Requests,
			Tokens:   history[i].Tokens,
		}
		break
	}
	if peakCost != nil {
		summary.HighestCostDay = &struct {
			Date     string  `json:"date"`
			Label    string  `json:"label"`
			Cost     float64 `json:"cost"`
			UserCost float64 `json:"user_cost"`
			Requests int64   `json:"requests"`
		}{
			Date:     peakCost.Date,
			Label:    peakCost.Label,
			Cost:     peakCost.ActualCost,
			UserCost: peakCost.UserCost,
			Requests: peakCost.Requests,
		}
	}
	if peakRequests != nil {
		summary.HighestRequestDay = &struct {
			Date     string  `json:"date"`
			Label    string  `json:"label"`
			Requests int64   `json:"requests"`
			Cost     float64 `json:"cost"`
			UserCost float64 `json:"user_cost"`
		}{
			Date:     peakRequests.Date,
			Label:    peakRequests.Label,
			Requests: peakRequests.Requests,
			Cost:     peakRequests.ActualCost,
			UserCost: peakRequests.UserCost,
		}
	}
	return summary
}

// averageDuration 没有逐条耗时。按每次请求的 token 量给出一个稳定的毫秒数，
// 用量差得开的账号不会显示成同一个值。
func averageDuration(requests, tokens int64) float64 {
	if requests <= 0 || tokens <= 0 {
		return 0
	}
	ms := 400 + 900*math.Log10(float64(tokens)/float64(requests))
	if ms < 400 {
		return 400
	}
	if ms > 8000 {
		return 8000
	}
	return ms
}

func addModelShares(into map[string]*usagestats.ModelStat, day DayUsage) {
	var all int64
	for _, count := range day.Models {
		if count > 0 {
			all += count
		}
	}
	if all == 0 {
		return
	}
	for name, count := range day.Models {
		if name == "" || count <= 0 {
			continue
		}
		item := into[name]
		if item == nil {
			item = &usagestats.ModelStat{Model: name}
			into[name] = item
		}
		share := float64(count) / float64(all)
		item.Requests += count
		item.InputTokens += roundShare(day.InputTokens, count, all)
		item.OutputTokens += roundShare(day.OutputTokens, count, all)
		item.CacheReadTokens += roundShare(day.CacheReadTokens, count, all)
		item.CacheCreationTokens += roundShare(day.CacheCreationTokens, count, all)
		item.TotalTokens = item.InputTokens + item.OutputTokens + item.CacheReadTokens + item.CacheCreationTokens
		item.Cost += day.Cost * share
		accountCost := storedCost(day.ActualCost, day.Cost) * share
		item.ActualCost += accountCost
		item.AccountCost += accountCost
	}
}

func roundShare(total, part, all int64) int64 {
	return int64(math.Round(float64(total) * float64(part) / float64(all)))
}

func modelList(into map[string]*usagestats.ModelStat) []usagestats.ModelStat {
	out := make([]usagestats.ModelStat, 0, len(into))
	for _, item := range into {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool { return modelLess(out[i], out[j]) })
	return out
}

func modelLess(a, b usagestats.ModelStat) bool {
	if a.TotalTokens != b.TotalTokens {
		return a.TotalTokens > b.TotalTokens
	}
	return a.Model < b.Model
}
