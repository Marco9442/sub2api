package accountsim

import (
	"fmt"
	"strings"
	"time"
)

var (
	nameWords   = []string{"river", "maple", "cedar", "ember", "nova", "orbit", "pixel", "quartz", "sable", "tide", "willow", "zephyr", "atlas", "birch", "coral", "delta", "fable", "harbor", "indigo", "juniper"}
	nameDomains = []string{"gmail.com", "outlook.com", "icloud.com", "proton.me", "hotmail.com", "yahoo.com"}
)

// newAccount 按配置生成一个自动运营账号。同一 ID 和创建时间永远得到同一账号。
func newAccount(id int64, createdAt time.Time, cfg Config) *Account {
	r := newRNG(id, createdAt.Unix())
	platform := r.pick(cfg.PlatformWeight)
	accountType := r.pick(filterPrefix(cfg.TypeWeight, platform))
	tier := r.pick(filterPrefix(cfg.TierWeight, platform))
	if !Supported(platform, accountType, tier) {
		// 配置里的组合全部为 0 时退回平台的默认组合。
		platform = "anthropic"
		accountType = platformTypes[platform][0]
		tier = platformTiers[platform][0].name
	}
	prof := profileOf(platform, accountType, tier)

	account := &Account{
		ID:             id,
		Name:           accountName(r, platform, accountType, tier),
		Platform:       platform,
		Type:           accountType,
		Tier:           tier,
		Concurrency:    r.between(prof.Concurrency[0], prof.Concurrency[1]),
		Priority:       r.between(1, 5),
		RateMultiplier: 1,
		Schedulable:    true,
		Status:         StatusActive,
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
		Intensity:      cfg.IntensityMin + r.float()*(cfg.IntensityMax-cfg.IntensityMin),
	}
	lifetime := time.Duration(r.between(cfg.LifetimeDaysMin*24, cfg.LifetimeDaysMax*24+23)) * time.Hour
	account.RetireAt = createdAt.Add(lifetime)
	if r.intn(100) < cfg.ExpiresPercent {
		// 到期时间落在寿命的 60%～120% 之间，部分账号会先到期后淘汰。
		expires := createdAt.Add(time.Duration(float64(lifetime) * (0.6 + 0.6*r.float()))).Truncate(time.Hour)
		account.ExpiresAt = &expires
		account.AutoPauseOnExpired = true
	}
	return account
}

func accountName(r *rng, platform, accountType, tier string) string {
	if accountType == "oauth" || accountType == "setup-token" {
		return fmt.Sprintf("%s.%s%d@%s", r.choose(nameWords), r.choose(nameWords), r.between(10, 9999), r.choose(nameDomains))
	}
	return fmt.Sprintf("%s-%s-%03d", platform, tier, r.between(1, 999))
}

// filterPrefix 取出 "平台/xxx" 形式的权重，返回 xxx 到权重的映射。
func filterPrefix(weights map[string]int, platform string) map[string]int {
	out := map[string]int{}
	prefix := platform + "/"
	for key, weight := range weights {
		if rest, ok := strings.CutPrefix(key, prefix); ok {
			out[rest] = weight
		}
	}
	return out
}
