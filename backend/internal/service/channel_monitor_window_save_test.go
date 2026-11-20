//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type windowSettingsStub struct {
	overrideCatalogSettings
	saved ChannelMonitorCustomConfig
}

func (s *windowSettingsStub) SaveChannelMonitorMode(_ context.Context, _ string, cfg ChannelMonitorCustomConfig) error {
	s.saved = cfg
	return nil
}

func (s *windowSettingsStub) GetChannelMonitorCustom(context.Context) (ChannelMonitorCustomConfig, bool) {
	return s.saved, true
}

func TestSaveMonitorModeKeepsAvailabilityWindows(t *testing.T) {
	settings := &windowSettingsStub{}
	svc := NewChannelMonitorService(&overrideCatalogRepo{}, &duplicateChannelMonitorEncryptor{})
	svc.SetRuntimeReader(settings)

	cfg := defaultChannelMonitorCustomConfig()
	cfg.AvailabilityWindows = []AvailabilityWindow{{Amount: 15, Unit: "m"}, {Amount: 1, Unit: "d"}, {Amount: 5, Unit: "d"}}
	require.NoError(t, svc.SaveMonitorMode(context.Background(), ChannelMonitorModeCustom, cfg))

	view := svc.GetMonitorMode(context.Background())
	require.Equal(t, "15m,1d,5d", formatAvailabilityWindows(view.Custom.AvailabilityWindows))
}

func TestSaveMonitorModeRejectsBadAvailabilityWindows(t *testing.T) {
	settings := &windowSettingsStub{}
	svc := NewChannelMonitorService(&overrideCatalogRepo{}, &duplicateChannelMonitorEncryptor{})
	svc.SetRuntimeReader(settings)

	cfg := defaultChannelMonitorCustomConfig()
	cfg.AvailabilityWindows = []AvailabilityWindow{{Amount: 31, Unit: "d"}}
	require.ErrorIs(t, svc.SaveMonitorMode(context.Background(), ChannelMonitorModeCustom, cfg), ErrChannelMonitorInvalidWindows)
}

type windowSecondsRepo struct {
	overrideCatalogRepo
	calledSeconds int
	rows          map[int64][]*ChannelMonitorAvailability
}

func (r *windowSecondsRepo) ComputeAvailabilityForMonitorsSeconds(_ context.Context, _ []int64, seconds int, _ string) (map[int64][]*ChannelMonitorAvailability, error) {
	r.calledSeconds = seconds
	return r.rows, nil
}

type v1RuntimeStub struct{ overrideCatalogSettings }

func (s *v1RuntimeStub) GetChannelMonitorRuntime(context.Context) ChannelMonitorRuntime {
	return ChannelMonitorRuntime{Enabled: true, Mode: ChannelMonitorModeV1}
}

func TestFillWindowAvailability_UsesConfiguredWindow(t *testing.T) {
	pct := 75.0
	repo := &windowSecondsRepo{rows: map[int64][]*ChannelMonitorAvailability{
		1: {{Model: "gpt-5", TotalChecks: 4, AvailabilityPct: pct}},
	}}
	settings := &windowSettingsStub{}
	cfg := defaultChannelMonitorCustomConfig()
	cfg.AvailabilityWindows = []AvailabilityWindow{{Amount: 15, Unit: "m"}}
	settings.saved = cfg
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})
	svc.SetRuntimeReader(settings)

	views := []*UserMonitorView{{ID: 1, PrimaryModel: "gpt-5"}}
	svc.fillWindowAvailability(context.Background(), views, "")

	require.Equal(t, 15*60, repo.calledSeconds)
	require.NotNil(t, views[0].WindowAvailability)
	require.Equal(t, 75.0, *views[0].WindowAvailability)
}

func TestFillWindowAvailability_SkipsV2(t *testing.T) {
	repo := &windowSecondsRepo{}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})

	views := []*UserMonitorView{{ID: 1, PrimaryModel: "gpt-5"}}
	svc.fillWindowAvailability(context.Background(), views, "")

	require.Equal(t, 0, repo.calledSeconds)
	require.Nil(t, views[0].WindowAvailability)
}

func TestFillWindowAvailability_V1UsesDefaultWindow(t *testing.T) {
	repo := &windowSecondsRepo{}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})
	svc.SetRuntimeReader(&v1RuntimeStub{})

	views := []*UserMonitorView{{ID: 1, PrimaryModel: "gpt-5"}}
	svc.fillWindowAvailability(context.Background(), views, "")

	require.Equal(t, 90*60, repo.calledSeconds)
}

func TestAvailabilityWindows_CustomReturnsConfigured(t *testing.T) {
	settings := &windowSettingsStub{}
	cfg := defaultChannelMonitorCustomConfig()
	cfg.AvailabilityWindows = []AvailabilityWindow{{Amount: 15, Unit: "m"}, {Amount: 1, Unit: "d"}}
	settings.saved = cfg
	svc := NewChannelMonitorService(&overrideCatalogRepo{}, &duplicateChannelMonitorEncryptor{})
	svc.SetRuntimeReader(settings)

	got := svc.AvailabilityWindows(context.Background())
	require.Equal(t, "15m,1d", formatAvailabilityWindows(got))
}

func TestAvailabilityWindows_V2ReturnsNil(t *testing.T) {
	svc := NewChannelMonitorService(&overrideCatalogRepo{}, &duplicateChannelMonitorEncryptor{})
	require.Nil(t, svc.AvailabilityWindows(context.Background()))
}

func TestAvailabilityWindows_V1ReturnsDefault(t *testing.T) {
	svc := NewChannelMonitorService(&overrideCatalogRepo{}, &duplicateChannelMonitorEncryptor{})
	svc.SetRuntimeReader(&v1RuntimeStub{})
	require.Equal(t, "90m,1d,3d,7d", formatAvailabilityWindows(svc.AvailabilityWindows(context.Background())))
}

func TestFillWindowAvailability_UsesRequestedWindow(t *testing.T) {
	repo := &windowSecondsRepo{rows: map[int64][]*ChannelMonitorAvailability{1: {{Model: "claude-sonnet", TotalChecks: 2, OperationalChecks: 1}}}}
	settings := &windowSettingsStub{}
	cfg := defaultChannelMonitorCustomConfig()
	cfg.AvailabilityWindows = []AvailabilityWindow{{Amount: 15, Unit: "m"}, {Amount: 1, Unit: "d"}}
	settings.saved = cfg
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})
	svc.SetRuntimeReader(settings)

	views := []*UserMonitorView{{ID: 1, PrimaryModel: "claude-sonnet"}}
	svc.fillWindowAvailability(context.Background(), views, "1d")

	require.Equal(t, 86400, repo.calledSeconds)
}
