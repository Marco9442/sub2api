package accountsim

import (
	"strings"
	"time"
)

// Config 是账号管理 #2 的全部可配置项。
// 零值不可用，必须先取 DefaultConfig 再调整；入库前经过 Normalize。
type Config struct {
	Enabled bool `json:"enabled"`

	DailyNewMin int64 `json:"daily_new_min"`
	DailyNewMax int64 `json:"daily_new_max"`
	// HourlyWeight 下标 0~23 为工作日每小时权重，24~47 为周末。
	HourlyWeight [48]int `json:"hourly_weight"`

	PlatformWeight map[string]int `json:"platform_weight"`
	// TypeWeight 的键是 "平台/认证类型"，TierWeight 的键是 "平台/订阅等级"。
	TypeWeight map[string]int `json:"type_weight"`
	TierWeight map[string]int `json:"tier_weight"`

	// UsageLevel 是整体使用强度，1 为默认。
	UsageLevel float64 `json:"usage_level"`
	// 单账号使用强度的随机范围，乘在整体强度上。
	IntensityMin float64 `json:"intensity_min"`
	IntensityMax float64 `json:"intensity_max"`

	// 异常事件按每账号每天的千分比触发。
	RateLimitPerMille   int `json:"rate_limit_per_mille"`
	OverloadPerMille    int `json:"overload_per_mille"`
	AuthErrorPerMille   int `json:"auth_error_per_mille"`
	BanPerMille         int `json:"ban_per_mille"`
	RateLimitMinutesMin int `json:"rate_limit_minutes_min"`
	RateLimitMinutesMax int `json:"rate_limit_minutes_max"`
	OverloadMinutesMin  int `json:"overload_minutes_min"`
	OverloadMinutesMax  int `json:"overload_minutes_max"`
	AuthErrorHoursMin   int `json:"auth_error_hours_min"`
	AuthErrorHoursMax   int `json:"auth_error_hours_max"`
	// 授权异常后能自动恢复的比例，其余进入淘汰。
	AuthErrorRecoverPercent int `json:"auth_error_recover_percent"`

	LifetimeDaysMin int `json:"lifetime_days_min"`
	LifetimeDaysMax int `json:"lifetime_days_max"`
	LingerDaysMin   int `json:"linger_days_min"`
	LingerDaysMax   int `json:"linger_days_max"`
	// 设置到期时间的账号比例，到期后按真实逻辑自动暂停。
	ExpiresPercent int `json:"expires_percent"`

	IDFloor int64 `json:"id_floor"`

	// 每多少个在役账号配一个分组、一个代理。
	AccountsPerGroup int    `json:"accounts_per_group"`
	AccountsPerProxy int    `json:"accounts_per_proxy"`
	GroupNamePattern string `json:"group_name_pattern"`
	ProxyNamePattern string `json:"proxy_name_pattern"`
}

// DefaultConfig 返回可直接长期运行的默认配置。
func DefaultConfig() Config {
	cfg := Config{
		Enabled:                 true,
		DailyNewMin:             30,
		DailyNewMax:             120,
		PlatformWeight:          map[string]int{},
		TypeWeight:              map[string]int{},
		TierWeight:              map[string]int{},
		UsageLevel:              1,
		IntensityMin:            0.4,
		IntensityMax:            1.6,
		RateLimitPerMille:       40,
		OverloadPerMille:        30,
		AuthErrorPerMille:       8,
		BanPerMille:             4,
		RateLimitMinutesMin:     20,
		RateLimitMinutesMax:     180,
		OverloadMinutesMin:      3,
		OverloadMinutesMax:      30,
		AuthErrorHoursMin:       1,
		AuthErrorHoursMax:       12,
		AuthErrorRecoverPercent: 60,
		LifetimeDaysMin:         3,
		LifetimeDaysMax:         15,
		LingerDaysMin:           1,
		LingerDaysMax:           3,
		ExpiresPercent:          35,
		AccountsPerGroup:        40,
		AccountsPerProxy:        25,
		GroupNamePattern:        "{platform}-{n}",
		ProxyNamePattern:        "{region}-{n}",
	}
	for hour := 0; hour < 24; hour++ {
		cfg.HourlyWeight[hour] = workdayWeight(hour)
		cfg.HourlyWeight[24+hour] = weekendWeight(hour)
	}
	for platform, weight := range defaultPlatformWeights {
		cfg.PlatformWeight[platform] = weight
	}
	fillCombinationWeights(&cfg)
	return cfg
}

func (c Config) hourWeights(day time.Weekday) []int {
	if day == time.Saturday || day == time.Sunday {
		return c.HourlyWeight[24:48]
	}
	return c.HourlyWeight[0:24]
}

func workdayWeight(hour int) int {
	switch {
	case hour >= 9 && hour < 12:
		return 10
	case hour >= 14 && hour < 18:
		return 9
	case hour >= 12 && hour < 14:
		return 6
	case hour >= 19 && hour < 23:
		return 5
	case hour >= 7 && hour < 9:
		return 4
	default:
		return 1
	}
}

func weekendWeight(hour int) int {
	if hour >= 10 && hour < 22 {
		return 4
	}
	return 1
}

// 平台权重只是默认分布，各平台都有非零值。
var defaultPlatformWeights = map[string]int{
	"anthropic":    30,
	"openai":       25,
	"gemini":       12,
	"antigravity":  6,
	"grok":         5,
	"kimi":         4,
	"zhipu":        4,
	"deepseek":     4,
	"minimax":      3,
	"typesafe":     2,
	"opencode_go":  2,
	"command_code": 1,
	"cline":        1,
}

// 各平台实际支持的认证类型。来源是真实账号创建时的平台约束。
var platformTypes = map[string][]string{
	"anthropic":    {"oauth", "setup-token", "apikey"},
	"openai":       {"oauth", "apikey"},
	"gemini":       {"oauth", "apikey"},
	"antigravity":  {"oauth"},
	"grok":         {"oauth", "apikey"},
	"kimi":         {"apikey"},
	"zhipu":        {"apikey"},
	"deepseek":     {"apikey"},
	"minimax":      {"apikey"},
	"typesafe":     {"apikey"},
	"opencode_go":  {"apikey"},
	"command_code": {"apikey"},
	"cline":        {"apikey"},
}

// 各平台实际存在的订阅等级，中档权重最高。
// 多协议供应商（kimi、zhipu、minimax、opencode_go 等）的等级就是真实的接入模式（account_mode）。
var platformTiers = map[string][]weightedTier{
	"anthropic":    {{"pro", 5}, {"max", 3}, {"team", 2}},
	"openai":       {{"plus", 5}, {"pro", 3}, {"team", 2}},
	"gemini":       {{"pro", 6}, {"ultra", 4}},
	"antigravity":  {{"pro", 7}, {"ultra", 3}},
	"grok":         {{"supergrok", 7}, {"heavy", 3}},
	"kimi":         {{"coding", 7}, {"payg", 3}},
	"zhipu":        {{"coding", 7}, {"payg", 3}},
	"deepseek":     {{"payg", 1}},
	"minimax":      {{"coding", 6}, {"payg", 4}},
	"typesafe":     {{"standard", 1}},
	"opencode_go":  {{"go", 8}, {"zen", 2}},
	"command_code": {{"payg", 1}},
	"cline":        {{"payg", 1}},
}

type weightedTier struct {
	name   string
	weight int
}

func fillCombinationWeights(cfg *Config) {
	for platform := range defaultPlatformWeights {
		for i, typ := range platformTypes[platform] {
			// 第一种认证类型权重最高，其余递减。
			cfg.TypeWeight[platform+"/"+typ] = 10 - i*3
		}
		for _, tier := range platformTiers[platform] {
			cfg.TierWeight[platform+"/"+tier.name] = tier.weight
		}
	}
}

// Supported 报告某平台是否支持给定的认证类型与订阅等级。
func Supported(platform, accountType, tier string) bool {
	ok := false
	for _, typ := range platformTypes[platform] {
		if typ == accountType {
			ok = true
		}
	}
	if !ok {
		return false
	}
	for _, item := range platformTiers[platform] {
		if item.name == tier {
			return true
		}
	}
	return false
}

// Normalize 修正越界和缺失的配置，返回可直接使用的副本。
// 前端提交和数据库读出的配置都经过这里，演进逻辑因此不必再做防御。
func (c Config) Normalize() Config {
	def := DefaultConfig()
	c.DailyNewMin = max(c.DailyNewMin, 0)
	c.DailyNewMax = max(c.DailyNewMax, c.DailyNewMin)
	for i := range c.HourlyWeight {
		c.HourlyWeight[i] = max(c.HourlyWeight[i], 0)
	}
	c.PlatformWeight = cleanWeights(c.PlatformWeight, def.PlatformWeight)
	c.TypeWeight = cleanWeights(c.TypeWeight, def.TypeWeight)
	c.TierWeight = cleanWeights(c.TierWeight, def.TierWeight)

	c.UsageLevel = min(max(c.UsageLevel, 0), 10)
	c.IntensityMin = max(c.IntensityMin, 0)
	c.IntensityMax = max(c.IntensityMax, c.IntensityMin)

	for _, v := range []*int{&c.RateLimitPerMille, &c.OverloadPerMille, &c.AuthErrorPerMille, &c.BanPerMille} {
		*v = min(max(*v, 0), 1000)
	}
	for _, v := range []*int{&c.AuthErrorRecoverPercent, &c.ExpiresPercent} {
		*v = min(max(*v, 0), 100)
	}
	clampRange(&c.RateLimitMinutesMin, &c.RateLimitMinutesMax, 1)
	clampRange(&c.OverloadMinutesMin, &c.OverloadMinutesMax, 1)
	clampRange(&c.AuthErrorHoursMin, &c.AuthErrorHoursMax, 1)
	clampRange(&c.LifetimeDaysMin, &c.LifetimeDaysMax, 1)
	clampRange(&c.LingerDaysMin, &c.LingerDaysMax, 0)

	c.IDFloor = max(c.IDFloor, 0)
	c.AccountsPerGroup = max(c.AccountsPerGroup, 1)
	c.AccountsPerProxy = max(c.AccountsPerProxy, 1)
	if strings.TrimSpace(c.GroupNamePattern) == "" {
		c.GroupNamePattern = def.GroupNamePattern
	}
	if strings.TrimSpace(c.ProxyNamePattern) == "" {
		c.ProxyNamePattern = def.ProxyNamePattern
	}
	return c
}

func clampRange(lo, hi *int, floor int) {
	*lo = max(*lo, floor)
	*hi = max(*hi, *lo)
}

// cleanWeights 复制权重表，去掉默认表里没有的键（未知平台或不支持的组合）和负值。
// 表为空时用默认表。
func cleanWeights(weights, defaults map[string]int) map[string]int {
	out := map[string]int{}
	for key, weight := range weights {
		if _, known := defaults[key]; known {
			out[key] = max(weight, 0)
		}
	}
	if len(out) == 0 {
		for key, weight := range defaults {
			out[key] = weight
		}
	}
	return out
}
