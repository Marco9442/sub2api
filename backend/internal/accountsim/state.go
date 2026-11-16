package accountsim

import "time"

// 账号状态与真实账号一致。
const (
	StatusActive   = "active"
	StatusInactive = "inactive"
	StatusError    = "error"
)

// Account 是一个模拟账号的完整持久化状态。
type Account struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Notes    string `json:"notes,omitempty"`
	Platform string `json:"platform"`
	Type     string `json:"type"`
	Tier     string `json:"tier"`

	GroupIDs       []int64 `json:"group_ids"`
	ProxyID        int64   `json:"proxy_id,omitempty"`
	Concurrency    int     `json:"concurrency"`
	Priority       int     `json:"priority"`
	RateMultiplier float64 `json:"rate_multiplier"`
	Schedulable    bool    `json:"schedulable"`
	Status         string  `json:"status"`
	ErrorMessage   string  `json:"error_message,omitempty"`

	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	LastUsedAt         *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
	AutoPauseOnExpired bool       `json:"auto_pause_on_expired"`

	RateLimitedAt     *time.Time `json:"rate_limited_at,omitempty"`
	RateLimitResetAt  *time.Time `json:"rate_limit_reset_at,omitempty"`
	OverloadUntil     *time.Time `json:"overload_until,omitempty"`
	TempUnschedUntil  *time.Time `json:"temp_unsched_until,omitempty"`
	TempUnschedReason string     `json:"temp_unsched_reason,omitempty"`

	// Credentials 和 Extra 只存非敏感展示字段，以及管理员在编辑弹窗里填写的内容。
	Credentials map[string]any `json:"credentials,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`

	// 以下为自动运营内部状态，不直接展示。
	Intensity float64   `json:"intensity"`
	RetireAt  time.Time `json:"retire_at"`
	// RetiredAt 非空表示已进入淘汰，DeleteAt 到达后删除。
	RetiredAt   *time.Time `json:"retired_at,omitempty"`
	DeleteAt    time.Time  `json:"delete_at"`
	RecoverAt   *time.Time `json:"recover_at,omitempty"`
	RetireOnErr bool       `json:"retire_on_err,omitempty"`

	Window5h Window     `json:"window_5h"`
	Window7d Window     `json:"window_7d"`
	Days     []DayUsage `json:"days"`

	// 累计值只增不减，不受 Days 只保留 30 天的影响。
	TotalRequests   int64   `json:"total_requests"`
	TotalTokens     int64   `json:"total_tokens"`
	TotalCost       float64 `json:"total_cost"`
	TotalActualCost float64 `json:"total_actual_cost"`

	// Pinned 记录管理员手动改过的字段，自动运营不再写入这些字段。
	// 状态被手动设置或账号由管理员创建时，生命周期冻结，不再自动淘汰。
	Pinned map[string]bool `json:"pinned,omitempty"`
}

// Window 是一个额度窗口内的累计用量。
// ActualCost 是生成时按当时倍率算出的费用。旧数据没有该字段，展示时回退到 Cost，不再乘当前倍率。
type Window struct {
	Start      time.Time `json:"start"`
	Requests   int64     `json:"requests"`
	Tokens     int64     `json:"tokens"`
	Cost       float64   `json:"cost"`
	ActualCost float64   `json:"actual_cost,omitempty"`
}

// DayUsage 是一个自然日（北京时间）的用量汇总。
type DayUsage struct {
	Date                string  `json:"date"`
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	Cost                float64 `json:"cost"`
	ActualCost          float64 `json:"actual_cost"`
	// Models 记录当天各模型的请求数，供统计弹窗的模型分布使用。
	Models map[string]int64 `json:"models,omitempty"`
}

// Tokens 返回当天全部 Token。
func (d DayUsage) Tokens() int64 {
	return d.InputTokens + d.OutputTokens + d.CacheReadTokens + d.CacheCreationTokens
}

// Group 是模拟分组。
type Group struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	// RateMultiplier 是分组倍率，展示用。
	RateMultiplier float64   `json:"rate_multiplier"`
	CreatedAt      time.Time `json:"created_at"`
}

// Proxy 是模拟代理。
type Proxy struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Protocol  string    `json:"protocol"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Region    string    `json:"region"`
	CreatedAt time.Time `json:"created_at"`
}

// State 是账号管理 #2 的全部状态。
type State struct {
	Accounts map[int64]*Account `json:"-"`
	Groups   []Group            `json:"groups"`
	Proxies  []Proxy            `json:"proxies"`
	// LastID 是已分配的最大账号 ID。删除的 ID 不复用。
	LastID      int64 `json:"last_id"`
	LastGroupID int64 `json:"last_group_id"`
	LastProxyID int64 `json:"last_proxy_id"`
	// Seq 是分组、代理命名里 {n} 的计数，键为命名前缀。
	Seq map[string]int64 `json:"seq"`
	// EvolvedUntil 是已演进到的时刻，之前的时段不会重复生成。
	// 零值表示首次使用，需要回填。
	EvolvedUntil time.Time `json:"evolved_until"`
}

// NewState 返回空状态。
func NewState() *State {
	return &State{Accounts: map[int64]*Account{}, Seq: map[string]int64{}}
}

// cloneState 复制一份可修改的状态。写入先改副本，落库成功后才替换内存。
func cloneState(src *State) *State {
	if src == nil {
		return NewState()
	}
	next := *src
	next.Accounts = make(map[int64]*Account, len(src.Accounts))
	for id, account := range src.Accounts {
		next.Accounts[id] = cloneAccount(account)
	}
	next.Groups = append([]Group(nil), src.Groups...)
	next.Proxies = append([]Proxy(nil), src.Proxies...)
	if src.Seq != nil {
		next.Seq = make(map[string]int64, len(src.Seq))
		for key, value := range src.Seq {
			next.Seq[key] = value
		}
	}
	return &next
}

// lifecycleFrozen 报告账号是否由管理员接管了生命周期。
func (a *Account) lifecycleFrozen() bool {
	return a.Pinned["status"] || a.Pinned["created"]
}

func (a *Account) pinned(field string) bool { return a.Pinned[field] }

// Pin 标记字段为手动设置。
func (a *Account) Pin(fields ...string) {
	if a.Pinned == nil {
		a.Pinned = map[string]bool{}
	}
	for _, field := range fields {
		a.Pinned[field] = true
	}
}
