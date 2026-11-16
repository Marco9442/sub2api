//go:build unit

package admin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCustomConfigFromRequestDefaultsStatusPercents(t *testing.T) {
	cfg := customConfigFromRequest(channelMonitorCustomConfigRequest{
		Prompt:        "ping",
		YellowSeconds: 2,
		RedSeconds:    8,
	})

	require.Equal(t, 30, cfg.StatusYellowPercent)
	require.Equal(t, 30, cfg.StatusRedPercent)
}

func TestCustomConfigFromRequestKeepsStatusPercents(t *testing.T) {
	cfg := customConfigFromRequest(channelMonitorCustomConfigRequest{
		Prompt:              "ping",
		YellowSeconds:       2,
		RedSeconds:          8,
		StatusYellowPercent: 40,
		StatusRedPercent:    40,
	})

	require.Equal(t, 40, cfg.StatusYellowPercent)
	require.Equal(t, 40, cfg.StatusRedPercent)
}
