//go:build unit

package service

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeChannelMonitorMode_Custom(t *testing.T) {
	require.Equal(t, ChannelMonitorModeCustom, normalizeChannelMonitorMode("custom"))
	require.Equal(t, ChannelMonitorModeCustom, normalizeChannelMonitorMode(" Custom "))
	require.Equal(t, ChannelMonitorModeV1, normalizeChannelMonitorMode("invalid"))
	require.Equal(t, ChannelMonitorModeV1, normalizeChannelMonitorMode(""))
}

func TestActiveProbesAllowed_Custom(t *testing.T) {
	require.True(t, (ChannelMonitorRuntime{Enabled: true, Mode: ChannelMonitorModeCustom}).ActiveProbesAllowed())
	require.False(t, (ChannelMonitorRuntime{Enabled: false, Mode: ChannelMonitorModeCustom}).ActiveProbesAllowed())
	require.False(t, (ChannelMonitorRuntime{Enabled: true, Mode: ChannelMonitorModeCustom}).PassiveAggregationAllowed())
	require.False(t, (ChannelMonitorRuntime{Enabled: true, Mode: ChannelMonitorModeV2}).ActiveProbesAllowed())
}

func TestValidateEndpoint_LoopbackException(t *testing.T) {
	require.NoError(t, validateEndpoint("http://127.0.0.1:8080"))
	require.NoError(t, validateEndpoint("  http://127.0.0.1:8080/  "))

	for _, ep := range []string{
		"http://8.8.8.8",
		"https://127.0.0.1",
		"https://127.0.0.1:8080",
		"https://10.0.0.1",
		"https://169.254.169.254",
		"https://[::1]",
		"https://[fd00::1]",
		"http://127.0.0.1:8080/v1",
		"http://127.0.0.1",
		"http://127.0.0.1:80",
		"http://127.0.0.1:8081",
		"http://localhost:8080",
		"HTTP://127.0.0.1:8080",
		"http://127.0.0.1:8080?x=1",
		"http://127.0.0.1:8080#frag",
	} {
		t.Run(ep, func(t *testing.T) {
			require.Error(t, validateEndpoint(ep))
		})
	}
}

func TestSafeDial_LoopbackExceptionIsExact(t *testing.T) {
	require.True(t, isExactLoopbackExceptionDial("127.0.0.1", "8080"))
	require.False(t, isExactLoopbackExceptionDial("::ffff:127.0.0.1", "8080"))
	require.False(t, isExactLoopbackExceptionDial("127.0.0.1", "80"))
	require.False(t, isExactLoopbackExceptionDial("localhost", "8080"))

	require.True(t, isPrivateIP(net.ParseIP("127.0.0.1")))
	require.True(t, isPrivateIP(net.ParseIP("::ffff:127.0.0.1")))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := safeDialContext(ctx, "tcp", "127.0.0.1:1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "blocked by SSRF policy")

	_, err = safeDialContext(ctx, "tcp", "[::ffff:127.0.0.1]:8080")
	require.Error(t, err)
	require.Contains(t, err.Error(), "blocked by SSRF policy")

	_, err = safeDialContext(ctx, "tcp", "127.0.0.1:8080")
	if err != nil {
		require.NotContains(t, err.Error(), "blocked by SSRF policy")
	}
}

func TestCustomThresholdConstraint(t *testing.T) {
	require.NoError(t, validateCustomThresholds(2, 8, 0, false))
	require.NoError(t, validateCustomThresholds(2, 15, 15, true))
	require.Error(t, validateCustomThresholds(0, 8, 15, true))
	require.Error(t, validateCustomThresholds(8, 8, 15, true))
	require.Error(t, validateCustomThresholds(9, 8, 15, true))
	require.Error(t, validateCustomThresholds(2, 16, 15, true))
	require.Error(t, validateCustomThresholds(2, 3601, 0, false))
	require.Error(t, validateActualIntervalAgainstRed(14, 15))
	require.NoError(t, validateActualIntervalAgainstRed(15, 15))
}

func TestApplyProcessedTTFT(t *testing.T) {
	off := ttftPolicy{Enabled: false, Multiplier: 4, MinMs: 100, MaxMs: 200}
	got, err := applyProcessedTTFT(80, off)
	require.NoError(t, err)
	require.Equal(t, 80, got)

	on := ttftPolicy{Enabled: true, Multiplier: 2, MinMs: 100, MaxMs: 1000}
	got, err = applyProcessedTTFT(80, on)
	require.NoError(t, err)
	require.Equal(t, 160, got)

	outside := []int{1, 10, 100, 1000, 5000, 20000}
	prev := -1
	policy := ttftPolicy{Enabled: true, Multiplier: 1, MinMs: 100, MaxMs: 1000}
	for _, measured := range outside {
		got, err = applyProcessedTTFT(measured, policy)
		require.NoError(t, err)
		require.Greater(t, got, prev)
		prev = got
		if measured > 1000 {
			require.Less(t, got, measured)
			require.Greater(t, got, 1000)
		}
	}

	_, err = applyProcessedTTFT(10, ttftPolicy{Enabled: true, Multiplier: 1, MinMs: 20, MaxMs: 20})
	require.Error(t, err)
}

func TestHealthFromProcessedTTFT(t *testing.T) {
	require.Equal(t, MonitorStatusOperational, healthFromProcessedTTFT(1999, 2, 8, true))
	require.Equal(t, MonitorStatusDegraded, healthFromProcessedTTFT(2000, 2, 8, true))
	require.Equal(t, MonitorStatusDegraded, healthFromProcessedTTFT(7999, 2, 8, true))
	require.Equal(t, MonitorStatusFailed, healthFromProcessedTTFT(8000, 2, 8, true))
	require.Equal(t, MonitorStatusError, healthFromProcessedTTFT(10, 2, 8, false))
}

func TestDeriveTokensPerSec_StableAndNegative(t *testing.T) {
	fast := deriveTokensPerSec(7, "gpt", 100, 10, 100)
	slow := deriveTokensPerSec(7, "gpt", 8000, 10, 100)
	again := deriveTokensPerSec(7, "gpt", 100, 10, 100)
	require.InDelta(t, fast, again, 0.0001)
	require.Greater(t, fast, slow)
	require.GreaterOrEqual(t, fast, 10.0)
	require.Less(t, fast, 100.0)
	require.GreaterOrEqual(t, slow, 10.0)
	require.Less(t, slow, 100.0)
	require.NotEqual(t, fast, deriveTokensPerSec(8, "gpt", 100, 10, 100))
	require.Zero(t, deriveTokensPerSec(7, "gpt", 100, 10, 10))
}

func TestShouldHideMonitorMetrics(t *testing.T) {
	require.True(t, shouldHideMonitorMetrics(MonitorMetricKindTTFT, MonitorStatusFailed))
	require.True(t, shouldHideMonitorMetrics(MonitorMetricKindTTFT, MonitorStatusError))
	require.False(t, shouldHideMonitorMetrics(MonitorMetricKindTTFT, MonitorStatusDegraded))
	require.False(t, shouldHideMonitorMetrics(MonitorMetricKindLatency, MonitorStatusFailed))
	require.False(t, shouldHideMonitorMetrics(MonitorMetricKindQuota, MonitorStatusError))
}
