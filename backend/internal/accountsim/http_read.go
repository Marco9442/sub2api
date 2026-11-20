package accountsim

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *Handler) registerRoutes() {
	h.handle(http.MethodGet, "accounts", h.list)
	h.handle(http.MethodGet, "accounts/:id", h.detail)
	h.handle(http.MethodGet, "accounts/:id/usage", h.usageOne)
	h.handle(http.MethodGet, "accounts/:id/today-stats", h.todayStatsOne)
	h.handle(http.MethodGet, "accounts/:id/stats", h.stats)
	h.handle(http.MethodPost, "accounts/:id/test", h.testConnection)
	h.handle(http.MethodGet, "accounts/:id/models", h.models)
	h.handle(http.MethodPost, "accounts/:id/models/sync-upstream", h.syncModels)
	h.handle(http.MethodPost, "accounts/usage/batch", h.usageBatch)
	h.handle(http.MethodPost, "accounts/today-stats/batch", h.todayStatsBatch)
	h.handle(http.MethodGet, "accounts/upstream-billing-probe/settings", h.probeSettings)
	h.handle(http.MethodGet, "groups/all", h.groupsAll)
	h.handle(http.MethodGet, "proxies/all", h.proxiesAll)
	h.handle(http.MethodGet, "config", h.getConfig)
	h.handle(http.MethodPut, "config", h.putConfig)
	h.handle(http.MethodGet, "dashboard/user-breakdown", h.userBreakdown)
	h.registerWriteRoutes()
}

// userBreakdown 是统计弹窗模型明细的空结果。不读真实用户，也不写状态。
func (h *Handler) userBreakdown(c *gin.Context, _ params) {
	response.Success(c, gin.H{
		"users":      []any{},
		"start_date": c.Query("start_date"),
		"end_date":   c.Query("end_date"),
	})
}

// read 在读锁下执行 fn，加载失败时写入 500。
func (h *Handler) read(c *gin.Context, fn func(v view)) {
	err := h.svc.View(c.Request.Context(), func(state *State, cfg Config, now time.Time) {
		fn(view{state: state, cfg: cfg, now: now})
	})
	if err != nil {
		internalError(c, err)
	}
}

func (h *Handler) list(c *gin.Context, _ params) {
	page, pageSize := response.ParsePagination(c)
	filter, ok := parseListFilter(c)
	if !ok {
		return
	}
	lite := c.Query("lite") == "1" || c.Query("lite") == "true"
	h.read(c, func(v view) {
		matched := make([]*Account, 0, len(v.state.Accounts))
		for _, a := range v.state.Accounts {
			if filter.match(a, v.now) {
				matched = append(matched, a)
			}
		}
		sortAccounts(matched, c.DefaultQuery("sort_by", "name"), c.DefaultQuery("sort_order", "asc"))
		total := int64(len(matched))
		from := min((page-1)*pageSize, len(matched))
		to := min(from+pageSize, len(matched))
		matched = matched[from:to]
		if lite {
			items := make([]accountListItem, 0, len(matched))
			for _, a := range matched {
				items = append(items, v.listItem(a))
			}
			response.Paginated(c, items, total, page, pageSize)
			return
		}
		items := make([]accountResponse, 0, len(matched))
		for _, a := range matched {
			items = append(items, v.detail(a))
		}
		response.Paginated(c, items, total, page, pageSize)
	})
}

// listFilter 与真实列表的筛选语义一致（repository.accountListFilteredQuery）。
type listFilter struct {
	platform, accountType, status, search, privacyMode string
	groupID                                            int64
}

const groupUngrouped = service.AccountListGroupUngrouped

func parseListFilter(c *gin.Context) (listFilter, bool) {
	f := listFilter{
		platform:    c.Query("platform"),
		accountType: c.Query("type"),
		status:      c.Query("status"),
		search:      strings.TrimSpace(c.Query("search")),
		privacyMode: strings.TrimSpace(c.Query("privacy_mode")),
	}
	if len(f.search) > 100 {
		f.search = f.search[:100]
	}
	if raw := c.Query("group"); raw != "" {
		if raw == "ungrouped" {
			f.groupID = groupUngrouped
		} else {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || id < 0 {
				response.BadRequest(c, "invalid group filter")
				return f, false
			}
			f.groupID = id
		}
	}
	return f, true
}

func (f listFilter) match(a *Account, now time.Time) bool {
	if f.platform != "" && a.Platform != f.platform {
		return false
	}
	if f.accountType != "" && a.Type != f.accountType {
		return false
	}
	if f.status != "" && !matchStatus(a, f.status, now) {
		return false
	}
	if f.search != "" && !strings.Contains(strings.ToLower(a.Name), strings.ToLower(f.search)) {
		return false
	}
	switch {
	case f.groupID == groupUngrouped && len(a.GroupIDs) > 0:
		return false
	case f.groupID > 0 && !containsID(a.GroupIDs, f.groupID):
		return false
	}
	if f.privacyMode != "" {
		mode, _ := a.Extra["privacy_mode"].(string)
		if f.privacyMode == service.AccountPrivacyModeUnsetFilter {
			return mode == ""
		}
		return mode == f.privacyMode
	}
	return true
}

func matchStatus(a *Account, status string, now time.Time) bool {
	limited := activeUntil(a.RateLimitResetAt, now) != nil
	tempUnsched := activeUntil(a.TempUnschedUntil, now) != nil
	switch status {
	case StatusActive:
		return a.Status == StatusActive && a.Schedulable && !limited && !tempUnsched
	case "rate_limited":
		return a.Status == StatusActive && limited && !tempUnsched
	case "temp_unschedulable":
		return a.Status == StatusActive && tempUnsched
	case "unschedulable":
		return a.Status == StatusActive && !a.Schedulable && !limited && !tempUnsched
	default:
		return a.Status == status
	}
}

func containsID(ids []int64, id int64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// sortAccounts 与真实列表的排序一致（repository.accountListOrder）：
// 按指定字段排序，同值按 id；空时间排在升序末尾、降序开头，与 PostgreSQL 默认一致。
func sortAccounts(accounts []*Account, sortBy, sortOrder string) {
	desc := strings.EqualFold(strings.TrimSpace(sortOrder), "desc")
	compare := fieldComparer(strings.ToLower(strings.TrimSpace(sortBy)))
	sort.SliceStable(accounts, func(i, j int) bool {
		a, b := accounts[i], accounts[j]
		if c := compare(a, b); c != 0 {
			return (c < 0) != desc
		}
		return (a.ID < b.ID) != desc
	})
}

func fieldComparer(sortBy string) func(a, b *Account) int {
	switch sortBy {
	case "id", "upstream_billing_rate":
		// 模拟账号没有上游倍率探测结果，真实列表此时按 id 排序。
		return func(a, b *Account) int { return 0 }
	case "status":
		return func(a, b *Account) int { return strings.Compare(a.Status, b.Status) }
	case "schedulable":
		return func(a, b *Account) int { return boolCompare(a.Schedulable, b.Schedulable) }
	case "priority":
		return func(a, b *Account) int { return a.Priority - b.Priority }
	case "rate_multiplier":
		return func(a, b *Account) int { return floatCompare(a.RateMultiplier, b.RateMultiplier) }
	case "last_used_at":
		return func(a, b *Account) int { return timeCompare(a.LastUsedAt, b.LastUsedAt) }
	case "expires_at":
		return func(a, b *Account) int { return timeCompare(a.ExpiresAt, b.ExpiresAt) }
	case "created_at":
		return func(a, b *Account) int { return a.CreatedAt.Compare(b.CreatedAt) }
	default:
		return func(a, b *Account) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) }
	}
}

func boolCompare(a, b bool) int {
	switch {
	case a == b:
		return 0
	case !a:
		return -1
	default:
		return 1
	}
}

func floatCompare(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func timeCompare(a, b *time.Time) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return 1
	case b == nil:
		return -1
	default:
		return a.Compare(*b)
	}
}

func (h *Handler) detail(c *gin.Context, p params) {
	id, ok := p.id(c)
	if !ok {
		return
	}
	h.read(c, func(v view) {
		a := v.state.Accounts[id]
		if a == nil {
			notFound(c)
			return
		}
		response.Success(c, v.detail(a))
	})
}

func (h *Handler) groupsAll(c *gin.Context, _ params) {
	platform := c.Query("platform")
	h.read(c, func(v view) {
		out := make([]dto.AdminGroup, 0, len(v.state.Groups))
		for i := range v.state.Groups {
			g := &v.state.Groups[i]
			if platform != "" && g.Platform != platform {
				continue
			}
			out = append(out, *dto.GroupFromServiceAdmin(groupToService(g)))
		}
		response.Success(c, out)
	})
}

func (h *Handler) proxiesAll(c *gin.Context, _ params) {
	withCount := c.Query("with_count") == "true"
	h.read(c, func(v view) {
		if !withCount {
			out := make([]dto.AdminProxy, 0, len(v.state.Proxies))
			for i := range v.state.Proxies {
				out = append(out, *dto.ProxyFromServiceAdmin(proxyToService(&v.state.Proxies[i])))
			}
			response.Success(c, out)
			return
		}
		_, members := poolMembers(v.state)
		out := make([]dto.AdminProxyWithAccountCount, 0, len(v.state.Proxies))
		for i := range v.state.Proxies {
			p := &v.state.Proxies[i]
			out = append(out, *dto.ProxyWithAccountCountFromServiceAdmin(&service.ProxyWithAccountCount{
				Proxy: *proxyToService(p), AccountCount: int64(members[p.ID]),
			}))
		}
		response.Success(c, out)
	})
}

func parseInt64(raw string) (int64, bool) {
	v, err := strconv.ParseInt(raw, 10, 64)
	return v, err == nil && v > 0
}

type idListRequest struct {
	AccountIDs []int64 `json:"account_ids"`
	Force      bool    `json:"force"`
}

func (h *Handler) usageOne(c *gin.Context, p params) {
	id, ok := p.id(c)
	if !ok {
		return
	}
	h.read(c, func(v view) {
		a := v.state.Accounts[id]
		if a == nil {
			notFound(c)
			return
		}
		response.Success(c, usageInfo(a, v.now))
	})
}

func (h *Handler) usageBatch(c *gin.Context, _ params) {
	var req idListRequest
	if !bind(c, &req) {
		return
	}
	h.read(c, func(v view) {
		usage := map[string]*service.UsageInfo{}
		for _, id := range req.AccountIDs {
			a := v.state.Accounts[id]
			if a == nil {
				continue
			}
			usage[strconv.FormatInt(id, 10)] = usageInfo(a, v.now)
		}
		response.Success(c, gin.H{"usage": usage, "errors": map[string]string{}})
	})
}

func (h *Handler) todayStatsOne(c *gin.Context, p params) {
	id, ok := p.id(c)
	if !ok {
		return
	}
	h.read(c, func(v view) {
		a := v.state.Accounts[id]
		if a == nil {
			notFound(c)
			return
		}
		response.Success(c, todayStats(a, v.now))
	})
}

func (h *Handler) todayStatsBatch(c *gin.Context, _ params) {
	var req idListRequest
	if !bind(c, &req) {
		return
	}
	h.read(c, func(v view) {
		stats := map[string]service.WindowStats{}
		for _, id := range req.AccountIDs {
			a := v.state.Accounts[id]
			if a == nil {
				continue
			}
			stats[strconv.FormatInt(id, 10)] = todayStats(a, v.now)
		}
		response.Success(c, gin.H{"stats": stats})
	})
}

func (h *Handler) probeSettings(c *gin.Context, _ params) {
	response.Success(c, service.UpstreamBillingProbeSettings{Enabled: false, IntervalMinutes: 60})
}

func (h *Handler) getConfig(c *gin.Context, _ params) {
	cfg, err := h.svc.Config(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *Handler) putConfig(c *gin.Context, _ params) {
	var cfg Config
	if !bind(c, &cfg) {
		return
	}
	saved, err := h.svc.SetConfig(c.Request.Context(), cfg)
	if err != nil {
		internalError(c, err)
		return
	}
	response.Success(c, saved)
}
