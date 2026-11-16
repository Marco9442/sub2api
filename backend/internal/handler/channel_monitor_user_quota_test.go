//go:build unit

package handler

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserMonitorViewOmitsQuotaError(t *testing.T) {
	source := &domain.MonitorQuotaSnapshot{
		Source:            "cn_balance",
		Success:           false,
		Error:             "Authentication failed (HTTP 401)",
		CredentialInvalid: true,
	}
	view := &service.UserMonitorView{LatestQuota: source}

	item := userMonitorViewToItem(view, true)

	require.NotNil(t, item.LatestQuota)
	require.False(t, item.LatestQuota.Success)
	require.Equal(t, "cn_balance", item.LatestQuota.Source)
	require.Empty(t, item.LatestQuota.Error)
	require.False(t, item.LatestQuota.CredentialInvalid)
	require.Equal(t, "Authentication failed (HTTP 401)", source.Error)
	require.True(t, source.CredentialInvalid)
	require.NotSame(t, source, item.LatestQuota)

	raw, err := json.Marshal(item.LatestQuota)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "credential_invalid")
	require.NotContains(t, string(raw), "Authentication failed")
}

func TestUserMonitorViewKeepsQuotaHiddenWhenSwitchOff(t *testing.T) {
	view := &service.UserMonitorView{LatestQuota: &domain.MonitorQuotaSnapshot{
		Success: false,
		Error:   "Authentication failed (HTTP 401)",
	}}

	item := userMonitorViewToItem(view, false)

	require.Nil(t, item.LatestQuota)
}
