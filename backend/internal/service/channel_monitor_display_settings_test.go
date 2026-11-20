//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSaveDisplaySettingsRejectsOutOfRange(t *testing.T) {
	repo := &memorySettingRepo{values: map[string]string{}}
	settings := NewSettingService(repo, &config.Config{})
	svc := NewChannelMonitorService(&overrideCatalogRepo{}, nil)
	svc.SetRuntimeReader(settings)

	require.ErrorIs(t, svc.SaveDisplaySettings(context.Background(), 10, 20), ErrChannelMonitorInvalidInterval)
	require.ErrorIs(t, svc.SaveDisplaySettings(context.Background(), 300, 0), ErrChannelMonitorInvalidTimelinePoints)
	require.ErrorIs(t, svc.SaveDisplaySettings(context.Background(), 300, 201), ErrChannelMonitorInvalidTimelinePoints)
	require.Empty(t, repo.values)
}

func TestSaveDisplaySettingsPersists(t *testing.T) {
	repo := &memorySettingRepo{values: map[string]string{}}
	settings := NewSettingService(repo, &config.Config{})
	svc := NewChannelMonitorService(&overrideCatalogRepo{}, nil)
	svc.SetRuntimeReader(settings)

	require.NoError(t, svc.SaveDisplaySettings(context.Background(), 300, 20))
	got := svc.DisplaySettings(context.Background())
	require.Equal(t, 300, got.DefaultIntervalSeconds)
	require.Equal(t, 20, got.TimelinePoints)
	runtime := settings.GetChannelMonitorRuntime(context.Background())
	require.Equal(t, 300, runtime.DefaultIntervalSeconds)
	require.Equal(t, 20, runtime.TimelinePoints)
}

func TestDisplaySettingsFallsBackWhenUnset(t *testing.T) {
	svc := NewChannelMonitorService(&overrideCatalogRepo{}, nil)
	got := svc.DisplaySettings(context.Background())
	require.Equal(t, 300, got.DefaultIntervalSeconds)
	require.Equal(t, 20, got.TimelinePoints)
	require.Equal(t, 20, svc.TimelinePoints(context.Background()))
}

func TestSaveDisplaySettingsWithoutWriter(t *testing.T) {
	svc := NewChannelMonitorService(&overrideCatalogRepo{}, nil)
	svc.SetRuntimeReader(&v1RuntimeStub{})
	require.ErrorIs(t, svc.SaveDisplaySettings(context.Background(), 300, 20), ErrChannelMonitorDisabled)
}
