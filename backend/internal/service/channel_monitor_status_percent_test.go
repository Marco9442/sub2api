//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultCustomConfigUsesThirtyPercentStatus(t *testing.T) {
	cfg := defaultChannelMonitorCustomConfig()
	require.Equal(t, 30, cfg.StatusYellowPercent)
	require.Equal(t, 30, cfg.StatusRedPercent)
	require.NoError(t, validateCustomConfig(cfg))
}

func TestValidateStatusPercentsAllowsEqualThresholds(t *testing.T) {
	cfg := defaultChannelMonitorCustomConfig()
	cfg.StatusYellowPercent = 30
	cfg.StatusRedPercent = 30
	require.NoError(t, validateCustomConfig(cfg))

	cfg.StatusYellowPercent = 80
	cfg.StatusRedPercent = 10
	require.NoError(t, validateCustomConfig(cfg))
}

func TestValidateStatusPercentsRejectsOutOfRange(t *testing.T) {
	for _, pct := range []int{0, -1, 101} {
		cfg := defaultChannelMonitorCustomConfig()
		cfg.StatusYellowPercent = pct
		require.ErrorIs(t, validateCustomConfig(cfg), ErrChannelMonitorInvalidStatusPercent)

		cfg = defaultChannelMonitorCustomConfig()
		cfg.StatusRedPercent = pct
		require.ErrorIs(t, validateCustomConfig(cfg), ErrChannelMonitorInvalidStatusPercent)
	}
}

func TestStatusPercentsOnlyInCustomMode(t *testing.T) {
	settings := &windowSettingsStub{}
	settings.saved = defaultChannelMonitorCustomConfig()
	settings.saved.StatusYellowPercent = 40
	settings.saved.StatusRedPercent = 25
	svc := NewChannelMonitorService(&overrideCatalogRepo{}, &duplicateChannelMonitorEncryptor{})
	svc.SetRuntimeReader(settings)

	yellow, red := svc.StatusPercents(context.Background())
	require.Equal(t, 40, yellow)
	require.Equal(t, 25, red)

	svc.SetRuntimeReader(&v1RuntimeStub{})
	yellow, red = svc.StatusPercents(context.Background())
	require.Zero(t, yellow)
	require.Zero(t, red)
}

func TestChannelMonitorCustomStatusPercentsRoundTrip(t *testing.T) {
	repo := &memorySettingRepo{values: map[string]string{}}
	svc := NewSettingService(repo, nil)

	cfg := defaultChannelMonitorCustomConfig()
	cfg.StatusYellowPercent = 45
	cfg.StatusRedPercent = 15
	require.NoError(t, svc.SaveChannelMonitorMode(context.Background(), ChannelMonitorModeCustom, cfg))

	got, _ := svc.GetChannelMonitorCustom(context.Background())
	require.Equal(t, 45, got.StatusYellowPercent)
	require.Equal(t, 15, got.StatusRedPercent)
}
