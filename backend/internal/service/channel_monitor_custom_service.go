package service

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
)

// ChannelMonitorModeView 是管理页读取的已保存模式，不是预览状态。
type ChannelMonitorModeView struct {
	Mode                string
	Enabled             bool
	Custom              ChannelMonitorCustomConfig
	ThresholdsPersisted bool
}

type channelMonitorCustomStore interface {
	GetCustomOverride(ctx context.Context, id int64) (*ChannelMonitorCustomOverride, error)
	ListCustomOverrides(ctx context.Context, ids []int64) (map[int64]*ChannelMonitorCustomOverride, error)
	UpsertCustomOverride(ctx context.Context, id int64, ov *ChannelMonitorCustomOverride) error
	DeleteCustomOverride(ctx context.Context, id int64) error
	ShortestActualInterval(ctx context.Context) (seconds int, hasMonitors bool, err error)
}

type channelMonitorCustomSettings interface {
	GetChannelMonitorCustom(ctx context.Context) (ChannelMonitorCustomConfig, bool)
	SaveChannelMonitorMode(ctx context.Context, mode string, cfg ChannelMonitorCustomConfig) error
}

func (s *ChannelMonitorService) loadCustomConfig(ctx context.Context) (ChannelMonitorCustomConfig, bool) {
	reader, ok := s.customSettings()
	if !ok {
		return defaultChannelMonitorCustomConfig(), false
	}
	return reader.GetChannelMonitorCustom(ctx)
}

func (s *ChannelMonitorService) GetMonitorMode(ctx context.Context) ChannelMonitorModeView {
	rt := s.probeRuntime(ctx)
	cfg, persisted := s.loadCustomConfig(ctx)
	return ChannelMonitorModeView{
		Mode:                rt.Mode,
		Enabled:             rt.Enabled,
		Custom:              cfg,
		ThresholdsPersisted: persisted,
	}
}

func (s *ChannelMonitorService) SaveMonitorMode(ctx context.Context, mode string, cfg ChannelMonitorCustomConfig) error {
	if err := validateCustomConfig(cfg); err != nil {
		return err
	}
	shortest, has := 0, false
	if store, ok := s.customStore(); ok {
		var err error
		shortest, has, err = store.ShortestActualInterval(ctx)
		if err != nil {
			return err
		}
	}
	if err := validateCustomThresholds(cfg.YellowSeconds, cfg.RedSeconds, shortest, has); err != nil {
		return err
	}
	if err := s.rejectBrokenOverrides(ctx, cfg); err != nil {
		return err
	}
	writer, ok := s.customSettings()
	if !ok {
		return fmt.Errorf("channel monitor settings writer is not configured")
	}
	return writer.SaveChannelMonitorMode(ctx, mode, cfg)
}

func (s *ChannelMonitorService) customSettings() (channelMonitorCustomSettings, bool) {
	if s == nil || s.settings == nil {
		return nil, false
	}
	reader, ok := s.settings.(channelMonitorCustomSettings)
	return reader, ok
}

func (s *ChannelMonitorService) customStore() (channelMonitorCustomStore, bool) {
	if s == nil || s.repo == nil {
		return nil, false
	}
	store, ok := s.repo.(channelMonitorCustomStore)
	return store, ok
}

func (s *ChannelMonitorService) enforceCustomInterval(ctx context.Context, interval, jitter int) error {
	cfg, persisted := s.loadCustomConfig(ctx)
	if !persisted {
		return nil
	}
	return validateActualIntervalAgainstRed(interval-jitter, cfg.RedSeconds)
}

func (s *ChannelMonitorService) saveCustomOverride(ctx context.Context, id int64, ov *ChannelMonitorCustomOverride) error {
	if ov == nil {
		return nil
	}
	cfg, _ := s.loadCustomConfig(ctx)
	if err := validateCustomOverride(cfg, ov); err != nil {
		return err
	}
	store, ok := s.customStore()
	if !ok {
		return nil
	}
	if customOverrideIsEmpty(ov) {
		return store.DeleteCustomOverride(ctx, id)
	}
	return store.UpsertCustomOverride(ctx, id, ov)
}

func (s *ChannelMonitorService) attachCustomOverrides(ctx context.Context, monitors []*ChannelMonitor) {
	store, ok := s.customStore()
	if !ok || len(monitors) == 0 {
		return
	}
	ids := make([]int64, 0, len(monitors))
	for _, m := range monitors {
		if m != nil {
			ids = append(ids, m.ID)
		}
	}
	found, err := store.ListCustomOverrides(ctx, ids)
	if err != nil {
		return
	}
	for _, m := range monitors {
		if m == nil {
			continue
		}
		m.CustomOverride = found[m.ID]
	}
}

// customPingContext 不继承模型检测的红色截止，避免 HEAD ping 吃掉模型请求预算。
func customPingContext(parent context.Context) context.Context {
	return context.WithoutCancel(parent)
}

// customModelContext 在 ping 结束后单独计时。截止长度是红色阈值。
func customModelContext(parent context.Context, red time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), red)
}

// historyWriteContext 让 Custom 的写库不受调度器短超时影响。V1 仍使用原上下文。
func historyWriteContext(parent context.Context, mode string) (context.Context, context.CancelFunc) {
	if mode != ChannelMonitorModeCustom {
		return parent, func() {}
	}
	return context.WithTimeout(context.WithoutCancel(parent), monitorRunOneBuffer)
}

type channelMonitorTxRunner interface {
	WithMonitorTx(ctx context.Context, fn func(context.Context) error) error
}

type channelMonitorOverrideCatalog interface {
	ListAllCustomOverrides(ctx context.Context) ([]*ChannelMonitorCustomOverride, error)
}

func (s *ChannelMonitorService) rejectBrokenOverrides(ctx context.Context, cfg ChannelMonitorCustomConfig) error {
	catalog, ok := s.repo.(channelMonitorOverrideCatalog)
	if !ok {
		return nil
	}
	items, err := catalog.ListAllCustomOverrides(ctx)
	if err != nil {
		return err
	}
	return overridesRemainValid(cfg, items)
}

// commitMonitorChange 先校验覆盖配置，再把主记录和覆盖配置放进同一事务。
// 没有事务能力的测试替身仍按先写主记录、再写覆盖的顺序执行。
func (s *ChannelMonitorService) commitMonitorChange(ctx context.Context, ov *ChannelMonitorCustomOverride, write func(context.Context) error, id func() int64) error {
	if ov != nil {
		cfg, _ := s.loadCustomConfig(ctx)
		if err := validateCustomOverride(cfg, ov); err != nil {
			return err
		}
	}
	save := func(txCtx context.Context) error {
		if err := write(txCtx); err != nil {
			return err
		}
		return s.saveCustomOverride(txCtx, id(), ov)
	}
	if runner, ok := s.repo.(channelMonitorTxRunner); ok && ov != nil {
		return runner.WithMonitorTx(ctx, save)
	}
	return save(ctx)
}

func (s *ChannelMonitorService) lookupCustomOverride(ctx context.Context, id int64) (*ChannelMonitorCustomOverride, error) {
	store, ok := s.customStore()
	if !ok {
		return nil, nil
	}
	return store.GetCustomOverride(ctx, id)
}

func (s *ChannelMonitorService) copyMissingCustomOverride(ctx context.Context, sourceID int64, dest *ChannelMonitor) error {
	if dest == nil {
		return nil
	}
	store, ok := s.customStore()
	if !ok {
		return nil
	}
	existing, err := store.GetCustomOverride(ctx, dest.ID)
	if err != nil || existing != nil {
		dest.CustomOverride = existing
		return err
	}
	source, err := store.GetCustomOverride(ctx, sourceID)
	if err != nil || source == nil {
		return err
	}
	if err := store.UpsertCustomOverride(ctx, dest.ID, source); err != nil {
		return err
	}
	dest.CustomOverride = source
	return nil
}

func overridesRemainValid(cfg ChannelMonitorCustomConfig, items []*ChannelMonitorCustomOverride) error {
	for _, ov := range items {
		if err := validateCustomOverride(cfg, ov); err != nil {
			return err
		}
	}
	return nil
}

func (s *ChannelMonitorService) runCustomChecks(ctx context.Context, m *ChannelMonitor) []*CheckResult {
	cfg, _ := s.loadCustomConfig(ctx)
	var override *ChannelMonitorCustomOverride
	if store, ok := s.customStore(); ok {
		override, _ = store.GetCustomOverride(ctx, m.ID)
	}
	deadline := time.Duration(cfg.RedSeconds) * time.Second
	if deadline <= 0 {
		deadline = time.Duration(monitorCustomRedDefault) * time.Second
	}
	pingMs := pingEndpointOrigin(customPingContext(ctx), m.Endpoint)
	ctx, cancel := customModelContext(ctx, deadline)
	defer cancel()

	models := append([]string{m.PrimaryModel}, m.ExtraModels...)
	results := make([]*CheckResult, len(models))
	opts := &CheckOptions{
		APIMode:          m.APIMode,
		ExtraHeaders:     m.ExtraHeaders,
		BodyOverrideMode: m.BodyOverrideMode,
		BodyOverride:     m.BodyOverride,
	}
	var eg errgroup.Group
	for i, model := range models {
		i, model := i, model
		eg.Go(func() error {
			results[i] = runCustomCheckForModel(ctx, customProbeInput{
				MonitorID: m.ID,
				Provider:  m.Provider,
				Endpoint:  m.Endpoint,
				APIKey:    m.APIKey,
				Model:     model,
				Opts:      opts,
				Config:    cfg,
				Override:  override,
			})
			results[i].PingLatencyMs = pingMs
			return nil
		})
	}
	_ = eg.Wait()
	return results
}

func historyMetricKind(mode, checkMode string, result *CheckResult) string {
	if result != nil && result.MetricKind != "" {
		return result.MetricKind
	}
	if defaultCheckMode(checkMode) == MonitorCheckModeQuota {
		return MonitorMetricKindQuota
	}
	if mode == ChannelMonitorModeCustom {
		return MonitorMetricKindTTFT
	}
	return MonitorMetricKindLatency
}

type availabilityKindRepo interface {
	ComputeAvailabilityKind(ctx context.Context, monitorID int64, windowDays int, kind string) ([]*ChannelMonitorAvailability, error)
	ComputeAvailabilityForMonitorsKind(ctx context.Context, ids []int64, windowDays int, kind string) (map[int64][]*ChannelMonitorAvailability, error)
}

func (s *ChannelMonitorService) availabilityKind(ctx context.Context) string {
	if s.probeRuntime(ctx).Mode == ChannelMonitorModeCustom {
		return MonitorMetricKindTTFT
	}
	return MonitorMetricKindLatency
}

func (s *ChannelMonitorService) availabilityForMonitors(ctx context.Context, ids []int64, windowDays int) (map[int64][]*ChannelMonitorAvailability, error) {
	if repo, ok := s.repo.(availabilityKindRepo); ok {
		return repo.ComputeAvailabilityForMonitorsKind(ctx, ids, windowDays, s.availabilityKind(ctx))
	}
	return s.repo.ComputeAvailabilityForMonitors(ctx, ids, windowDays)
}

func (s *ChannelMonitorService) availabilityForMonitor(ctx context.Context, id int64, windowDays int) ([]*ChannelMonitorAvailability, error) {
	if repo, ok := s.repo.(availabilityKindRepo); ok {
		return repo.ComputeAvailabilityKind(ctx, id, windowDays, s.availabilityKind(ctx))
	}
	return s.repo.ComputeAvailability(ctx, id, windowDays)
}

func redactDisplayedMetrics(kind, status string, latency *int, tokens *float64) (*int, *float64) {
	if shouldHideMonitorMetrics(kind, status) {
		return nil, nil
	}
	return latency, tokens
}
