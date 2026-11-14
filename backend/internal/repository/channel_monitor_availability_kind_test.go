//go:build unit

package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAvailabilityKindPredicateSeparatesModes(t *testing.T) {
	require.Equal(t, "metric_kind IN ('ttft', 'quota')", availabilityKindPredicate(service.MonitorMetricKindTTFT))
	require.Equal(t, "metric_kind IS DISTINCT FROM 'ttft'", availabilityKindPredicate(service.MonitorMetricKindLatency))

	custom := availabilityQuery(service.MonitorMetricKindTTFT, false)
	require.Contains(t, custom, "AND metric_kind IN ('ttft', 'quota')")
	require.Contains(t, custom, "metric_kind = 'ttft' AND status IN ('operational','degraded')")

	v1 := availabilityQuery(service.MonitorMetricKindLatency, true)
	require.Contains(t, v1, "AND metric_kind IS DISTINCT FROM 'ttft'")
	require.NotContains(t, v1, "metric_kind IN ('ttft', 'quota')")
}
