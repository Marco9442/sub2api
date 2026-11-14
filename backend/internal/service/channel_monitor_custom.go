package service

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	MonitorMetricKindLatency = "latency"
	MonitorMetricKindTTFT    = "ttft"
	MonitorMetricKindQuota   = "quota"

	monitorCustomPromptDefault   = "ping"
	monitorCustomPromptMaxRunes  = 500
	monitorCustomYellowDefault   = 2
	monitorCustomRedDefault      = 8
	monitorLoopbackExceptionURL  = "http://127.0.0.1:8080"
	monitorLoopbackExceptionHost = "127.0.0.1"
	monitorLoopbackExceptionPort = "8080"
)

// ChannelMonitorCustomConfig 是 Custom 模式的全局检测配置。
// 黄/红阈值单位是秒，和检测间隔同一量纲。
type ChannelMonitorCustomConfig struct {
	Prompt            string  `json:"prompt"`
	YellowSeconds     int     `json:"yellow_seconds"`
	RedSeconds        int     `json:"red_seconds"`
	TTFTAdjustEnabled bool    `json:"ttft_adjust_enabled"`
	TTFTMultiplier    float64 `json:"ttft_multiplier"`
	TTFTMinMs         int     `json:"ttft_min_ms"`
	TTFTMaxMs         int     `json:"ttft_max_ms"`
	TPSEnabled        bool    `json:"tps_enabled"`
	TPSMin            float64 `json:"tps_min"`
	TPSMax            float64 `json:"tps_max"`
}

// ChannelMonitorCustomOverride 是单条监控的覆盖。nil 字段继承全局配置。
type ChannelMonitorCustomOverride struct {
	TTFTAdjustEnabled *bool    `json:"ttft_adjust_enabled"`
	TTFTMultiplier    *float64 `json:"ttft_multiplier"`
	TTFTMinMs         *int     `json:"ttft_min_ms"`
	TTFTMaxMs         *int     `json:"ttft_max_ms"`
	TPSEnabled        *bool    `json:"tps_enabled"`
	TPSMin            *float64 `json:"tps_min"`
	TPSMax            *float64 `json:"tps_max"`
}

type ttftPolicy struct {
	Enabled    bool
	Multiplier float64
	MinMs      int
	MaxMs      int
}

type tpsPolicy struct {
	Enabled bool
	Min     float64
	Max     float64
}

func defaultChannelMonitorCustomConfig() ChannelMonitorCustomConfig {
	return ChannelMonitorCustomConfig{
		Prompt:         monitorCustomPromptDefault,
		YellowSeconds:  monitorCustomYellowDefault,
		RedSeconds:     monitorCustomRedDefault,
		TTFTMultiplier: 1,
	}
}

func isExactLoopbackExceptionURL(ep string) bool {
	ep = strings.TrimSpace(ep)
	ep = strings.TrimRight(ep, "/")
	return ep == monitorLoopbackExceptionURL
}

func isExactLoopbackExceptionDial(host, port string) bool {
	return host == monitorLoopbackExceptionHost && port == monitorLoopbackExceptionPort
}

func validateCustomPrompt(prompt string) error {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" || utf8.RuneCountInString(prompt) > monitorCustomPromptMaxRunes {
		return ErrChannelMonitorInvalidPrompt
	}
	return nil
}

// validateCustomThresholds 保证 0 < yellow < red，且 red 不超过最短实际间隔。
// 没有监控时，red 只受检测间隔上限约束。
func validateCustomThresholds(yellow, red, shortest int, hasMonitors bool) error {
	if yellow <= 0 || yellow >= red {
		return ErrChannelMonitorInvalidThreshold
	}
	limit := monitorMaxIntervalSeconds
	if hasMonitors {
		limit = shortest
	}
	if red > limit {
		return ErrChannelMonitorInvalidThreshold
	}
	return nil
}

func validateActualIntervalAgainstRed(actual, red int) error {
	if red > 0 && actual < red {
		return ErrChannelMonitorIntervalBelowRed
	}
	return nil
}

func validateCustomConfig(cfg ChannelMonitorCustomConfig) error {
	if err := validateCustomPrompt(cfg.Prompt); err != nil {
		return err
	}
	if err := validateTTFTPolicy(resolveTTFTPolicy(cfg, nil)); err != nil {
		return err
	}
	return validateTPSPolicy(resolveTPSPolicy(cfg, nil))
}

func validateCustomOverride(cfg ChannelMonitorCustomConfig, ov *ChannelMonitorCustomOverride) error {
	if ov == nil || customOverrideIsEmpty(ov) {
		return nil
	}
	if err := validateTTFTPolicy(resolveTTFTPolicy(cfg, ov)); err != nil {
		return err
	}
	return validateTPSPolicy(resolveTPSPolicy(cfg, ov))
}

func customOverrideIsEmpty(ov *ChannelMonitorCustomOverride) bool {
	if ov == nil {
		return true
	}
	return ov.TTFTAdjustEnabled == nil &&
		ov.TTFTMultiplier == nil &&
		ov.TTFTMinMs == nil &&
		ov.TTFTMaxMs == nil &&
		ov.TPSEnabled == nil &&
		ov.TPSMin == nil &&
		ov.TPSMax == nil
}

func resolveTTFTPolicy(cfg ChannelMonitorCustomConfig, ov *ChannelMonitorCustomOverride) ttftPolicy {
	policy := ttftPolicy{
		Enabled:    cfg.TTFTAdjustEnabled,
		Multiplier: cfg.TTFTMultiplier,
		MinMs:      cfg.TTFTMinMs,
		MaxMs:      cfg.TTFTMaxMs,
	}
	if policy.Multiplier <= 0 {
		policy.Multiplier = 1
	}
	if ov == nil {
		return policy
	}
	if ov.TTFTAdjustEnabled != nil {
		policy.Enabled = *ov.TTFTAdjustEnabled
	}
	if ov.TTFTMultiplier != nil {
		policy.Multiplier = *ov.TTFTMultiplier
	}
	if ov.TTFTMinMs != nil {
		policy.MinMs = *ov.TTFTMinMs
	}
	if ov.TTFTMaxMs != nil {
		policy.MaxMs = *ov.TTFTMaxMs
	}
	return policy
}

func resolveTPSPolicy(cfg ChannelMonitorCustomConfig, ov *ChannelMonitorCustomOverride) tpsPolicy {
	policy := tpsPolicy{Enabled: cfg.TPSEnabled, Min: cfg.TPSMin, Max: cfg.TPSMax}
	if ov == nil {
		return policy
	}
	if ov.TPSEnabled != nil {
		policy.Enabled = *ov.TPSEnabled
	}
	if ov.TPSMin != nil {
		policy.Min = *ov.TPSMin
	}
	if ov.TPSMax != nil {
		policy.Max = *ov.TPSMax
	}
	return policy
}

func validateTTFTPolicy(policy ttftPolicy) error {
	if !policy.Enabled {
		return nil
	}
	if policy.Multiplier <= 0 || policy.MinMs < 0 || policy.MinMs >= policy.MaxMs {
		return ErrChannelMonitorInvalidTTFT
	}
	return nil
}

func validateTPSPolicy(policy tpsPolicy) error {
	if !policy.Enabled {
		return nil
	}
	if policy.Min < 0 || policy.Min >= policy.Max {
		return ErrChannelMonitorInvalidTPS
	}
	return nil
}

func applyProcessedTTFT(measuredMs int, policy ttftPolicy) (int, error) {
	if measuredMs < 0 {
		measuredMs = 0
	}
	if !policy.Enabled {
		return measuredMs, nil
	}
	if err := validateTTFTPolicy(policy); err != nil {
		return 0, err
	}
	scaled := float64(measuredMs) * policy.Multiplier
	return int(math.Round(softBoundTTFT(scaled, float64(policy.MinMs), float64(policy.MaxMs)))), nil
}

// softBoundTTFT 先保持区间内的值，区间外用平滑压缩，避免硬截断把不同测值压成同一个数。
func softBoundTTFT(v, min, max float64) float64 {
	if v >= min && v <= max {
		return v
	}
	span := max - min
	if span <= 0 {
		return v
	}
	if v < min {
		excess := min - v
		return min - excess/(1+excess/span)
	}
	excess := v - max
	return max + excess/(1+excess/span)
}

func healthFromProcessedTTFT(processedMs, yellowSec, redSec int, completed bool) string {
	if !completed {
		return MonitorStatusError
	}
	yellowMs := yellowSec * 1000
	redMs := redSec * 1000
	if processedMs < yellowMs {
		return MonitorStatusOperational
	}
	if processedMs < redMs {
		return MonitorStatusDegraded
	}
	return MonitorStatusFailed
}

// deriveTokensPerSec 用最终 TTFT 生成稳定的 token/s。
// 同一监控、模型和 TTFT 始终得到同一个值；TTFT 越高，结果越低。
func deriveTokensPerSec(monitorID int64, model string, ttftMs int, min, max float64) float64 {
	if min < 0 || min >= max {
		return 0
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(model) + "\x00" + strconv.FormatInt(monitorID, 10)))
	offset := float64(binary.BigEndian.Uint32(sum[:4])%10000) / 10000
	slow := math.Atan(float64(ttftMs)/1000) / (math.Pi / 2)
	pos := slow*0.85 + offset*0.15
	if pos > 1 {
		pos = 1
	}
	value := max - pos*(max-min)
	if value < min {
		value = min
	}
	if value >= max {
		value = max - (max-min)*0.001
	}
	return math.Round(value*1000) / 1000
}

func shouldHideMonitorMetrics(kind, status string) bool {
	return kind == MonitorMetricKindTTFT &&
		(status == MonitorStatusFailed || status == MonitorStatusError)
}
