package accountsim

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// accountResponse 与真实接口的 AccountWithConcurrency 序列化结果一致。
type accountResponse struct {
	*dto.Account
	CurrentConcurrency int `json:"current_concurrency"`
}

// accountListItem 与真实接口 lite=1 时的 AccountListItemWithConcurrency 一致。
type accountListItem struct {
	*dto.AccountListItem
	CurrentConcurrency int `json:"current_concurrency"`
}

// toService 把模拟账号转成真实的 service.Account，再交给真实 DTO 映射，保证响应结构一致。
// 管理员手动填写的 credentials / extra 覆盖自动生成的展示字段。
func toService(a *Account, state *State, now time.Time) *service.Account {
	rate := a.RateMultiplier
	out := &service.Account{
		ID:                      a.ID,
		Name:                    a.Name,
		Platform:                a.Platform,
		Type:                    a.Type,
		Credentials:             merged(displayCredentials(a), a.Credentials),
		Extra:                   merged(displayExtra(a, now), a.Extra),
		Concurrency:             a.Concurrency,
		Priority:                a.Priority,
		RateMultiplier:          &rate,
		Status:                  a.Status,
		ErrorMessage:            a.ErrorMessage,
		LastUsedAt:              a.LastUsedAt,
		ExpiresAt:               a.ExpiresAt,
		AutoPauseOnExpired:      a.AutoPauseOnExpired,
		CreatedAt:               a.CreatedAt,
		UpdatedAt:               a.UpdatedAt,
		Schedulable:             a.Schedulable,
		RateLimitedAt:           activeAt(a.RateLimitedAt, a.RateLimitResetAt, now),
		RateLimitResetAt:        activeUntil(a.RateLimitResetAt, now),
		OverloadUntil:           activeUntil(a.OverloadUntil, now),
		TempUnschedulableUntil:  activeUntil(a.TempUnschedUntil, now),
		TempUnschedulableReason: a.TempUnschedReason,
		GroupIDs:                append([]int64{}, a.GroupIDs...),
	}
	if a.Notes != "" {
		notes := a.Notes
		out.Notes = &notes
	}
	if a.Window5h.Requests > 0 && a.Platform == "anthropic" {
		start, end := a.Window5h.Start, a.Window5h.Start.Add(window5h)
		out.SessionWindowStart, out.SessionWindowEnd, out.SessionWindowStatus = &start, &end, "allowed"
	}
	if proxy := findProxy(state, a.ProxyID); proxy != nil {
		id := proxy.ID
		out.ProxyID = &id
		out.Proxy = proxyToService(proxy)
	}
	for _, groupID := range a.GroupIDs {
		if group := findGroup(state, groupID); group != nil {
			g := groupToService(group)
			out.Groups = append(out.Groups, g)
			out.AccountGroups = append(out.AccountGroups, service.AccountGroup{
				AccountID: a.ID, GroupID: group.ID, Priority: 1, CreatedAt: a.CreatedAt, Group: g,
			})
		}
	}
	return out
}

// view 是一次请求看到的状态快照，只在 Service.View / Update 的回调里使用。
type view struct {
	state *State
	cfg   Config
	now   time.Time
}

func (v view) detail(a *Account) accountResponse {
	return accountResponse{Account: dto.AccountFromService(toService(a, v.state, v.now)), CurrentConcurrency: v.concurrency(a)}
}

func (v view) listItem(a *Account) accountListItem {
	full := dto.AccountFromService(toService(a, v.state, v.now))
	return accountListItem{AccountListItem: dto.AccountListItemFromAccount(full), CurrentConcurrency: v.concurrency(a)}
}

// concurrency 按分钟给出稳定的在用并发，只在账号可接单时非零。
func (v view) concurrency(a *Account) int {
	if !serving(a, v.now) || a.Concurrency <= 0 {
		return 0
	}
	load := a.Intensity * hourFactor(v.now, v.cfg) * newRNG(a.ID, v.now.Truncate(time.Minute).Unix()).float()
	return min(a.Concurrency, int(float64(a.Concurrency)*load+0.5))
}

func activeUntil(until *time.Time, now time.Time) *time.Time {
	if until == nil || !until.After(now) {
		return nil
	}
	return until
}

func activeAt(at, until *time.Time, now time.Time) *time.Time {
	if activeUntil(until, now) == nil {
		return nil
	}
	return at
}

func merged(base, override map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(override))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		out[k] = v
	}
	return out
}

func findProxy(state *State, id int64) *Proxy {
	for i := range state.Proxies {
		if state.Proxies[i].ID == id {
			return &state.Proxies[i]
		}
	}
	return nil
}

func findGroup(state *State, id int64) *Group {
	for i := range state.Groups {
		if state.Groups[i].ID == id {
			return &state.Groups[i]
		}
	}
	return nil
}

func proxyToService(p *Proxy) *service.Proxy {
	return &service.Proxy{
		ID: p.ID, Name: p.Name, Protocol: p.Protocol, Host: p.Host, Port: p.Port,
		Status: service.StatusActive, CreatedAt: p.CreatedAt, UpdatedAt: p.CreatedAt,
	}
}

func groupToService(g *Group) *service.Group {
	return &service.Group{
		ID: g.ID, Name: g.Name, Platform: g.Platform, RateMultiplier: g.RateMultiplier,
		Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard,
		CreatedAt: g.CreatedAt, UpdatedAt: g.CreatedAt,
	}
}
