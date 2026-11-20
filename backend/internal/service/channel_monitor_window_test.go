//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAvailabilityWindows_Default(t *testing.T) {
	got, err := parseAvailabilityWindows("")
	require.NoError(t, err)
	require.Equal(t, []AvailabilityWindow{
		{Amount: 90, Unit: "m"},
		{Amount: 1, Unit: "d"},
		{Amount: 3, Unit: "d"},
		{Amount: 7, Unit: "d"},
	}, got)
	require.Equal(t, 90*60, got[0].Seconds())
	require.Equal(t, 24*60*60, got[1].Seconds())
}

func TestParseAvailabilityWindows_Custom(t *testing.T) {
	got, err := parseAvailabilityWindows("15m,1d,5d")
	require.NoError(t, err)
	require.Equal(t, []AvailabilityWindow{
		{Amount: 15, Unit: "m"},
		{Amount: 1, Unit: "d"},
		{Amount: 5, Unit: "d"},
	}, got)
	require.Equal(t, "15m", got[0].Key())
	require.Equal(t, 15*60, got[0].Seconds())
	require.Equal(t, 24*60*60, got[1].Seconds())
}

func TestParseAvailabilityWindows_RejectsInvalid(t *testing.T) {
	for _, raw := range []string{
		"0d", "91m", "31d", "7h", "24h", "7D", "15m,15m",
		"1m,2m,3m,4m,5m,6m,7m",
	} {
		_, err := parseAvailabilityWindows(raw)
		require.ErrorIs(t, err, ErrChannelMonitorInvalidWindows, raw)
	}
}

func TestParseAvailabilityWindows_NormalizesOrder(t *testing.T) {
	got, err := parseAvailabilityWindows(" 5d, 15m ")
	require.NoError(t, err)
	require.Equal(t, "15m,5d", formatAvailabilityWindows(got))
}
