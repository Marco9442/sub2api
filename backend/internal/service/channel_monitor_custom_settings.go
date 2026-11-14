package service

import (
	"context"
	"strconv"
	"strings"
)

func channelMonitorCustomKeys() []string {
	return []string{
		SettingKeyChannelMonitorCustomPrompt,
		SettingKeyChannelMonitorCustomYellowSeconds,
		SettingKeyChannelMonitorCustomRedSeconds,
		SettingKeyChannelMonitorCustomTTFTAdjustEnabled,
		SettingKeyChannelMonitorCustomTTFTMultiplier,
		SettingKeyChannelMonitorCustomTTFTMinMs,
		SettingKeyChannelMonitorCustomTTFTMaxMs,
		SettingKeyChannelMonitorCustomTPSEnabled,
		SettingKeyChannelMonitorCustomTPSMin,
		SettingKeyChannelMonitorCustomTPSMax,
	}
}

// GetChannelMonitorCustom 读取 Custom 全局配置。
// 第二个返回值表示黄/红阈值已经写入，监控创建和更新这时才受阈值约束。
func (s *SettingService) GetChannelMonitorCustom(ctx context.Context) (ChannelMonitorCustomConfig, bool) {
	cfg := defaultChannelMonitorCustomConfig()
	if s == nil || s.settingRepo == nil {
		return cfg, false
	}
	vals, err := s.settingRepo.GetMultiple(ctx, channelMonitorCustomKeys())
	if err != nil {
		return cfg, false
	}
	_, hasYellow := vals[SettingKeyChannelMonitorCustomYellowSeconds]
	_, hasRed := vals[SettingKeyChannelMonitorCustomRedSeconds]
	return parseChannelMonitorCustom(vals), hasYellow && hasRed
}

// SaveChannelMonitorMode 写入生效模式和 Custom 全局配置，并刷新公开设置缓存。
func (s *SettingService) SaveChannelMonitorMode(ctx context.Context, mode string, cfg ChannelMonitorCustomConfig) error {
	if s == nil || s.settingRepo == nil {
		return ErrChannelMonitorDisabled
	}
	if err := validateCustomConfig(cfg); err != nil {
		return err
	}
	updates := map[string]string{
		SettingKeyChannelMonitorMode:                    normalizeChannelMonitorMode(mode),
		SettingKeyChannelMonitorCustomPrompt:            strings.TrimSpace(cfg.Prompt),
		SettingKeyChannelMonitorCustomYellowSeconds:     strconv.Itoa(cfg.YellowSeconds),
		SettingKeyChannelMonitorCustomRedSeconds:        strconv.Itoa(cfg.RedSeconds),
		SettingKeyChannelMonitorCustomTTFTAdjustEnabled: strconv.FormatBool(cfg.TTFTAdjustEnabled),
		SettingKeyChannelMonitorCustomTTFTMultiplier:    strconv.FormatFloat(cfg.TTFTMultiplier, 'f', -1, 64),
		SettingKeyChannelMonitorCustomTTFTMinMs:         strconv.Itoa(cfg.TTFTMinMs),
		SettingKeyChannelMonitorCustomTTFTMaxMs:         strconv.Itoa(cfg.TTFTMaxMs),
		SettingKeyChannelMonitorCustomTPSEnabled:        strconv.FormatBool(cfg.TPSEnabled),
		SettingKeyChannelMonitorCustomTPSMin:            strconv.FormatFloat(cfg.TPSMin, 'f', -1, 64),
		SettingKeyChannelMonitorCustomTPSMax:            strconv.FormatFloat(cfg.TPSMax, 'f', -1, 64),
	}
	if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
		return err
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}
	s.notifyChannelMonitorRuntimeListeners()
	return nil
}

func parseChannelMonitorCustom(vals map[string]string) ChannelMonitorCustomConfig {
	cfg := defaultChannelMonitorCustomConfig()
	if prompt := strings.TrimSpace(vals[SettingKeyChannelMonitorCustomPrompt]); prompt != "" {
		cfg.Prompt = prompt
	}
	if v, ok := atoiSetting(vals, SettingKeyChannelMonitorCustomYellowSeconds); ok {
		cfg.YellowSeconds = v
	}
	if v, ok := atoiSetting(vals, SettingKeyChannelMonitorCustomRedSeconds); ok {
		cfg.RedSeconds = v
	}
	if v, ok := vals[SettingKeyChannelMonitorCustomTTFTAdjustEnabled]; ok {
		cfg.TTFTAdjustEnabled = v == "true"
	}
	if v, ok := atofSetting(vals, SettingKeyChannelMonitorCustomTTFTMultiplier); ok && v > 0 {
		cfg.TTFTMultiplier = v
	}
	if v, ok := atoiSetting(vals, SettingKeyChannelMonitorCustomTTFTMinMs); ok {
		cfg.TTFTMinMs = v
	}
	if v, ok := atoiSetting(vals, SettingKeyChannelMonitorCustomTTFTMaxMs); ok {
		cfg.TTFTMaxMs = v
	}
	if v, ok := vals[SettingKeyChannelMonitorCustomTPSEnabled]; ok {
		cfg.TPSEnabled = v == "true"
	}
	if v, ok := atofSetting(vals, SettingKeyChannelMonitorCustomTPSMin); ok {
		cfg.TPSMin = v
	}
	if v, ok := atofSetting(vals, SettingKeyChannelMonitorCustomTPSMax); ok {
		cfg.TPSMax = v
	}
	return cfg
}

func atoiSetting(vals map[string]string, key string) (int, bool) {
	raw, ok := vals[key]
	if !ok {
		return 0, false
	}
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, false
	}
	return v, true
}

func atofSetting(vals map[string]string, key string) (float64, bool) {
	raw, ok := vals[key]
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
