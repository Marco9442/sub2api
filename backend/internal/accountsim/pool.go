package accountsim

import (
	"fmt"
	"strings"
)

var (
	proxyRegions     = []string{"hk", "sg", "jp", "us-west", "us-east", "de", "uk", "kr"}
	proxyProtocols   = []string{"http", "socks5"}
	groupMultipliers = []float64{1, 1, 1, 1.2, 1.5, 0.8}
)

// assignPools 把新账号放进同平台成员最少的分组和成员最少的代理。
// 现有分组或代理都已满时新建一个，分组和代理因此随号池规模增长。
func assignPools(state *State, account *Account, cfg Config) {
	groupMembers, proxyMembers := poolMembers(state)

	var group *Group
	for i := range state.Groups {
		g := &state.Groups[i]
		if g.Platform == account.Platform && (group == nil || groupMembers[g.ID] < groupMembers[group.ID]) {
			group = g
		}
	}
	if group == nil || groupMembers[group.ID] >= cfg.AccountsPerGroup {
		group = newGroup(state, account, cfg)
	}
	account.GroupIDs = []int64{group.ID}

	var proxy *Proxy
	for i := range state.Proxies {
		p := &state.Proxies[i]
		if proxy == nil || proxyMembers[p.ID] < proxyMembers[proxy.ID] {
			proxy = p
		}
	}
	if proxy == nil || proxyMembers[proxy.ID] >= cfg.AccountsPerProxy {
		proxy = newProxy(state, account, cfg)
	}
	account.ProxyID = proxy.ID
}

func newGroup(state *State, account *Account, cfg Config) *Group {
	r := newRNG(account.ID, 0x67)
	state.LastGroupID++
	state.Groups = append(state.Groups, Group{
		ID:             state.LastGroupID,
		Name:           poolName(state, cfg.GroupNamePattern, map[string]string{"platform": account.Platform}),
		Platform:       account.Platform,
		RateMultiplier: groupMultipliers[r.intn(len(groupMultipliers))],
		CreatedAt:      account.CreatedAt,
	})
	return &state.Groups[len(state.Groups)-1]
}

func newProxy(state *State, account *Account, cfg Config) *Proxy {
	r := newRNG(account.ID, 0x70)
	region := r.choose(proxyRegions)
	state.LastProxyID++
	state.Proxies = append(state.Proxies, Proxy{
		ID:       state.LastProxyID,
		Name:     poolName(state, cfg.ProxyNamePattern, map[string]string{"region": region, "platform": account.Platform}),
		Protocol: r.choose(proxyProtocols),
		// 地址取文档保留网段，不会指向真实主机。
		Host:      fmt.Sprintf("198.51.100.%d", r.between(2, 254)),
		Port:      r.between(10000, 60000),
		Region:    region,
		CreatedAt: account.CreatedAt,
	})
	return &state.Proxies[len(state.Proxies)-1]
}

// poolName 替换命名模板里的占位符，{n} 按替换后的前缀各自计数。
func poolName(state *State, pattern string, values map[string]string) string {
	name := pattern
	for key, value := range values {
		name = strings.ReplaceAll(name, "{"+key+"}", value)
	}
	if !strings.Contains(name, "{n}") {
		return name
	}
	state.Seq[name]++
	return strings.ReplaceAll(name, "{n}", fmt.Sprint(state.Seq[name]))
}

// prunePools 删除没有成员的分组和代理，号池缩小时它们随之减少。
func prunePools(state *State) {
	groupMembers, proxyMembers := poolMembers(state)
	groups := state.Groups[:0]
	for _, group := range state.Groups {
		if groupMembers[group.ID] > 0 {
			groups = append(groups, group)
		}
	}
	state.Groups = groups
	proxies := state.Proxies[:0]
	for _, proxy := range state.Proxies {
		if proxyMembers[proxy.ID] > 0 {
			proxies = append(proxies, proxy)
		}
	}
	state.Proxies = proxies
}

func poolMembers(state *State) (groups, proxies map[int64]int) {
	groups, proxies = map[int64]int{}, map[int64]int{}
	for _, account := range state.Accounts {
		for _, id := range account.GroupIDs {
			groups[id]++
		}
		if account.ProxyID != 0 {
			proxies[account.ProxyID]++
		}
	}
	return groups, proxies
}
