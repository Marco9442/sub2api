package accountsim

import (
	"hash/fnv"
	"sort"
	"time"
)

// 基准点来自需求：该时刻的 ID 水位为 6511。
const (
	idAnchorUnix = 1794713258
	idAnchorID   = 6511
)

// 号池按北京时间运营。中国没有夏令时，固定时区即可，不依赖 tzdata。
var cst = time.FixedZone("CST", 8*3600)

func dayIndex(t time.Time) int64 {
	return floorDiv(t.Unix()+8*3600, 86400)
}

func dayStart(index int64) time.Time {
	return time.Unix(index*86400-8*3600, 0).In(cst)
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// dailyNewCount 由日期和配置确定，同一天永远得到同一个数量。
func dailyNewCount(day int64, cfg Config) int64 {
	span := cfg.DailyNewMax - cfg.DailyNewMin + 1
	if span <= 1 {
		return cfg.DailyNewMin
	}
	h := fnv.New64a()
	var buf [8]byte
	for i := range buf {
		buf[i] = byte(uint64(day) >> (8 * i))
	}
	_, _ = h.Write(buf[:])
	return cfg.DailyNewMin + int64(h.Sum64()%uint64(span))
}

// hourlyAllocation 把一天的新增按时段权重分到 24 个小时。
// 余数给权重最大的小时，24 小时之和恰好等于当天新增。
func hourlyAllocation(day int64, cfg Config) [24]int64 {
	var out [24]int64
	count := dailyNewCount(day, cfg)
	weights := cfg.hourWeights(dayStart(day).Weekday())
	total, heaviest := 0, 0
	for hour, weight := range weights {
		total += weight
		if weight > weights[heaviest] {
			heaviest = hour
		}
	}
	if total == 0 {
		out[heaviest] = count
		return out
	}
	used := int64(0)
	for hour, weight := range weights {
		out[hour] = count * int64(weight) / int64(total)
		used += out[hour]
	}
	out[heaviest] += count - used
	return out
}

// Step 是自动运营的时间步长。补齐和展示都以它为最小粒度。
const Step = 10 * time.Minute

// hourPlan 返回某个整点开始的一小时内各新账号的创建偏移（秒），升序。
// 同一小时永远得到同一组偏移，ID 因此按创建时间递增。
func hourPlan(hourStart time.Time, cfg Config) []int {
	n := int(hourlyAllocation(dayIndex(hourStart), cfg)[hourStart.In(cst).Hour()])
	r := newRNG(hourStart.Unix(), 0x6e6577)
	offsets := make([]int, n)
	for i := range offsets {
		offsets[i] = r.intn(3600)
	}
	sort.Ints(offsets)
	return offsets
}

// cumulative 返回从基准日零点到给定步长起点之前的累计新增，基准日之前为负数。
func cumulative(at time.Time, cfg Config) int64 {
	at = at.Truncate(Step)
	anchorDay := dayIndex(time.Unix(idAnchorUnix, 0))
	day := dayIndex(at)
	sum := int64(0)
	for d := anchorDay; d < day; d++ {
		sum += dailyNewCount(d, cfg)
	}
	for d := day; d < anchorDay; d++ {
		sum -= dailyNewCount(d, cfg)
	}
	alloc := hourlyAllocation(day, cfg)
	hour := at.In(cst).Hour()
	for h := 0; h < hour; h++ {
		sum += alloc[h]
	}
	hourStart := at.Truncate(time.Hour)
	elapsed := int(at.Sub(hourStart) / time.Second)
	for _, offset := range hourPlan(hourStart, cfg) {
		if offset < elapsed {
			sum++
		}
	}
	return sum
}

// naturalWaterline 是只按时间推算、不含下限的 ID 水位。
func naturalWaterline(at time.Time, cfg Config) int64 {
	return idAnchorID + cumulative(at, cfg) - cumulative(time.Unix(idAnchorUnix, 0), cfg)
}

// IDWaterline 计算给定时刻的 ID 水位，以 Step 为粒度。
// 同一时刻重复计算得到同一结果，且不低于配置的 ID 下限。
func IDWaterline(at time.Time, cfg Config) int64 {
	if w := naturalWaterline(at, cfg); w > cfg.IDFloor {
		return w
	}
	return cfg.IDFloor
}
