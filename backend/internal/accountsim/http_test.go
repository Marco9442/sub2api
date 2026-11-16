package accountsim

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/gin-gonic/gin"
)

type memStore struct {
	state *State
	cfg   Config
	saves int
}

func (m *memStore) Load(context.Context) (*State, Config, error) { return m.state, m.cfg, nil }
func (m *memStore) SaveState(_ context.Context, state *State) error {
	m.saves++
	m.state = state
	return nil
}
func (m *memStore) SaveConfig(_ context.Context, cfg Config) error {
	m.cfg = cfg
	return nil
}

var fixtureNow = time.Date(2026, 11, 15, 12, 0, 0, 0, cst)

// fixtureState 构造一个小号池：两个分组、一个代理、四个账号，覆盖正常、限流、停调度和未分组。
func fixtureState() *State {
	state := NewState()
	created := fixtureNow.Add(-48 * time.Hour)
	state.Groups = []Group{
		{ID: 1, Name: "anthropic-1", Platform: "anthropic", RateMultiplier: 1, CreatedAt: created},
		{ID: 2, Name: "openai-1", Platform: "openai", RateMultiplier: 1.2, CreatedAt: created},
	}
	state.Proxies = []Proxy{{ID: 1, Name: "hk-1", Protocol: "http", Host: "198.51.100.7", Port: 20000, Region: "hk", CreatedAt: created}}
	reset := fixtureNow.Add(30 * time.Minute)
	accounts := []*Account{
		{ID: 6601, Name: "cedar.nova12@gmail.com", Platform: "anthropic", Type: "oauth", Tier: "max", GroupIDs: []int64{1}, ProxyID: 1, Concurrency: 6, Priority: 2, Schedulable: true, Status: StatusActive},
		{ID: 6602, Name: "amber.tide7@outlook.com", Platform: "openai", Type: "oauth", Tier: "plus", GroupIDs: []int64{2}, ProxyID: 1, Concurrency: 3, Priority: 1, Schedulable: true, Status: StatusActive, RateLimitedAt: &created, RateLimitResetAt: &reset},
		{ID: 6603, Name: "deepseek-payg-042", Platform: "deepseek", Type: "apikey", Tier: "payg", Concurrency: 10, Priority: 3, Schedulable: false, Status: StatusActive},
		{ID: 6604, Name: "birch.orbit88@icloud.com", Platform: "anthropic", Type: "oauth", Tier: "pro", GroupIDs: []int64{1}, ProxyID: 1, Concurrency: 2, Priority: 4, Schedulable: false, Status: StatusError, ErrorMessage: "401 Unauthorized: token expired"},
	}
	for _, account := range accounts {
		account.RateMultiplier = 1
		account.Intensity = 1
		account.CreatedAt, account.UpdatedAt = created, created
		account.RetireAt = fixtureNow.Add(7 * 24 * time.Hour)
		account.Days = []DayUsage{{Date: fixtureNow.Format(time.DateOnly), Requests: 40, InputTokens: 1000, OutputTokens: 500, Cost: 2, ActualCost: 2, Models: map[string]int64{"claude-sonnet-5-5": 40}}}
		account.TotalRequests, account.TotalTokens, account.TotalCost, account.TotalActualCost = 40, 1500, 2, 2
		state.Accounts[account.ID] = account
	}
	state.LastID, state.LastGroupID, state.LastProxyID = 6604, 2, 1
	state.EvolvedUntil = fixtureNow
	return state
}

func newTestServer(t *testing.T) (*gin.Engine, *memStore) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	store := &memStore{state: fixtureState(), cfg: DefaultConfig()}
	svc := NewService(store, func() time.Time { return fixtureNow })
	router := gin.New()
	router.Any("/api/v1/admin/account-sim/*path", NewHandler(svc).Serve)
	return router, store
}

type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func call(t *testing.T, router *gin.Engine, method, path, body string) (int, envelope) {
	t.Helper()
	req := httptest.NewRequest(method, "/api/v1/admin/account-sim"+path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	var out envelope
	if rec.Body.Len() > 0 && strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s 响应不是 JSON: %v\n%s", method, path, err, rec.Body.String())
		}
	}
	return rec.Code, out
}

func decode[T any](t *testing.T, raw json.RawMessage) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("解析 data 失败: %v\n%s", err, raw)
	}
	return out
}

type listPage struct {
	Items    []map[string]any `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Pages    int              `json:"pages"`
}

func listIDs(t *testing.T, router *gin.Engine, query string) []int64 {
	t.Helper()
	status, resp := call(t, router, http.MethodGet, "/accounts?lite=1&"+query, "")
	if status != http.StatusOK || resp.Code != 0 {
		t.Fatalf("列表请求失败: %d %+v", status, resp)
	}
	page := decode[listPage](t, resp.Data)
	ids := make([]int64, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, int64(asFloat(t, item["id"])))
	}
	return ids
}

func TestListEnvelopeMatchesRealShape(t *testing.T) {
	router, _ := newTestServer(t)
	status, resp := call(t, router, http.MethodGet, "/accounts?page=1&page_size=2&lite=1", "")
	if status != http.StatusOK || resp.Code != 0 || resp.Message != "success" {
		t.Fatalf("信封不符: %d %+v", status, resp)
	}
	page := decode[listPage](t, resp.Data)
	if page.Total != 4 || page.Page != 1 || page.PageSize != 2 || page.Pages != 2 || len(page.Items) != 2 {
		t.Fatalf("分页不符: %+v", page)
	}
	item := page.Items[0]
	for _, key := range []string{"id", "name", "platform", "type", "credentials", "extra", "proxy_id", "concurrency", "current_concurrency", "status", "schedulable", "group_ids", "rate_limit_reset_at", "expires_at"} {
		if _, ok := item[key]; !ok {
			t.Errorf("列表项缺少字段 %s", key)
		}
	}
	if _, ok := item["groups"]; ok {
		t.Errorf("lite 列表不应包含 groups")
	}
}

func TestListFiltersAndSortsLikeRealList(t *testing.T) {
	router, _ := newTestServer(t)
	cases := []struct {
		query string
		want  []int64
	}{
		{"", []int64{6602, 6604, 6601, 6603}},
		{"sort_by=id&sort_order=desc", []int64{6604, 6603, 6602, 6601}},
		{"platform=anthropic&sort_by=id", []int64{6601, 6604}},
		{"type=apikey", []int64{6603}},
		{"status=active", []int64{6601}},
		{"status=rate_limited", []int64{6602}},
		{"status=unschedulable", []int64{6603}},
		{"status=error", []int64{6604}},
		{"group=ungrouped", []int64{6603}},
		{"group=2", []int64{6602}},
		{"search=ORBIT", []int64{6604}},
		{"sort_by=priority&sort_order=desc", []int64{6604, 6603, 6601, 6602}},
	}
	for _, tc := range cases {
		got := listIDs(t, router, tc.query)
		if !equalIDs(got, tc.want) {
			t.Errorf("%q: 期望 %v，实际 %v", tc.query, tc.want, got)
		}
	}
}

func asMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("期望对象，实际 %T", v)
	}
	return m
}

func asFloat(t *testing.T, v any) float64 {
	t.Helper()
	n, ok := v.(float64)
	if !ok {
		t.Fatalf("期望数字，实际 %T", v)
	}
	return n
}

func equalIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDetailIncludesGroupsAndProxy(t *testing.T) {
	router, _ := newTestServer(t)
	status, resp := call(t, router, http.MethodGet, "/accounts/6601", "")
	if status != http.StatusOK {
		t.Fatalf("详情失败: %d %+v", status, resp)
	}
	account := decode[map[string]any](t, resp.Data)
	if account["proxy"] == nil || account["groups"] == nil {
		t.Fatalf("详情应包含 proxy 与 groups: %v", account)
	}
	if _, ok := account["current_concurrency"]; !ok {
		t.Fatalf("详情应包含 current_concurrency")
	}

	status, resp = call(t, router, http.MethodGet, "/accounts/1", "")
	if status != http.StatusNotFound || resp.Code != http.StatusNotFound {
		t.Fatalf("不存在的账号应返回 404: %d %+v", status, resp)
	}
}

func TestGroupsAndProxiesAll(t *testing.T) {
	router, _ := newTestServer(t)
	_, resp := call(t, router, http.MethodGet, "/groups/all?platform=openai", "")
	groups := decode[[]map[string]any](t, resp.Data)
	if len(groups) != 1 || groups[0]["name"] != "openai-1" || groups[0]["status"] != "active" {
		t.Fatalf("分组筛选不符: %v", groups)
	}

	_, resp = call(t, router, http.MethodGet, "/proxies/all?with_count=true", "")
	proxies := decode[[]map[string]any](t, resp.Data)
	if len(proxies) != 1 || asFloat(t, proxies[0]["account_count"]) != 3 {
		t.Fatalf("代理账号数不符: %v", proxies)
	}
}

func TestDisplayFieldsMatchPageContract(t *testing.T) {
	router, store := newTestServer(t)
	store.state.Accounts[6601].Window5h = Window{Start: fixtureNow.Add(-time.Hour), Requests: 10, Tokens: 100, Cost: 9}
	store.state.Accounts[6601].Credentials = map[string]any{"plan_type": "team"}

	status, resp := call(t, router, http.MethodGet, "/accounts?page_size=20&lite=1", "")
	if status != http.StatusOK {
		t.Fatalf("列表失败: %d", status)
	}
	page := decode[listPage](t, resp.Data)
	byID := map[int64]map[string]any{}
	for _, item := range page.Items {
		byID[int64(asFloat(t, item["id"]))] = item
	}

	cred := asMap(t, byID[6601]["credentials"])
	if cred["plan_type"] != "team" || cred["email"] != "cedar.nova12@gmail.com" {
		t.Fatalf("手动 plan_type 应覆盖展示值，并保留邮箱: %v", cred)
	}
	openai := asMap(t, byID[6602]["credentials"])
	if openai["plan_type"] != "plus" || openai["auth_mode"] != "chatgpt" || openai["email"] != "amber.tide7@outlook.com" {
		t.Fatalf("openai 展示凭据不符: %v", openai)
	}
	deepseek := asMap(t, byID[6603]["credentials"])
	if deepseek["base_url"] != "https://api.deepseek.com" || deepseek["account_mode"] != "payg" {
		t.Fatalf("deepseek 展示凭据不符: %v", deepseek)
	}
	extra := asMap(t, byID[6603]["extra"])
	balances, _ := extra["deepseek_balances"].([]any)
	if len(balances) != 2 {
		t.Fatalf("deepseek 应有人民币和美元余额: %v", extra)
	}
}

func TestUsageAndTodayStatsShapes(t *testing.T) {
	router, store := newTestServer(t)
	store.state.Accounts[6601].Window5h = Window{Start: fixtureNow.Add(-time.Hour), Requests: 10, Tokens: 100, Cost: 9}

	status, resp := call(t, router, http.MethodPost, "/accounts/usage/batch", `{"account_ids":[6601,6602],"force":false}`)
	if status != http.StatusOK {
		t.Fatalf("用量批量失败: %d %+v", status, resp)
	}
	body := decode[map[string]any](t, resp.Data)
	usage := asMap(t, body["usage"])
	if _, ok := body["errors"].(map[string]any); !ok {
		t.Fatalf("用量响应应包含 errors: %v", body)
	}
	claude := asMap(t, usage["6601"])
	five := asMap(t, claude["five_hour"])
	if asFloat(t, five["utilization"]) != 20 {
		t.Fatalf("5 小时用量应为 20%%: %v", five)
	}
	if claude["seven_day"] == nil {
		t.Fatalf("anthropic 应包含 seven_day")
	}
	if _, ok := asMap(t, usage["6602"])["five_hour"].(map[string]any); !ok {
		t.Fatalf("openai 应包含 five_hour 对象")
	}

	status, resp = call(t, router, http.MethodPost, "/accounts/today-stats/batch", `{"account_ids":[6601]}`)
	statsBody := decode[map[string]any](t, resp.Data)
	stats := asMap(t, asMap(t, statsBody["stats"])["6601"])
	if status != http.StatusOK || asFloat(t, stats["requests"]) != 40 {
		t.Fatalf("今日统计不符: %d %v", status, stats)
	}

	status, resp = call(t, router, http.MethodGet, "/accounts/upstream-billing-probe/settings", "")
	settings := decode[map[string]any](t, resp.Data)
	if status != http.StatusOK || settings["enabled"] != false || asFloat(t, settings["interval_minutes"]) != 60 {
		t.Fatalf("探测设置不符: %d %v", status, settings)
	}
}

func TestUsageInfoCoversVisiblePlatforms(t *testing.T) {
	now := fixtureNow
	grok := usageInfo(&Account{Platform: "grok", Type: "oauth", Tier: "heavy", RateMultiplier: 1, Window7d: Window{Start: now.Add(-time.Hour), Cost: 12}}, now)
	if grok.GrokBilling == nil || grok.GrokBilling.PeriodType != "weekly" || grok.GrokBilling.Plan != "SuperGrok Heavy" || grok.GrokQuotaSnapshotState != "observed" {
		t.Fatalf("grok 用量不符: %+v", grok.GrokBilling)
	}
	if grok.GrokBilling.MonthlyLimitCents != nil {
		t.Fatal("只有周额度时不应带月额度")
	}
	gemini := usageInfo(&Account{Platform: "gemini", Type: "oauth", Tier: "ultra", RateMultiplier: 1, Days: []DayUsage{{Date: now.In(cst).Format(time.DateOnly), Cost: 5, ActualCost: 5}}}, now)
	if gemini.GeminiSharedDaily == nil || gemini.GeminiSharedDaily.Utilization <= 0 {
		t.Fatalf("gemini 日额度不符: %+v", gemini.GeminiSharedDaily)
	}
	geminiKey := usageInfo(&Account{Platform: "gemini", Type: "apikey", Tier: "pro", RateMultiplier: 1, Days: []DayUsage{{Date: now.In(cst).Format(time.DateOnly), Cost: 5, ActualCost: 5}}}, now)
	if geminiKey.GeminiSharedDaily != nil {
		t.Fatalf("gemini API Key 不应有日额度条: %+v", geminiKey.GeminiSharedDaily)
	}
	groups := usageInfo(&Account{Platform: "antigravity", Type: "oauth", Tier: "ultra", RateMultiplier: 1, Window5h: Window{Cost: 6}}, now).AntigravityQuotaGroups
	if len(groups) != 1 || len(groups[0].Windows) != 2 || groups[0].Windows[1].Window != "weekly" {
		t.Fatalf("antigravity 额度组不符: %+v", groups)
	}
	extra := displayExtra(&Account{Platform: "antigravity", Type: "oauth", Tier: "ultra", Name: "a.b1@gmail.com"}, now)
	paid := asMap(t, asMap(t, extra["load_code_assist"])["paidTier"])
	if paid["id"] != "g1-ultra-tier" {
		t.Fatalf("antigravity 等级不符: %v", extra)
	}
	kimi := displayCredentials(&Account{Name: "kimi-coding-001", Platform: "kimi", Type: "apikey", Tier: "coding"})
	if kimi["base_url"] != "https://api.kimi.com/coding/v1" || kimi["account_mode"] != "coding" {
		t.Fatalf("kimi 凭据不符: %v", kimi)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	router, store := newTestServer(t)
	status, resp := call(t, router, http.MethodGet, "/config", "")
	cfg := decode[Config](t, resp.Data)
	if status != http.StatusOK || cfg.DailyNewMin != 30 || cfg.DailyNewMax != 120 {
		t.Fatalf("默认配置不符: %d %+v", status, cfg)
	}
	cfg.DailyNewMin = 40
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	status, resp = call(t, router, http.MethodPut, "/config", string(raw))
	saved := decode[Config](t, resp.Data)
	if status != http.StatusOK || saved.DailyNewMin != 40 || store.saves != 0 {
		t.Fatalf("保存配置不应改账号状态: %d %+v saves=%d", status, saved, store.saves)
	}
	if store.cfg.DailyNewMin != 40 {
		t.Fatalf("配置未落库: %+v", store.cfg)
	}
}

func TestUnknownEndpointReturnsSafeDefault(t *testing.T) {
	router, store := newTestServer(t)
	status, resp := call(t, router, http.MethodPost, "/accounts/6601/some-new-action", `{"x":1}`)
	if status != http.StatusOK || resp.Code != 0 {
		t.Fatalf("未覆盖接口应返回成功: %d %+v", status, resp)
	}
	if store.saves != 0 {
		t.Fatalf("未覆盖接口不应写入状态")
	}
}

// statsAccount 把 6601 改成跨过 90 天窗口的账号：两天真实用量，外加创建日前的一条干扰数据。
func statsAccount(store *memStore) {
	account := store.state.Accounts[6601]
	account.CreatedAt = time.Date(2026, 9, 1, 8, 0, 0, 0, cst)
	account.RateMultiplier = 2
	account.Days = []DayUsage{
		{Date: "2026-08-15", Requests: 99999, InputTokens: 99999, Cost: 999, ActualCost: 999, Models: map[string]int64{"claude-opus-5-5": 99999}},
		{Date: "2026-11-14", Requests: 20, InputTokens: 400, OutputTokens: 100, Cost: 1, ActualCost: 3, Models: map[string]int64{"claude-sonnet-5-5": 20}},
		{Date: "2026-11-15", Requests: 40, InputTokens: 1000, OutputTokens: 500, Cost: 2, ActualCost: 6, Models: map[string]int64{"claude-sonnet-5-5": 30, "claude-opus-5-5": 10}},
	}
}

func getStats(t *testing.T, router *gin.Engine, query string) (envelope, usagestats.AccountUsageStatsResponse, map[string]json.RawMessage) {
	t.Helper()
	status, resp := call(t, router, http.MethodGet, "/accounts/6601/stats"+query, "")
	if status != http.StatusOK || resp.Code != 0 {
		t.Fatalf("统计请求失败: %d %+v", status, resp)
	}
	return resp, decode[usagestats.AccountUsageStatsResponse](t, resp.Data), decode[map[string]json.RawMessage](t, resp.Data)
}

func historyOn(t *testing.T, history []usagestats.AccountUsageHistory, date string) usagestats.AccountUsageHistory {
	t.Helper()
	var prev string
	for _, item := range history {
		if prev != "" && item.Date < prev {
			t.Fatalf("历史应按日期升序: %s 在 %s 之后", item.Date, prev)
		}
		prev = item.Date
		if item.Date == date {
			return item
		}
	}
	t.Fatalf("历史中没有 %s", date)
	return usagestats.AccountUsageHistory{}
}

func historyAbsent(t *testing.T, history []usagestats.AccountUsageHistory, date string) {
	t.Helper()
	for _, item := range history {
		if item.Date == date {
			t.Fatalf("不应包含 %s", date)
		}
	}
}

func near(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s: 期望 %v，实际 %v", name, want, got)
	}
}

func TestAccountStatsUsesStoredDaysAndEstimatesOlderRange(t *testing.T) {
	router, store := newTestServer(t)
	statsAccount(store)

	_, stats, raw := getStats(t, router, "")
	if string(raw["endpoints"]) != "[]" || string(raw["upstream_endpoints"]) != "[]" {
		t.Fatalf("端点应为数组而不是 null: %s %s", raw["endpoints"], raw["upstream_endpoints"])
	}
	if stats.Summary.Days != 31 || stats.Summary.ActualDaysUsed != 2 || len(stats.History) != 2 {
		t.Fatalf("30 天只应有已存储的两天，且天数与真实接口一样多 1: %+v len=%d", stats.Summary, len(stats.History))
	}
	historyAbsent(t, stats.History, "2026-08-15")
	historyAbsent(t, stats.History, "2026-09-01")
	historyAbsent(t, stats.History, "2026-10-17")
	day := historyOn(t, stats.History, "2026-11-15")
	if day.Label != "11/15" || day.Requests != 40 || day.Tokens != 1500 {
		t.Fatalf("当天记录应原样返回: %+v", day)
	}
	near(t, "standard", day.Cost, 2)
	near(t, "account", day.ActualCost, 6)
	near(t, "user", day.UserCost, 6)
	earlier := historyOn(t, stats.History, "2026-11-14")
	if earlier.Requests != 20 || earlier.Tokens != 500 {
		t.Fatalf("前一天记录应原样返回: %+v", earlier)
	}
	near(t, "total_cost", stats.Summary.TotalCost, 9)
	near(t, "total_user_cost", stats.Summary.TotalUserCost, 9)
	near(t, "total_standard_cost", stats.Summary.TotalStandardCost, 3)
	if stats.Summary.TotalRequests != 60 || stats.Summary.TotalTokens != 2000 {
		t.Fatalf("汇总不符: %+v", stats.Summary)
	}
	near(t, "avg_cost", stats.Summary.AvgDailyCost, 4.5)
	near(t, "avg_user", stats.Summary.AvgDailyUserCost, 4.5)
	near(t, "avg_requests", stats.Summary.AvgDailyRequests, 30)
	near(t, "avg_tokens", stats.Summary.AvgDailyTokens, 1000)
	near(t, "duration", stats.Summary.AvgDurationMs, 400+900*math.Log10(2000/60.0))
	if stats.Summary.Today == nil || stats.Summary.Today.Date != "2026-11-15" || stats.Summary.Today.Requests != 40 || stats.Summary.Today.Tokens != 1500 {
		t.Fatalf("今日不符: %+v", stats.Summary.Today)
	}
	near(t, "today_cost", stats.Summary.Today.Cost, 6)
	near(t, "today_user", stats.Summary.Today.UserCost, 6)
	if stats.Summary.HighestCostDay == nil || stats.Summary.HighestCostDay.Date != "2026-11-15" || stats.Summary.HighestRequestDay.Date != "2026-11-15" {
		t.Fatalf("峰值日不符: %+v %+v", stats.Summary.HighestCostDay, stats.Summary.HighestRequestDay)
	}
	near(t, "peak_cost", stats.Summary.HighestCostDay.Cost, 6)
	near(t, "peak_user", stats.Summary.HighestCostDay.UserCost, 6)
	if len(stats.Models) != 2 || stats.Models[0].Model != "claude-sonnet-5-5" || stats.Models[1].Model != "claude-opus-5-5" {
		t.Fatalf("模型顺序应按 token 降序: %+v", stats.Models)
	}
	sonnet, opus := stats.Models[0], stats.Models[1]
	if sonnet.Requests != 50 || sonnet.InputTokens != 1150 || sonnet.OutputTokens != 475 || sonnet.TotalTokens != 1625 {
		t.Fatalf("sonnet 分摊不符: %+v", sonnet)
	}
	near(t, "sonnet_cost", sonnet.Cost, 2.5)
	near(t, "sonnet_actual", sonnet.ActualCost, 7.5)
	near(t, "sonnet_account", sonnet.AccountCost, 7.5)
	if opus.Requests != 10 || opus.InputTokens != 250 || opus.OutputTokens != 125 || opus.TotalTokens != 375 {
		t.Fatalf("opus 分摊不符: %+v", opus)
	}
	near(t, "opus_cost", opus.Cost, 0.5)
	near(t, "opus_actual", opus.ActualCost, 1.5)
	if store.saves != 0 {
		t.Fatal("读统计不应写状态")
	}

	_, oneDay, _ := getStats(t, router, "?days=1")
	if len(oneDay.History) != 1 || oneDay.History[0].Date != "2026-11-15" || oneDay.Summary.Days != 2 {
		t.Fatalf("1 天只应包含当天: %+v days=%d", oneDay.History, oneDay.Summary.Days)
	}
	_, invalid, _ := getStats(t, router, "?days=0")
	if len(invalid.History) != 2 {
		t.Fatalf("非法天数应回落到 30 天: %d", len(invalid.History))
	}

	_, wide, _ := getStats(t, router, "?days=60")
	historyAbsent(t, wide.History, "2026-09-01")
	historyAbsent(t, wide.History, "2026-10-17")
	estimated := historyOn(t, wide.History, "2026-09-17")
	if estimated.Requests != 30 || estimated.Tokens != 1000 || estimated.Label != "09/17" {
		t.Fatalf("更早的天数应为已有天数的日均: %+v", estimated)
	}
	near(t, "estimated_standard", estimated.Cost, 1.5)
	near(t, "estimated_account", estimated.ActualCost, 4.5)
	near(t, "estimated_user", estimated.UserCost, 4.5)
	if tail := historyOn(t, wide.History, "2026-10-16"); tail.Requests != 30 {
		t.Fatalf("估算应覆盖到第 31 天之前: %+v", tail)
	}
	if kept := historyOn(t, wide.History, "2026-11-15"); kept.Requests != 40 {
		t.Fatalf("已存储的天不应被估算覆盖: %+v", kept)
	}
	var sonnetRequests int64
	for _, model := range wide.Models {
		if model.Model == "claude-sonnet-5-5" {
			sonnetRequests = model.Requests
		}
	}
	if sonnetRequests != 800 {
		t.Fatalf("60 天的模型请求应包含估算天数: %d", sonnetRequests)
	}

	_, longest, _ := getStats(t, router, "?days=90")
	historyAbsent(t, longest.History, "2026-08-31")
	historyAbsent(t, longest.History, "2026-08-15")
	if first := longest.History[0].Date; first != "2026-09-01" || longest.History[0].Requests != 30 {
		t.Fatalf("90 天应从创建日开始估算，而不是创建日之前: %+v", longest.History[0])
	}

	status, resp := call(t, router, http.MethodGet, "/accounts/1/stats", "")
	if status != http.StatusNotFound || resp.Code != http.StatusNotFound {
		t.Fatalf("不存在的账号应返回 404: %d %+v", status, resp)
	}
}

func rawCall(t *testing.T, router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/v1/admin/account-sim"+path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func sseEvents(t *testing.T, body string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &event); err != nil {
			t.Fatalf("SSE 不是 JSON: %s", line)
		}
		out = append(out, event)
	}
	return out
}

func TestSimulatedTestAndModelsDoNotTouchState(t *testing.T) {
	router, store := newTestServer(t)
	store.state.Accounts[6601].Credentials = map[string]any{"access_token": "kept"}

	rec := rawCall(t, router, http.MethodPost, "/accounts/6601/test", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("测试连接应为 SSE: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	events := sseEvents(t, rec.Body.String())
	if len(events) != 3 || events[0]["type"] != "test_start" || events[0]["model"] != "claude-sonnet-5-5" {
		t.Fatalf("应先报告开始和模型: %v", events)
	}
	if events[1]["type"] != "content" || events[1]["text"] == "" {
		t.Fatalf("应有一段回复: %v", events)
	}
	if events[2]["type"] != "test_complete" || events[2]["success"] != true {
		t.Fatalf("应报告成功: %v", events)
	}

	status, resp := call(t, router, http.MethodGet, "/accounts/6601/models", "")
	models := decode[[]map[string]any](t, resp.Data)
	if status != http.StatusOK || len(models) != len(platformModels["anthropic"]) || models[0]["id"] != "claude-sonnet-5-5" || models[0]["type"] != "model" {
		t.Fatalf("可用模型不符: %d %v", status, models)
	}

	status, resp = call(t, router, http.MethodPost, "/accounts/6601/models/sync-upstream", "")
	synced := decode[map[string]any](t, resp.Data)
	names, _ := synced["models"].([]any)
	if status != http.StatusOK || synced["discovery_source"] != "configured_fallback" || len(names) != len(models) || names[0] != "claude-sonnet-5-5" {
		t.Fatalf("同步模型不符: %d %v", status, synced)
	}
	if store.saves != 0 || store.state.Accounts[6601].Credentials["access_token"] != "kept" {
		t.Fatal("测试和同步不应改账号或凭据")
	}

	store.state.Accounts[6603].Platform = "unknown"
	status, resp = call(t, router, http.MethodGet, "/accounts/6603/models", "")
	if status != http.StatusOK || string(resp.Data) != "[]" {
		t.Fatalf("未知平台应返回空数组: %d %s", status, resp.Data)
	}
	status, resp = call(t, router, http.MethodPost, "/accounts/9/test", "")
	if status != http.StatusNotFound {
		t.Fatalf("不存在的账号不应做测试: %d %+v", status, resp)
	}
}

type failStore struct {
	memStore
}

func (f *failStore) SaveState(context.Context, *State) error {
	f.saves++
	return errors.New("db down")
}

func testRouter(store stateStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := NewService(store, func() time.Time { return fixtureNow })
	router := gin.New()
	router.Any("/api/v1/admin/account-sim/*path", NewHandler(svc).Serve)
	return router
}

func oneJSON(t *testing.T, body string) envelope {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(body))
	var env envelope
	if err := dec.Decode(&env); err != nil || dec.More() {
		t.Fatalf("响应应是一段 JSON: %v\n%s", err, body)
	}
	return env
}

func TestSaveFailureKeepsMemoryAndWritesOnce(t *testing.T) {
	store := &failStore{memStore: memStore{state: fixtureState(), cfg: DefaultConfig()}}
	original := store.state.Accounts[6601]
	rec := rawCall(t, testRouter(store), http.MethodPut, "/accounts/6601", `{"name":"renamed"}`)
	env := oneJSON(t, rec.Body.String())
	if rec.Code != http.StatusInternalServerError || env.Code != http.StatusInternalServerError {
		t.Fatalf("保存失败应为 500: %d %+v", rec.Code, env)
	}
	if store.state.Accounts[6601] != original || original.Name != "cedar.nova12@gmail.com" {
		t.Fatal("保存失败不应改内存")
	}
	status, resp := call(t, testRouter(store), http.MethodGet, "/accounts/6601", "")
	detail := decode[map[string]any](t, resp.Data)
	if status != http.StatusOK || detail["name"] != "cedar.nova12@gmail.com" {
		t.Fatalf("读取仍应是原账号: %d %v", status, detail["name"])
	}
}

func TestSuccessfulWriteSwapsOnlyAfterSave(t *testing.T) {
	router, store := newTestServer(t)
	original := store.state.Accounts[6601]
	status, _ := call(t, router, http.MethodPut, "/accounts/6601", `{"name":"renamed"}`)
	if status != http.StatusOK || store.saves != 1 {
		t.Fatalf("保存应成功一次: %d saves=%d", status, store.saves)
	}
	if original.Name == "renamed" || store.state.Accounts[6601] == original || store.state.Accounts[6601].Name != "renamed" {
		t.Fatalf("成功后应换成副本: before=%q after=%q", original.Name, store.state.Accounts[6601].Name)
	}
}

func TestBulkUpdateWithoutIDsDoesNotSave(t *testing.T) {
	router, store := newTestServer(t)
	rec := rawCall(t, router, http.MethodPost, "/accounts/bulk-update", `{}`)
	env := oneJSON(t, rec.Body.String())
	if rec.Code != http.StatusBadRequest || env.Code != http.StatusBadRequest || store.saves != 0 {
		t.Fatalf("缺少 id 应拒绝且不落库: %d %+v saves=%d", rec.Code, env, store.saves)
	}
}

func TestCreateRejectsUnsupportedPlatform(t *testing.T) {
	router, store := newTestServer(t)
	status, resp := call(t, router, http.MethodPost, "/accounts", `{"name":"x","platform":"unknown","type":"apikey","credentials":{"api_key":"k"}}`)
	if status != http.StatusBadRequest || store.saves != 0 || !strings.Contains(resp.Message, "Unsupported") {
		t.Fatalf("未知平台应拒绝且不落库: %d %+v saves=%d", status, resp, store.saves)
	}
	p := profileOf("unknown", "apikey", "")
	if p.Cap5h != 0 || p.Cap7d != 0 || p.CapDaily != 0 {
		t.Fatalf("未知平台不应有额度窗口: %+v", p)
	}
	_ = usageInfo(&Account{Platform: "unknown", Type: "apikey", RateMultiplier: 1}, fixtureNow)
}

func TestManualRestoreClearsRetirementAndResumesUsage(t *testing.T) {
	router, store := newTestServer(t)
	retired := fixtureNow.Add(-time.Hour)
	account := store.state.Accounts[6601]
	account.Status = StatusError
	account.ErrorMessage = "401"
	account.Schedulable = false
	account.RetiredAt = &retired
	account.DeleteAt = fixtureNow.Add(24 * time.Hour)
	account.RetireOnErr = true

	status, _ := call(t, router, http.MethodPost, "/accounts/6601/clear-error", `{}`)
	restored := store.state.Accounts[6601]
	if status != http.StatusOK || restored.Status != StatusActive || restored.RetiredAt != nil || !restored.DeleteAt.IsZero() || restored.RetireOnErr || !restored.Pinned["status"] || restored.Schedulable {
		t.Fatalf("清除错误应取消淘汰并保持手动接管: status=%d account=%+v", status, restored)
	}
	if serving(restored, fixtureNow) {
		t.Fatal("未打开调度时不应接单")
	}

	heldAccount := store.state.Accounts[6603]
	heldAccount.Status = StatusInactive
	heldAccount.RetiredAt = &retired
	heldAccount.DeleteAt = fixtureNow.Add(24 * time.Hour)
	status, _ = call(t, router, http.MethodPost, "/accounts/6603/schedulable", `{"schedulable":true}`)
	held := store.state.Accounts[6603]
	if status != http.StatusOK || held.RetiredAt == nil || !held.Schedulable || serving(held, fixtureNow) {
		t.Fatalf("停用账号打开调度不应取消淘汰: %d retired=%v", status, held.RetiredAt != nil)
	}

	status, _ = call(t, router, http.MethodPost, "/accounts/6601/schedulable", `{"schedulable":true}`)
	resumed := store.state.Accounts[6601]
	if status != http.StatusOK || !resumed.Schedulable || resumed.RetiredAt != nil || !resumed.Pinned["status"] || !serving(resumed, fixtureNow) {
		t.Fatalf("正常账号打开调度后应可接单: %d %+v", status, resumed)
	}
	before := resumed.TotalCost
	consume(resumed, fixtureNow, newRNG(resumed.ID, fixtureNow.Unix()), DefaultConfig())
	if resumed.TotalCost <= before {
		t.Fatal("打开调度后用量应增加")
	}
}

func TestEditingActiveClearsRetirement(t *testing.T) {
	router, store := newTestServer(t)
	retired := fixtureNow.Add(-time.Hour)
	account := store.state.Accounts[6604]
	account.RetiredAt = &retired
	account.DeleteAt = fixtureNow.Add(48 * time.Hour)
	account.RetireOnErr = true
	status, _ := call(t, router, http.MethodPut, "/accounts/6604", `{"status":"active"}`)
	got := store.state.Accounts[6604]
	if status != http.StatusOK || got.Status != StatusActive || got.RetiredAt != nil || !got.DeleteAt.IsZero() || got.RetireOnErr || !got.Pinned["status"] {
		t.Fatalf("改回正常应取消淘汰: %d %+v", status, got)
	}
}

func TestDisplayedCostUsesStoredValue(t *testing.T) {
	day := fixtureNow.In(cst).Format(time.DateOnly)
	account := &Account{
		Platform: "anthropic", Type: "oauth", Tier: "max", RateMultiplier: 4,
		Days:     []DayUsage{{Date: day, Requests: 3, Cost: 2, ActualCost: 5}},
		Window5h: Window{Start: fixtureNow.Add(-time.Hour), Requests: 3, Tokens: 10, Cost: 2},
	}
	stats := todayStats(account, fixtureNow)
	if stats.Cost != 5 || stats.UserCost != 5 || stats.StandardCost != 2 {
		t.Fatalf("今日费用应读保存值: %+v", stats)
	}
	window := windowStatsOf(account.Window5h)
	if window.Cost != 2 || window.UserCost != 2 || window.StandardCost != 2 {
		t.Fatalf("旧窗口应回退标准费用: %+v", window)
	}
	account.Window5h.ActualCost = 7
	window = windowStatsOf(account.Window5h)
	if window.Cost != 7 || window.UserCost != 7 {
		t.Fatalf("新窗口应读保存费用: %+v", window)
	}
}
