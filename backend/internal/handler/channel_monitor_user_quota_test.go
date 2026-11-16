//go:build unit

package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserMonitorViewOmitsQuotaError(t *testing.T) {
	source := &domain.MonitorQuotaSnapshot{
		Source:  "cn_balance",
		Success: false,
		Error:   "Authentication failed (HTTP 401)",
	}
	view := &service.UserMonitorView{LatestQuota: source}

	item := userMonitorViewToItem(view, true)

	require.NotNil(t, item.LatestQuota)
	require.False(t, item.LatestQuota.Success)
	require.Equal(t, "cn_balance", item.LatestQuota.Source)
	require.Empty(t, item.LatestQuota.Error)
	require.Equal(t, "Authentication failed (HTTP 401)", source.Error)
	require.NotSame(t, source, item.LatestQuota)
}

func TestUserMonitorViewKeepsQuotaHiddenWhenSwitchOff(t *testing.T) {
	view := &service.UserMonitorView{LatestQuota: &domain.MonitorQuotaSnapshot{
		Success: false,
		Error:   "Authentication failed (HTTP 401)",
	}}

	item := userMonitorViewToItem(view, false)

	require.Nil(t, item.LatestQuota)
}
