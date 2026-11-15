//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildStatusSummary_NoHistoryLeavesAvailabilityNil(t *testing.T) {
	summary := buildStatusSummary(map[string]*ChannelMonitorLatest{}, map[string]*ChannelMonitorAvailability{}, "gpt-5", nil)
	require.Nil(t, summary.Availability7d)
}

func TestWindowAvailability_NilWhenNoChecks(t *testing.T) {
	got := windowAvailability(map[string]*ChannelMonitorAvailability{}, "gpt-5")
	require.Nil(t, got)
}

func TestWindowAvailability_KeepsZero(t *testing.T) {
	avail := map[string]*ChannelMonitorAvailability{
		"gpt-5": {Model: "gpt-5", TotalChecks: 3, AvailabilityPct: 0},
	}
	got := windowAvailability(avail, "gpt-5")
	require.NotNil(t, got)
	require.Equal(t, 0.0, *got)
}
