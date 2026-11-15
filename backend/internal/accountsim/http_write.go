package accountsim

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

var errAccountNotFound = errors.New("account not found")

func (h *Handler) registerWriteRoutes() {
	h.handle(http.MethodPost, "accounts", h.create)
	h.handle(http.MethodPut, "accounts/:id", h.update)
	h.handle(http.MethodDelete, "accounts/:id", h.remove)
	h.handle(http.MethodPost, "accounts/:id/duplicate", h.duplicate)
	h.handle(http.MethodPost, "accounts/:id/schedulable", h.setSchedulable)
	h.handle(http.MethodPost, "accounts/:id/clear-error", h.mutate(clearError))
	h.handle(http.MethodPost, "accounts/:id/clear-rate-limit", h.mutate(clearRateLimit))
	h.handle(http.MethodPost, "accounts/:id/recover-state", h.mutate(recoverState))
	h.handle(http.MethodPost, "accounts/:id/reset-quota", h.mutate(resetQuota))
	h.handle(http.MethodPost, "accounts/:id/refresh", h.mutate(refreshCredentials))
	h.handle(http.MethodPost, "accounts/:id/refresh-tier", h.mutate(touch))
	h.handle(http.MethodPost, "accounts/:id/set-privacy", h.mutate(touch))
	h.handle(http.MethodPost, "accounts/:id/apply-oauth-credentials", h.mutate(refreshCredentials))
	h.handle(http.MethodPost, "accounts/:id/revert-proxy-fallback", h.mutate(touch))
	h.handle(http.MethodGet, "accounts/:id/temp-unschedulable", h.tempUnschedulable)
	h.handle(http.MethodDelete, "accounts/:id/temp-unschedulable", h.clearTempUnschedulable)
	h.handle(http.MethodPost, "accounts/batch-delete", h.batchDelete)
	h.handle(http.MethodPost, "accounts/batch-clear-error", h.batch(clearError))
	h.handle(http.MethodPost, "accounts/batch-refresh", h.batch(refreshCredentials))
	h.handle(http.MethodPost, "accounts/bulk-update", h.bulkUpdate)
	h.handle(http.MethodPost, "accounts/check-mixed-channel", func(c *gin.Context, _ params) {
		response.Success(c, gin.H{"has_risk": false})
	})
}

// write 在写锁下执行 fn，成功后落库。fn 返回的错误已由 fn 自己写入响应时返回 errHandled。
func (h *Handler) write(c *gin.Context, fn func(v view) error) {
	err := h.svc.Update(c.Request.Context(), func(state *State, cfg Config, now time.Time) error {
		return fn(view{state: state, cfg: cfg, now: now})
	})
	switch {
	case err == nil, errors.Is(err, errHandled):
	case errors.Is(err, errAccountNotFound):
		notFound(c)
	default:
		internalError(c, err)
	}
}

// errHandled 表示回调已写入响应且不应落库。
var errHandled = errors.New("handled")

// mutate 把对单个账号的操作包装成接口：找到账号、执行、返回更新后的详情。
func (h *Handler) mutate(op func(a *Account, now time.Time)) func(c *gin.Context, p params) {
	return func(c *gin.Context, p params) {
		id, ok := p.id(c)
		if !ok {
			return
		}
		h.write(c, func(v view) error {
			a := v.state.Accounts[id]
			if a == nil {
				return errAccountNotFound
			}
			op(a, v.now)
			response.Success(c, v.detail(a))
			return nil
		})
	}
}

func touch(a *Account, now time.Time) { a.UpdatedAt = now }

// clearError 与真实操作一致：恢复为 active 并清空错误信息。手动恢复后不再自动淘汰。
func clearError(a *Account, now time.Time) {
	a.Status, a.ErrorMessage = StatusActive, ""
	a.RecoverAt, a.RetireOnErr = nil, false
	a.Pin("status")
	a.UpdatedAt = now
}

func clearRateLimit(a *Account, now time.Time) {
	a.RateLimitedAt, a.RateLimitResetAt = nil, nil
	a.UpdatedAt = now
}

// recoverState 清除限流、过载、临时不可调度和可恢复的错误。
func recoverState(a *Account, now time.Time) {
	clearRateLimit(a, now)
	a.OverloadUntil, a.TempUnschedUntil, a.TempUnschedReason = nil, nil, ""
	if a.Status == StatusError {
		clearError(a, now)
	}
}

func resetQuota(a *Account, now time.Time) {
	a.Window5h, a.Window7d = Window{}, Window{}
	clearRateLimit(a, now)
}

// refreshCredentials 模拟刷新令牌成功：授权类错误随之恢复。
func refreshCredentials(a *Account, now time.Time) {
	if a.Status == StatusError && a.RetiredAt == nil {
		clearError(a, now)
	}
	a.UpdatedAt = now
}

func (h *Handler) create(c *gin.Context, _ params) {
	var req struct {
		Name               string         `json:"name" binding:"required"`
		Notes              *string        `json:"notes"`
		Platform           string         `json:"platform" binding:"required"`
		Type               string         `json:"type" binding:"required,oneof=oauth setup-token apikey upstream bedrock service_account"`
		Credentials        map[string]any `json:"credentials" binding:"required"`
		Extra              map[string]any `json:"extra"`
		ProxyID            *int64         `json:"proxy_id"`
		Concurrency        int            `json:"concurrency"`
		Priority           int            `json:"priority"`
		RateMultiplier     *float64       `json:"rate_multiplier"`
		GroupIDs           []int64        `json:"group_ids"`
		ExpiresAt          *int64         `json:"expires_at"`
		AutoPauseOnExpired *bool          `json:"auto_pause_on_expired"`
	}
	if !bind(c, &req) {
		return
	}
	h.write(c, func(v view) error {
		v.state.LastID++
		a := &Account{
			ID: v.state.LastID, Name: req.Name, Platform: req.Platform, Type: req.Type,
			Tier:        defaultTier(req.Platform),
			Concurrency: req.Concurrency, Priority: req.Priority, RateMultiplier: 1,
			Schedulable: true, Status: StatusActive,
			CreatedAt: v.now, UpdatedAt: v.now, Intensity: 1,
			Credentials: maskSecrets(req.Credentials), Extra: req.Extra,
			GroupIDs: existingGroups(v.state, req.GroupIDs), AutoPauseOnExpired: true,
		}
		if req.Notes != nil {
			a.Notes = *req.Notes
		}
		if req.RateMultiplier != nil {
			a.RateMultiplier = *req.RateMultiplier
		}
		if req.ProxyID != nil && findProxy(v.state, *req.ProxyID) != nil {
			a.ProxyID = *req.ProxyID
		}
		if req.ExpiresAt != nil && *req.ExpiresAt > 0 {
			expires := time.Unix(*req.ExpiresAt, 0)
			a.ExpiresAt = &expires
		}
		if req.AutoPauseOnExpired != nil {
			a.AutoPauseOnExpired = *req.AutoPauseOnExpired
		}
		// 管理员创建的账号不参与自动淘汰，字段也不被自动运营改写。
		a.Pin("created")
		v.state.Accounts[a.ID] = a
		response.Success(c, v.detail(a))
		return nil
	})
}

func defaultTier(platform string) string {
	if tiers := platformTiers[platform]; len(tiers) > 0 {
		return tiers[0].name
	}
	return ""
}

// maskSecrets 把敏感凭据替换为占位值。只保留"已配置"这一事实，真实密钥不落库。
func maskSecrets(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if service.IsSensitiveCredentialKey(k) {
			if s, ok := v.(string); ok && s == "" {
				continue
			}
			v = true
		}
		out[k] = v
	}
	return out
}

func existingGroups(state *State, ids []int64) []int64 {
	out := []int64{}
	for _, id := range ids {
		if findGroup(state, id) != nil && !containsID(out, id) {
			out = append(out, id)
		}
	}
	return out
}

type updateRequest struct {
	Name               string         `json:"name"`
	Notes              *string        `json:"notes"`
	Type               string         `json:"type" binding:"omitempty,oneof=oauth setup-token apikey upstream bedrock service_account"`
	Credentials        map[string]any `json:"credentials"`
	Extra              map[string]any `json:"extra"`
	ProxyID            *int64         `json:"proxy_id"`
	Concurrency        *int           `json:"concurrency"`
	Priority           *int           `json:"priority"`
	RateMultiplier     *float64       `json:"rate_multiplier"`
	Status             string         `json:"status" binding:"omitempty,oneof=active inactive error"`
	Schedulable        *bool          `json:"schedulable"`
	GroupIDs           *[]int64       `json:"group_ids"`
	ExpiresAt          *int64         `json:"expires_at"`
	AutoPauseOnExpired *bool          `json:"auto_pause_on_expired"`
}

// apply 写入请求里出现的字段并标记为手动设置，之后自动运营不再改写这些字段。
func (req updateRequest) apply(a *Account, state *State, now time.Time) {
	if req.Name != "" {
		a.Name = req.Name
		a.Pin("name")
	}
	if req.Notes != nil {
		a.Notes = *req.Notes
	}
	if req.Type != "" {
		a.Type = req.Type
	}
	if req.Credentials != nil {
		a.Credentials = maskSecrets(service.MergePreservingSensitiveCreds(merged(displayCredentials(a), a.Credentials), req.Credentials))
	}
	if req.Extra != nil {
		a.Extra = req.Extra
	}
	if req.ProxyID != nil {
		a.ProxyID = 0
		if findProxy(state, *req.ProxyID) != nil {
			a.ProxyID = *req.ProxyID
		}
		a.Pin("proxy")
	}
	if req.Concurrency != nil {
		a.Concurrency = *req.Concurrency
	}
	if req.Priority != nil {
		a.Priority = *req.Priority
	}
	if req.RateMultiplier != nil {
		a.RateMultiplier = *req.RateMultiplier
	}
	if req.Status != "" {
		a.Status = req.Status
		if req.Status == StatusActive {
			a.ErrorMessage = ""
		}
		a.Pin("status")
	}
	if req.Schedulable != nil {
		a.Schedulable = *req.Schedulable
		a.Pin("schedulable")
	}
	if req.GroupIDs != nil {
		a.GroupIDs = existingGroups(state, *req.GroupIDs)
		a.Pin("groups")
	}
	if req.ExpiresAt != nil {
		a.ExpiresAt = nil
		if *req.ExpiresAt > 0 {
			expires := time.Unix(*req.ExpiresAt, 0)
			a.ExpiresAt = &expires
		}
		a.Pin("expires")
	}
	if req.AutoPauseOnExpired != nil {
		a.AutoPauseOnExpired = *req.AutoPauseOnExpired
	}
	a.UpdatedAt = now
}

func (h *Handler) update(c *gin.Context, p params) {
	id, ok := p.id(c)
	if !ok {
		return
	}
	var req updateRequest
	if !bind(c, &req) {
		return
	}
	req.Schedulable = nil // 真实编辑接口不接受 schedulable，单独走 /schedulable。
	h.write(c, func(v view) error {
		a := v.state.Accounts[id]
		if a == nil {
			return errAccountNotFound
		}
		req.apply(a, v.state, v.now)
		response.Success(c, v.detail(a))
		return nil
	})
}

func (h *Handler) remove(c *gin.Context, p params) {
	id, ok := p.id(c)
	if !ok {
		return
	}
	h.write(c, func(v view) error {
		if v.state.Accounts[id] == nil {
			return errAccountNotFound
		}
		delete(v.state.Accounts, id)
		response.Success(c, gin.H{"message": "Account deleted successfully"})
		return nil
	})
}

func (h *Handler) duplicate(c *gin.Context, p params) {
	id, ok := p.id(c)
	if !ok {
		return
	}
	h.write(c, func(v view) error {
		src := v.state.Accounts[id]
		if src == nil {
			return errAccountNotFound
		}
		v.state.LastID++
		dup := cloneAccount(src)
		dup.ID, dup.Name = v.state.LastID, src.Name+" (copy)"
		dup.CreatedAt, dup.UpdatedAt, dup.LastUsedAt = v.now, v.now, nil
		dup.Window5h, dup.Window7d, dup.Days = Window{}, Window{}, nil
		dup.TotalRequests, dup.TotalTokens, dup.TotalCost, dup.TotalActualCost = 0, 0, 0, 0
		dup.RetiredAt = nil
		dup.Pin("created")
		v.state.Accounts[dup.ID] = dup
		response.Success(c, v.detail(dup))
		return nil
	})
}

func cloneAccount(a *Account) *Account {
	out := *a
	out.GroupIDs = append([]int64{}, a.GroupIDs...)
	out.Credentials = merged(nil, a.Credentials)
	out.Extra = merged(nil, a.Extra)
	out.Pinned = map[string]bool{}
	for k, v := range a.Pinned {
		out.Pinned[k] = v
	}
	return &out
}

func (h *Handler) setSchedulable(c *gin.Context, p params) {
	id, ok := p.id(c)
	if !ok {
		return
	}
	var req struct {
		Schedulable bool `json:"schedulable"`
	}
	if !bind(c, &req) {
		return
	}
	h.write(c, func(v view) error {
		a := v.state.Accounts[id]
		if a == nil {
			return errAccountNotFound
		}
		a.Schedulable = req.Schedulable
		a.Pin("schedulable")
		a.UpdatedAt = v.now
		response.Success(c, v.detail(a))
		return nil
	})
}

func (h *Handler) tempUnschedulable(c *gin.Context, p params) {
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
		until := activeUntil(a.TempUnschedUntil, v.now)
		if until == nil {
			response.Success(c, gin.H{"active": false})
			return
		}
		response.Success(c, gin.H{"active": true, "state": gin.H{
			"until_unix": until.Unix(), "triggered_at_unix": a.UpdatedAt.Unix(), "error_message": a.TempUnschedReason,
		}})
	})
}

func (h *Handler) clearTempUnschedulable(c *gin.Context, p params) {
	id, ok := p.id(c)
	if !ok {
		return
	}
	h.write(c, func(v view) error {
		a := v.state.Accounts[id]
		if a == nil {
			return errAccountNotFound
		}
		a.TempUnschedUntil, a.TempUnschedReason = nil, ""
		response.Success(c, gin.H{"message": "Temp unschedulable cleared successfully"})
		return nil
	})
}

type idsRequest struct {
	AccountIDs []int64 `json:"account_ids"`
}

func bindIDs(c *gin.Context) ([]int64, bool) {
	var req idsRequest
	if !bind(c, &req) {
		return nil, false
	}
	if len(req.AccountIDs) == 0 {
		response.BadRequest(c, "account_ids is required")
		return nil, false
	}
	return req.AccountIDs, true
}

func (h *Handler) batchDelete(c *gin.Context, _ params) {
	ids, ok := bindIDs(c)
	if !ok {
		return
	}
	h.write(c, func(v view) error {
		success, failed := []int64{}, []int64{}
		errs := []gin.H{}
		for _, id := range ids {
			if v.state.Accounts[id] == nil {
				failed = append(failed, id)
				errs = append(errs, gin.H{"account_id": id, "error": errAccountNotFound.Error()})
				continue
			}
			delete(v.state.Accounts, id)
			success = append(success, id)
		}
		response.Success(c, gin.H{
			"total": len(ids), "success": len(success), "failed": len(failed),
			"success_ids": success, "failed_ids": failed, "errors": errs,
		})
		return nil
	})
}

// batch 把单账号操作包装成批量接口，响应与真实 batch-clear-error / batch-refresh 一致。
func (h *Handler) batch(op func(a *Account, now time.Time)) func(c *gin.Context, p params) {
	return func(c *gin.Context, _ params) {
		ids, ok := bindIDs(c)
		if !ok {
			return
		}
		h.write(c, func(v view) error {
			success := 0
			errs := []gin.H{}
			for _, id := range ids {
				a := v.state.Accounts[id]
				if a == nil {
					errs = append(errs, gin.H{"account_id": id, "error": errAccountNotFound.Error()})
					continue
				}
				op(a, v.now)
				success++
			}
			response.Success(c, gin.H{"total": len(ids), "success": success, "failed": len(errs), "errors": errs})
			return nil
		})
	}
}

func (h *Handler) bulkUpdate(c *gin.Context, _ params) {
	var req struct {
		updateRequest
		AccountIDs []int64 `json:"account_ids"`
		Filters    *struct {
			Platform    string `json:"platform"`
			Type        string `json:"type"`
			Status      string `json:"status"`
			Group       string `json:"group"`
			Search      string `json:"search"`
			PrivacyMode string `json:"privacy_mode"`
		} `json:"filters"`
	}
	if !bind(c, &req) {
		return
	}
	h.write(c, func(v view) error {
		ids := req.AccountIDs
		if len(ids) == 0 && req.Filters != nil {
			f := listFilter{
				platform: req.Filters.Platform, accountType: req.Filters.Type, status: req.Filters.Status,
				search: strings.TrimSpace(req.Filters.Search), privacyMode: req.Filters.PrivacyMode,
			}
			if req.Filters.Group == "ungrouped" {
				f.groupID = groupUngrouped
			} else if g, ok := parseInt64(req.Filters.Group); ok {
				f.groupID = g
			}
			for _, id := range sortedIDs(v.state.Accounts) {
				if f.match(v.state.Accounts[id], v.now) {
					ids = append(ids, id)
				}
			}
		}
		if len(ids) == 0 {
			response.BadRequest(c, "account_ids or filters is required")
			return errHandled
		}
		success, failed := []int64{}, []int64{}
		results := []gin.H{}
		for _, id := range ids {
			a := v.state.Accounts[id]
			if a == nil {
				failed = append(failed, id)
				results = append(results, gin.H{"account_id": id, "success": false, "error": errAccountNotFound.Error()})
				continue
			}
			req.apply(a, v.state, v.now)
			success = append(success, id)
			results = append(results, gin.H{"account_id": id, "success": true})
		}
		response.Success(c, gin.H{
			"success": len(success), "failed": len(failed),
			"success_ids": success, "failed_ids": failed, "results": results,
		})
		return nil
	})
}
