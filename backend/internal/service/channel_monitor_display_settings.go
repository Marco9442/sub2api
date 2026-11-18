package service

import (
	"context"
	"strconv"
)

// ChannelMonitorDisplaySettings 是状态页的显示设置。
// 间隔只作为之后新建渠道的默认值，不改已有渠道。记录条数决定卡片上的最近记录。
type ChannelMonitorDisplaySettings struct {
	DefaultIntervalSeconds int `json:"default_interval_seconds"`
	TimelinePoints         int `json:"timeline_points"`
}

type channelMonitorDisplaySettingsWriter interface {
	SaveChannelMonitorDisplaySettings(ctx context.Context, intervalSeconds, timelinePoints int) error
}

// DisplaySettings 读取当前显示设置。缺省或超出范围时用 300 秒和 20 条。
func (s *ChannelMonitorService) DisplaySettings(ctx context.Context) ChannelMonitorDisplaySettings {
	runtime := s.probeRuntime(ctx)
	interval := runtime.DefaultIntervalSeconds
	if interval < channelMonitorIntervalMin || interval > channelMonitorIntervalMax {
		interval = channelMonitorIntervalFallback
	}
	points := runtime.TimelinePoints
	if points < 1 || points > channelMonitorTimelinePointsMax {
		points = channelMonitorTimelinePointsDefault
	}
	return ChannelMonitorDisplaySettings{DefaultIntervalSeconds: interval, TimelinePoints: points}
}

// TimelinePoints 返回状态页要取的记录条数。0 按 20，超过 200 收到 200。
func (s *ChannelMonitorService) TimelinePoints(ctx context.Context) int {
	points := s.probeRuntime(ctx).TimelinePoints
	if points < 1 {
		return channelMonitorTimelinePointsDefault
	}
	if points > channelMonitorTimelinePointsMax {
		return channelMonitorTimelinePointsMax
	}
	return points
}

// SaveDisplaySettings 写入新建渠道的默认间隔和状态页记录条数。
func (s *ChannelMonitorService) SaveDisplaySettings(ctx context.Context, intervalSeconds, timelinePoints int) error {
	writer, ok := s.settings.(channelMonitorDisplaySettingsWriter)
	if !ok || writer == nil {
		return ErrChannelMonitorDisabled
	}
	return writer.SaveChannelMonitorDisplaySettings(ctx, intervalSeconds, timelinePoints)
}

// SaveChannelMonitorDisplaySettings 拒绝超出范围的值，再写入并刷新运行时缓存。
func (s *SettingService) SaveChannelMonitorDisplaySettings(ctx context.Context, intervalSeconds, timelinePoints int) error {
	if s == nil || s.settingRepo == nil {
		return ErrChannelMonitorDisabled
	}
	if intervalSeconds < channelMonitorIntervalMin || intervalSeconds > channelMonitorIntervalMax {
		return ErrChannelMonitorInvalidInterval
	}
	if timelinePoints < 1 || timelinePoints > channelMonitorTimelinePointsMax {
		return ErrChannelMonitorInvalidTimelinePoints
	}
	if err := s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyChannelMonitorDefaultIntervalSeconds: strconv.Itoa(intervalSeconds),
		SettingKeyChannelMonitorTimelinePoints:         strconv.Itoa(timelinePoints),
	}); err != nil {
		return err
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}
	s.notifyChannelMonitorRuntimeListeners()
	return nil
}
