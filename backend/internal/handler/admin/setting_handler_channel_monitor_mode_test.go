//go:build unit

package admin

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUpdateSettingsRejectsEnteringCustom(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyChannelMonitorMode: service.ChannelMonitorModeV1,
	})

	rec := doUpdateSettings(t, h, map[string]any{
		"channel_monitor_mode": service.ChannelMonitorModeCustom,
	}, nil)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "CHANNEL_MONITOR_CUSTOM_MODE_STALE")
	require.Nil(t, repo.lastUpdates)
	require.Equal(t, service.ChannelMonitorModeV1, repo.values[service.SettingKeyChannelMonitorMode])
}

func TestUpdateSettingsKeepsActiveCustom(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyChannelMonitorMode: service.ChannelMonitorModeCustom,
	})

	rec := doUpdateSettings(t, h, map[string]any{
		"channel_monitor_mode": service.ChannelMonitorModeCustom,
	}, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, service.ChannelMonitorModeCustom, repo.values[service.SettingKeyChannelMonitorMode])
}

func TestUpdateSettingsAllowsLeavingCustom(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyChannelMonitorMode: service.ChannelMonitorModeCustom,
	})

	rec := doUpdateSettings(t, h, map[string]any{
		"channel_monitor_mode": service.ChannelMonitorModeV1,
	}, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, service.ChannelMonitorModeV1, repo.values[service.SettingKeyChannelMonitorMode])
}

func TestUpdateSettingsOmittedModeKeepsActiveCustom(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyChannelMonitorMode: service.ChannelMonitorModeCustom,
	})

	rec := doUpdateSettings(t, h, map[string]any{"site_name": "kept"}, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, service.ChannelMonitorModeCustom, repo.values[service.SettingKeyChannelMonitorMode])
}
