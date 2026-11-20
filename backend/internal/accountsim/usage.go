package accountsim

// modelPrice 是每百万 Token 的标准价格（美元）。
type modelPrice struct {
	Name       string
	Weight     int
	Input      float64
	Output     float64
	CacheRead  float64
	CacheWrite float64
}

// profile 描述一个平台与订阅等级的额度和用量特征。
// 额度以标准费用计，0 表示没有该窗口。
type profile struct {
	Cap5h    float64
	Cap7d    float64
	CapDaily float64
	// Balance 是按量账号的初始余额，0 表示不展示余额。
	Balance float64
	// HourlyCost 是强度为 1 时高峰小时的标准费用。
	HourlyCost  float64
	Concurrency [2]int
	// CacheHeavy 表示请求以缓存读取为主（编程类客户端）。
	CacheHeavy bool
}

var (
	claudeModels = []modelPrice{
		{"claude-sonnet-5-5", 55, 3, 15, 0.3, 3.75},
		{"claude-opus-5-5", 20, 5, 25, 0.5, 6.25},
		{"claude-fable-5-1", 10, 5, 25, 0.5, 6.25},
		{"claude-haiku-5-5", 15, 1, 5, 0.1, 1.25},
	}
	gptModels = []modelPrice{
		{"gpt-5.5", 50, 1.25, 10, 0.125, 0},
		{"gpt-5.6-sol", 25, 1.5, 12, 0.15, 0},
		{"gpt-5.3-codex", 25, 1.25, 10, 0.125, 0},
	}
	geminiModels = []modelPrice{
		{"gemini-3.1-pro", 55, 2, 12, 0.2, 0},
		{"gemini-3.8-flash", 45, 0.3, 2.5, 0.03, 0},
	}
	grokModels = []modelPrice{
		{"grok-4.7", 70, 3, 15, 0.75, 0},
		{"grok-4.6", 30, 2, 10, 0.5, 0},
	}
	kimiModels     = []modelPrice{{"kimi-k3", 100, 0.6, 2.5, 0.15, 0}}
	glmModels      = []modelPrice{{"glm-5.3", 80, 0.6, 2.2, 0.11, 0}, {"glm-4.7", 20, 0.4, 1.6, 0.08, 0}}
	deepseekModels = []modelPrice{{"deepseek-v4-pro", 60, 0.55, 2.2, 0.07, 0}, {"deepseek-v4-flash", 40, 0.27, 1.1, 0.07, 0}}
	minimaxModels  = []modelPrice{{"MiniMax-M3", 100, 0.3, 1.2, 0.03, 0}}
	mixedModels    = []modelPrice{
		{"claude-sonnet-5-5", 50, 3, 15, 0.3, 3.75},
		{"gpt-5.5", 30, 1.25, 10, 0.125, 0},
		{"gemini-3.1-pro", 20, 2, 12, 0.2, 0},
	}
)

var platformModels = map[string][]modelPrice{
	"anthropic":    claudeModels,
	"openai":       gptModels,
	"gemini":       geminiModels,
	"antigravity":  append(append([]modelPrice{}, geminiModels...), claudeModels[0]),
	"grok":         grokModels,
	"kimi":         kimiModels,
	"zhipu":        glmModels,
	"deepseek":     deepseekModels,
	"minimax":      minimaxModels,
	"typesafe":     mixedModels,
	"opencode_go":  mixedModels,
	"command_code": mixedModels,
	"cline":        mixedModels,
}

var profiles = map[string]profile{
	"anthropic/pro":     {Cap5h: 9, Cap7d: 70, HourlyCost: 1.1, Concurrency: [2]int{2, 4}, CacheHeavy: true},
	"anthropic/max":     {Cap5h: 45, Cap7d: 350, HourlyCost: 5.4, Concurrency: [2]int{4, 10}, CacheHeavy: true},
	"anthropic/team":    {Cap5h: 15, Cap7d: 120, HourlyCost: 1.8, Concurrency: [2]int{3, 6}, CacheHeavy: true},
	"openai/plus":       {Cap5h: 6, Cap7d: 45, HourlyCost: 0.72, Concurrency: [2]int{2, 4}, CacheHeavy: true},
	"openai/pro":        {Cap5h: 60, Cap7d: 450, HourlyCost: 7.2, Concurrency: [2]int{4, 10}, CacheHeavy: true},
	"openai/team":       {Cap5h: 10, Cap7d: 80, HourlyCost: 1.2, Concurrency: [2]int{3, 6}, CacheHeavy: true},
	"gemini/pro":        {CapDaily: 25, HourlyCost: 1.2, Concurrency: [2]int{2, 5}},
	"gemini/ultra":      {CapDaily: 80, HourlyCost: 3.8, Concurrency: [2]int{4, 8}},
	"antigravity/pro":   {Cap5h: 20, Cap7d: 150, HourlyCost: 2.4, Concurrency: [2]int{2, 5}, CacheHeavy: true},
	"antigravity/ultra": {Cap5h: 60, Cap7d: 450, HourlyCost: 7, Concurrency: [2]int{4, 8}, CacheHeavy: true},
	"grok/supergrok":    {Cap7d: 25, HourlyCost: 0.25, Concurrency: [2]int{2, 4}},
	"grok/heavy":        {Cap7d: 60, HourlyCost: 0.6, Concurrency: [2]int{2, 5}},
	"kimi/coding":       {Cap5h: 3, Cap7d: 25, HourlyCost: 0.36, Concurrency: [2]int{3, 8}, CacheHeavy: true},
	"kimi/payg":         {Balance: 200, HourlyCost: 0.4, Concurrency: [2]int{5, 20}},
	"zhipu/coding":      {Cap5h: 3, Cap7d: 25, HourlyCost: 0.36, Concurrency: [2]int{3, 8}, CacheHeavy: true},
	"zhipu/payg":        {HourlyCost: 0.4, Concurrency: [2]int{5, 20}},
	"deepseek/payg":     {Balance: 300, HourlyCost: 0.5, Concurrency: [2]int{5, 30}},
	"minimax/coding":    {Cap5h: 4, Cap7d: 30, HourlyCost: 0.4, Concurrency: [2]int{3, 8}, CacheHeavy: true},
	"minimax/payg":      {HourlyCost: 0.3, Concurrency: [2]int{5, 15}},
	"typesafe/standard": {HourlyCost: 0.3, Concurrency: [2]int{3, 8}},
	"opencode_go/go":    {Cap5h: 12, Cap7d: 30, HourlyCost: 0.5, Concurrency: [2]int{2, 5}},
	"opencode_go/zen":   {HourlyCost: 0.5, Concurrency: [2]int{2, 5}},
	"command_code/payg": {Balance: 150, HourlyCost: 0.8, Concurrency: [2]int{2, 6}},
	"cline/payg":        {Balance: 150, HourlyCost: 0.5, Concurrency: [2]int{3, 8}},
}

func profileOf(platform, accountType, tier string) profile {
	p, ok := profiles[platform+"/"+tier]
	if !ok {
		p = profile{HourlyCost: 0.5, Concurrency: [2]int{2, 5}}
	}
	// 以订阅为主的平台上，API Key 账号按量计费，没有额度窗口。
	// 未知平台没有类型表，不能按下标取，否则后台演进会退出进程。
	types := platformTypes[platform]
	if accountType == "apikey" && (len(types) == 0 || types[0] != "apikey") {
		p.Cap5h, p.Cap7d, p.CapDaily = 0, 0, 0
	}
	return p
}

func pickModel(r *rng, platform string) modelPrice {
	models := platformModels[platform]
	if len(models) == 0 {
		models = mixedModels
	}
	total := 0
	for _, m := range models {
		total += m.Weight
	}
	roll := r.intn(total)
	for _, m := range models {
		roll -= m.Weight
		if roll < 0 {
			return m
		}
	}
	return models[0]
}

// requestShape 生成单个请求的 Token 构成，并返回该请求的标准费用。
func requestShape(r *rng, m modelPrice, cacheHeavy bool) (in, out, cacheRead, cacheWrite int64, cost float64) {
	in = int64(r.between(600, 5000))
	out = int64(r.between(200, 2400))
	if cacheHeavy {
		cacheRead = int64(r.between(8000, 70000))
		if m.CacheWrite > 0 {
			cacheWrite = int64(r.between(0, 6000))
		}
	} else {
		cacheRead = int64(r.between(0, 4000))
	}
	cost = (float64(in)*m.Input + float64(out)*m.Output +
		float64(cacheRead)*m.CacheRead + float64(cacheWrite)*m.CacheWrite) / 1e6
	return
}
