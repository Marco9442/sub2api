package service

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// AvailabilityWindow 是用户页可用率的一个时间档。
// 数量 1–60，单位是分钟或天，最长 30 天。
type AvailabilityWindow struct {
	Amount int    `json:"amount"`
	Unit   string `json:"unit"`
}

const (
	availabilityWindowMaxCount  = 6
	availabilityWindowMaxAmount = 60
	availabilityWindowMaxDays   = 30
)

func (w AvailabilityWindow) Key() string {
	return fmt.Sprintf("%d%s", w.Amount, w.Unit)
}

func (w AvailabilityWindow) Seconds() int {
	if w.Unit == "d" {
		return w.Amount * 24 * 60 * 60
	}
	return w.Amount * 60
}

func defaultAvailabilityWindows() []AvailabilityWindow {
	return []AvailabilityWindow{{Amount: 7, Unit: "d"}, {Amount: 15, Unit: "d"}, {Amount: 30, Unit: "d"}}
}

// parseAvailabilityWindows 解析 "15m,1d,5d" 这种文本。空文本返回默认档。
func parseAvailabilityWindows(raw string) ([]AvailabilityWindow, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultAvailabilityWindows(), nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) == 0 || len(parts) > availabilityWindowMaxCount {
		return nil, ErrChannelMonitorInvalidWindows
	}
	seen := make(map[string]bool, len(parts))
	out := make([]AvailabilityWindow, 0, len(parts))
	for _, part := range parts {
		w, ok := parseOneWindow(strings.TrimSpace(part))
		if !ok || seen[w.Key()] {
			return nil, ErrChannelMonitorInvalidWindows
		}
		seen[w.Key()] = true
		out = append(out, w)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Seconds() < out[j].Seconds() })
	return out, nil
}

func parseOneWindow(part string) (AvailabilityWindow, bool) {
	if len(part) < 2 {
		return AvailabilityWindow{}, false
	}
	unit := part[len(part)-1:]
	if unit != "m" && unit != "d" {
		return AvailabilityWindow{}, false
	}
	amount, err := strconv.Atoi(part[:len(part)-1])
	if err != nil || amount < 1 || amount > availabilityWindowMaxAmount {
		return AvailabilityWindow{}, false
	}
	if unit == "d" && amount > availabilityWindowMaxDays {
		return AvailabilityWindow{}, false
	}
	return AvailabilityWindow{Amount: amount, Unit: unit}, true
}

func formatAvailabilityWindows(windows []AvailabilityWindow) string {
	parts := make([]string, len(windows))
	for i, w := range windows {
		parts[i] = w.Key()
	}
	return strings.Join(parts, ",")
}

// windowAvailability 取指定模型在一个时间档内的可用率。
// 该档内没有任何记录时返回 nil，有记录但全部失败时返回 0。
func windowAvailability(avail map[string]*ChannelMonitorAvailability, model string) *float64 {
	row, ok := avail[model]
	if !ok || row.TotalChecks == 0 {
		return nil
	}
	pct := row.AvailabilityPct
	return &pct
}
