package accountsim

import (
	"sort"
	"time"
)

// backfillWindow 是首次使用时回填的历史长度，也是 Days 保留的天数。
const backfillWindow = 30 * 24 * time.Hour

// Evolve 把状态从上次演进的时刻推进到 at。
// 首次使用时回填 30 天；之后按 Step 逐段补齐，已演进的时段不会重复生成。
func Evolve(state *State, at time.Time, cfg Config) {
	end := at.Truncate(Step)
	if state.EvolvedUntil.IsZero() {
		start := end.Add(-backfillWindow)
		// 回填结束时最大 ID 恰好等于当前水位。
		state.LastID = IDWaterline(at, cfg) - (cumulative(end, cfg) - cumulative(start, cfg))
		state.EvolvedUntil = start
	}
	if gap := end.Sub(state.EvolvedUntil); gap > backfillWindow {
		// 长时间中断时，更早的时段只占用 ID，生成的账号到现在也已淘汰。
		skipTo := end.Add(-backfillWindow)
		state.LastID += cumulative(skipTo, cfg) - cumulative(state.EvolvedUntil, cfg)
		state.EvolvedUntil = skipTo
	}
	for t := state.EvolvedUntil; t.Before(end); t = t.Add(Step) {
		step(state, t, cfg)
		state.EvolvedUntil = t.Add(Step)
	}
}

// step 模拟 [t, t+Step) 这一段时间。
func step(state *State, t time.Time, cfg Config) {
	for _, id := range sortedIDs(state.Accounts) {
		account := state.Accounts[id]
		if retireDue(account, t) {
			delete(state.Accounts, id)
			continue
		}
		advance(account, t, cfg)
	}
	createAccounts(state, t, cfg)
	if t.Minute() == 0 {
		prunePools(state)
	}
}

func createAccounts(state *State, t time.Time, cfg Config) {
	hourStart := t.Truncate(time.Hour)
	from := int(t.Sub(hourStart) / time.Second)
	to := from + int(Step/time.Second)
	for _, offset := range hourPlan(hourStart, cfg) {
		if offset < from || offset >= to {
			continue
		}
		state.LastID++
		account := newAccount(state.LastID, hourStart.Add(time.Duration(offset)*time.Second), cfg)
		assignPools(state, account, cfg)
		state.Accounts[account.ID] = account
	}
}

// retireDue 报告已淘汰的账号是否过了滞留期，应当删除。
func retireDue(account *Account, t time.Time) bool {
	return account.RetiredAt != nil && !account.lifecycleFrozen() && !t.Before(account.DeleteAt)
}

func advance(account *Account, t time.Time, cfg Config) {
	if !t.After(account.CreatedAt) {
		return
	}
	r := newRNG(account.ID, t.Unix())
	clearExpired(account, t)
	resetWindows(account, t)
	if !account.lifecycleFrozen() && account.RetiredAt == nil {
		if !t.Before(account.RetireAt) {
			retire(account, t, r, cfg)
		} else {
			applyEvents(account, t, r, cfg)
		}
	}
	if account.ExpiresAt != nil && account.AutoPauseOnExpired && !t.Before(*account.ExpiresAt) && !account.pinned("schedulable") {
		// 与真实到期逻辑一致：只停止调度，不改状态。
		account.Schedulable = false
	}
	if serving(account, t) {
		consume(account, t, r, cfg)
	}
}

// clearExpired 清除到期的限流、过载，并恢复可恢复的授权异常。
func clearExpired(account *Account, t time.Time) {
	if account.RateLimitResetAt != nil && !t.Before(*account.RateLimitResetAt) {
		account.RateLimitedAt, account.RateLimitResetAt = nil, nil
	}
	if account.OverloadUntil != nil && !t.Before(*account.OverloadUntil) {
		account.OverloadUntil = nil
	}
	if account.RecoverAt != nil && !t.Before(*account.RecoverAt) {
		account.RecoverAt = nil
		if !account.pinned("status") && account.Status == StatusError {
			account.Status, account.ErrorMessage = StatusActive, ""
		}
	}
}

var (
	retireMessages = []string{"OAuth token refresh failed: invalid_grant", "Account has been disabled by upstream", "Subscription expired"}
	authMessages   = []string{"401 Unauthorized: token expired", "OAuth token refresh failed: invalid_grant", "403 Forbidden: permission denied"}
	banMessages    = []string{"Account has been suspended", "403 Forbidden: organization disabled"}
)

func retire(account *Account, t time.Time, r *rng, cfg Config) {
	retiredAt := t
	account.RetiredAt = &retiredAt
	account.DeleteAt = t.Add(time.Duration(r.between(cfg.LingerDaysMin*24, cfg.LingerDaysMax*24+23)) * time.Hour)
	account.RecoverAt = nil
	if !account.RetireOnErr {
		if r.intn(100) < 60 {
			account.Status, account.ErrorMessage = StatusInactive, ""
		} else {
			account.Status, account.ErrorMessage = StatusError, r.choose(retireMessages)
		}
	}
	account.Schedulable = false
	account.UpdatedAt = t
}

// applyEvents 按千分比触发限流、过载、授权异常和封禁。概率按天给出，分摊到每个 Step。
func applyEvents(account *Account, t time.Time, r *rng, cfg Config) {
	stepsPerDay := int(24 * time.Hour / Step)
	roll := func(perMille int) bool { return r.intn(1000*stepsPerDay) < perMille }

	if account.Status != StatusActive || account.pinned("status") {
		return
	}
	switch {
	case roll(cfg.BanPerMille):
		account.Status, account.ErrorMessage = StatusError, r.choose(banMessages)
		account.RetireOnErr = true
		account.RetireAt = t
		account.UpdatedAt = t
	case roll(cfg.AuthErrorPerMille):
		account.Status, account.ErrorMessage = StatusError, r.choose(authMessages)
		account.UpdatedAt = t
		if r.intn(100) < cfg.AuthErrorRecoverPercent {
			recoverAt := t.Add(time.Duration(r.between(cfg.AuthErrorHoursMin*60, cfg.AuthErrorHoursMax*60)) * time.Minute)
			account.RecoverAt = &recoverAt
		} else {
			account.RetireOnErr = true
			account.RetireAt = t
		}
	case account.RateLimitResetAt == nil && roll(cfg.RateLimitPerMille):
		rateLimit(account, t, t.Add(time.Duration(r.between(cfg.RateLimitMinutesMin, cfg.RateLimitMinutesMax))*time.Minute))
	case account.OverloadUntil == nil && roll(cfg.OverloadPerMille):
		until := t.Add(time.Duration(r.between(cfg.OverloadMinutesMin, cfg.OverloadMinutesMax)) * time.Minute)
		account.OverloadUntil = &until
	}
}

func rateLimit(account *Account, t, reset time.Time) {
	at := t
	account.RateLimitedAt, account.RateLimitResetAt = &at, &reset
}

// serving 报告账号在这一段时间里是否会被调度使用。
func serving(account *Account, t time.Time) bool {
	return account.Status == StatusActive && account.Schedulable && account.RetiredAt == nil &&
		account.RateLimitResetAt == nil && (account.OverloadUntil == nil || !t.Before(*account.OverloadUntil))
}

func sortedIDs(accounts map[int64]*Account) []int64 {
	ids := make([]int64, 0, len(accounts))
	for id := range accounts {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
