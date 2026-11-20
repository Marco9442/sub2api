package accountsim

import (
	"hash/fnv"
	"sort"
)

// rng 是由种子确定的随机数。同一组种子永远得到同一序列，
// 补齐和重算因此可以重复，不依赖进程或调用顺序。
type rng struct{ state uint64 }

func newRNG(seeds ...int64) *rng {
	h := fnv.New64a()
	var buf [8]byte
	for _, seed := range seeds {
		for i := range buf {
			buf[i] = byte(uint64(seed) >> (8 * i))
		}
		_, _ = h.Write(buf[:])
	}
	return &rng{state: h.Sum64() | 1}
}

func (r *rng) next() uint64 {
	// splitmix64
	r.state += 0x9e3779b97f4a7c15
	z := r.state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func (r *rng) intn(n int) int {
	if n <= 1 {
		return 0
	}
	return int(r.next() % uint64(n))
}

// between 返回 [lo, hi] 内的整数。
func (r *rng) between(lo, hi int) int {
	if hi <= lo {
		return lo
	}
	return lo + r.intn(hi-lo+1)
}

func (r *rng) float() float64 {
	return float64(r.next()>>11) / float64(1<<53)
}

// pick 按权重选一个键。键按字典序遍历，结果不受 map 顺序影响。
func (r *rng) pick(weights map[string]int) string {
	keys := make([]string, 0, len(weights))
	total := 0
	for key, weight := range weights {
		if weight > 0 {
			keys = append(keys, key)
			total += weight
		}
	}
	if total == 0 {
		return ""
	}
	sort.Strings(keys)
	roll := r.intn(total)
	for _, key := range keys {
		roll -= weights[key]
		if roll < 0 {
			return key
		}
	}
	return keys[len(keys)-1]
}

func (r *rng) choose(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return items[r.intn(len(items))]
}
