//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type memorySettingRepo struct {
	values map[string]string
}

func (r *memorySettingRepo) Get(context.Context, string) (*Setting, error) { return nil, nil }
func (r *memorySettingRepo) GetValue(context.Context, string) (string, error) {
	return "", nil
}
func (r *memorySettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}
func (r *memorySettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, key := range keys {
		if v, ok := r.values[key]; ok {
			out[key] = v
		}
	}
	return out, nil
}
func (r *memorySettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	for k, v := range settings {
		r.values[k] = v
	}
	return nil
}
func (r *memorySettingRepo) GetAll(context.Context) (map[string]string, error) { return r.values, nil }
func (r *memorySettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func TestChannelMonitorCustomWindowsRoundTrip(t *testing.T) {
	repo := &memorySettingRepo{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})

	cfg := defaultChannelMonitorCustomConfig()
	cfg.AvailabilityWindows = []AvailabilityWindow{{Amount: 15, Unit: "m"}, {Amount: 5, Unit: "d"}}
	require.NoError(t, svc.SaveChannelMonitorMode(context.Background(), ChannelMonitorModeCustom, cfg))

	got, persisted := svc.GetChannelMonitorCustom(context.Background())
	require.True(t, persisted)
	require.Equal(t, "15m,5d", formatAvailabilityWindows(got.AvailabilityWindows))

	empty := defaultChannelMonitorCustomConfig()
	require.NoError(t, svc.SaveChannelMonitorMode(context.Background(), ChannelMonitorModeCustom, empty))
	got, _ = svc.GetChannelMonitorCustom(context.Background())
	require.Equal(t, "7d,15d,30d", formatAvailabilityWindows(got.AvailabilityWindows))
}
