//go:build unit

package admin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCustomConfigFromRequestKeepsAvailabilityWindows(t *testing.T) {
	cfg := customConfigFromRequest(channelMonitorCustomConfigRequest{
		Prompt:        "ping",
		YellowSeconds: 2,
		RedSeconds:    8,
		AvailabilityWindows: []availabilityWindowRequest{
			{Amount: 15, Unit: "m"},
			{Amount: 1, Unit: "d"},
		},
	})

	require.Equal(t, 2, len(cfg.AvailabilityWindows))
	require.Equal(t, 15, cfg.AvailabilityWindows[0].Amount)
	require.Equal(t, "m", cfg.AvailabilityWindows[0].Unit)
	require.Equal(t, 1, cfg.AvailabilityWindows[1].Amount)
	require.Equal(t, "d", cfg.AvailabilityWindows[1].Unit)
}

func TestCustomConfigFromRequestLeavesWindowsEmptyWhenOmitted(t *testing.T) {
	cfg := customConfigFromRequest(channelMonitorCustomConfigRequest{
		Prompt:        "ping",
		YellowSeconds: 2,
		RedSeconds:    8,
	})

	require.Empty(t, cfg.AvailabilityWindows)
}
