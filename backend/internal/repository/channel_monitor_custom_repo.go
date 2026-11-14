package repository

import (
	"context"
	"database/sql"
	"fmt"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/channelmonitorhistory"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func (r *channelMonitorRepository) InsertHistoryBatch(ctx context.Context, rows []*service.ChannelMonitorHistoryRow) error {
	if len(rows) == 0 {
		return nil
	}
	if dbent.TxFromContext(ctx) != nil {
		return r.insertHistoryBatch(ctx, clientFromContext(ctx, r.client), rows)
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin history tx: %w", err)
	}
	if err := r.insertHistoryBatch(ctx, tx.Client(), rows); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit history tx: %w", err)
	}
	return nil
}

func (r *channelMonitorRepository) insertHistoryBatch(ctx context.Context, client *dbent.Client, rows []*service.ChannelMonitorHistoryRow) error {
	bulk := make([]*dbent.ChannelMonitorHistoryCreate, 0, len(rows))
	for _, row := range rows {
		c := client.ChannelMonitorHistory.Create().
			SetMonitorID(row.MonitorID).
			SetModel(row.Model).
			SetStatus(channelmonitorhistory.Status(row.Status)).
			SetMessage(row.Message).
			SetCheckedAt(row.CheckedAt)
		if row.LatencyMs != nil {
			c = c.SetLatencyMs(*row.LatencyMs)
		}
		if row.PingLatencyMs != nil {
			c = c.SetPingLatencyMs(*row.PingLatencyMs)
		}
		if row.Quota != nil {
			c = c.SetQuota(row.Quota)
		}
		bulk = append(bulk, c)
	}
	saved, err := client.ChannelMonitorHistory.CreateBulk(bulk...).Save(ctx)
	if err != nil {
		return fmt.Errorf("insert history bulk: %w", err)
	}
	return annotateHistoryMetrics(ctx, client, saved, rows)
}

func annotateHistoryMetrics(ctx context.Context, client *dbent.Client, saved []*dbent.ChannelMonitorHistory, rows []*service.ChannelMonitorHistoryRow) error {
	for i, row := range rows {
		if row == nil || i >= len(saved) || saved[i] == nil {
			continue
		}
		kind := row.MetricKind
		if kind == "" {
			kind = service.MonitorMetricKindLatency
		}
		if kind == service.MonitorMetricKindLatency && row.TokensPerSec == nil {
			continue
		}
		if _, err := client.ExecContext(ctx,
			`UPDATE channel_monitor_histories SET metric_kind = $2, tokens_per_sec = $3 WHERE id = $1`,
			saved[i].ID, kind, row.TokensPerSec,
		); err != nil {
			return fmt.Errorf("annotate history metrics: %w", err)
		}
	}
	return nil
}

func (r *channelMonitorRepository) attachHistoryMetrics(ctx context.Context, entries []*service.ChannelMonitorHistoryEntry) error {
	if len(entries) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(entries))
	for _, entry := range entries {
		if entry != nil {
			ids = append(ids, entry.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := clientFromContext(ctx, r.client).QueryContext(ctx,
		`SELECT id, metric_kind, tokens_per_sec FROM channel_monitor_histories WHERE id = ANY($1)`,
		pq.Array(ids),
	)
	if err != nil {
		return fmt.Errorf("load history metrics: %w", err)
	}
	defer func() { _ = rows.Close() }()

	byID := make(map[int64]historyMetric, len(ids))
	for rows.Next() {
		var id int64
		var metric historyMetric
		if err := rows.Scan(&id, &metric.Kind, &metric.Tokens); err != nil {
			return fmt.Errorf("scan history metrics: %w", err)
		}
		byID[id] = metric
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		metric, ok := byID[entry.ID]
		if !ok {
			entry.MetricKind = service.MonitorMetricKindLatency
			continue
		}
		entry.MetricKind = metric.Kind
		assignNullFloat(&entry.TokensPerSec, metric.Tokens)
	}
	return nil
}

type historyMetric struct {
	Kind   string
	Tokens sql.NullFloat64
}

func (r *channelMonitorRepository) GetCustomOverride(ctx context.Context, id int64) (*service.ChannelMonitorCustomOverride, error) {
	found, err := r.ListCustomOverrides(ctx, []int64{id})
	if err != nil {
		return nil, err
	}
	return found[id], nil
}

func (r *channelMonitorRepository) ListCustomOverrides(ctx context.Context, ids []int64) (map[int64]*service.ChannelMonitorCustomOverride, error) {
	out := map[int64]*service.ChannelMonitorCustomOverride{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := clientFromContext(ctx, r.client).QueryContext(ctx, `
		SELECT monitor_id, ttft_adjust_enabled, ttft_multiplier, ttft_min_ms, ttft_max_ms,
		       tps_enabled, tps_min, tps_max
		FROM channel_monitor_custom_overrides
		WHERE monitor_id = ANY($1)
	`, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("list custom overrides: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var monitorID int64
		var adjust, tpsEnabled sql.NullBool
		var multiplier, tpsMin, tpsMax sql.NullFloat64
		var minMs, maxMs sql.NullInt64
		if err := rows.Scan(&monitorID, &adjust, &multiplier, &minMs, &maxMs, &tpsEnabled, &tpsMin, &tpsMax); err != nil {
			return nil, fmt.Errorf("scan custom override: %w", err)
		}
		out[monitorID] = overrideFromNulls(adjust, multiplier, minMs, maxMs, tpsEnabled, tpsMin, tpsMax)
	}
	return out, rows.Err()
}

func (r *channelMonitorRepository) UpsertCustomOverride(ctx context.Context, id int64, ov *service.ChannelMonitorCustomOverride) error {
	if ov == nil {
		return nil
	}
	_, err := clientFromContext(ctx, r.client).ExecContext(ctx, `
		INSERT INTO channel_monitor_custom_overrides (
		    monitor_id, ttft_adjust_enabled, ttft_multiplier, ttft_min_ms, ttft_max_ms,
		    tps_enabled, tps_min, tps_max, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (monitor_id) DO UPDATE SET
		    ttft_adjust_enabled = EXCLUDED.ttft_adjust_enabled,
		    ttft_multiplier = EXCLUDED.ttft_multiplier,
		    ttft_min_ms = EXCLUDED.ttft_min_ms,
		    ttft_max_ms = EXCLUDED.ttft_max_ms,
		    tps_enabled = EXCLUDED.tps_enabled,
		    tps_min = EXCLUDED.tps_min,
		    tps_max = EXCLUDED.tps_max,
		    updated_at = NOW()
	`, id, ov.TTFTAdjustEnabled, ov.TTFTMultiplier, ov.TTFTMinMs, ov.TTFTMaxMs, ov.TPSEnabled, ov.TPSMin, ov.TPSMax)
	if err != nil {
		return fmt.Errorf("upsert custom override: %w", err)
	}
	return nil
}

func (r *channelMonitorRepository) DeleteCustomOverride(ctx context.Context, id int64) error {
	if _, err := clientFromContext(ctx, r.client).ExecContext(ctx,
		`DELETE FROM channel_monitor_custom_overrides WHERE monitor_id = $1`, id,
	); err != nil {
		return fmt.Errorf("delete custom override: %w", err)
	}
	return nil
}

func (r *channelMonitorRepository) ShortestActualInterval(ctx context.Context) (int, bool, error) {
	rows, err := clientFromContext(ctx, r.client).QueryContext(ctx,
		`SELECT MIN(interval_seconds - COALESCE(jitter_seconds, 0)) FROM channel_monitors`,
	)
	if err != nil {
		return 0, false, fmt.Errorf("shortest monitor interval: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var shortest sql.NullInt64
	if !rows.Next() {
		return 0, false, rows.Err()
	}
	if err := rows.Scan(&shortest); err != nil {
		return 0, false, fmt.Errorf("scan shortest monitor interval: %w", err)
	}
	if !shortest.Valid {
		return 0, false, nil
	}
	return int(shortest.Int64), true, nil
}

func (r *channelMonitorRepository) ComputeAvailabilityKind(ctx context.Context, monitorID int64, windowDays int, kind string) ([]*service.ChannelMonitorAvailability, error) {
	if windowDays <= 0 {
		windowDays = 7
	}
	rows, err := r.db.QueryContext(ctx, availabilityQuery(kind, false), monitorID, windowDays)
	if err != nil {
		return nil, fmt.Errorf("query availability: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]*service.ChannelMonitorAvailability, 0)
	for rows.Next() {
		row, err := scanAvailabilityRow(rows, windowDays)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *channelMonitorRepository) ComputeAvailabilityForMonitorsKind(ctx context.Context, ids []int64, windowDays int, kind string) (map[int64][]*service.ChannelMonitorAvailability, error) {
	out := make(map[int64][]*service.ChannelMonitorAvailability, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	if windowDays <= 0 {
		windowDays = 7
	}
	rows, err := r.db.QueryContext(ctx, availabilityQuery(kind, true), pq.Array(ids), windowDays)
	if err != nil {
		return nil, fmt.Errorf("query availability batch: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var monitorID int64
		row := &service.ChannelMonitorAvailability{WindowDays: windowDays}
		var avgLatency sql.NullFloat64
		if err := rows.Scan(&monitorID, &row.Model, &row.TotalChecks, &row.OperationalChecks, &avgLatency); err != nil {
			return nil, fmt.Errorf("scan availability batch row: %w", err)
		}
		finalizeAvailabilityRow(row, avgLatency)
		out[monitorID] = append(out[monitorID], row)
	}
	return out, rows.Err()
}

func availabilityQuery(kind string, batch bool) string {
	where := "monitor_id = $1"
	idCol := ""
	group := "GROUP BY model"
	if batch {
		where = "monitor_id = ANY($1)"
		idCol = "monitor_id,"
		group = "GROUP BY monitor_id, model"
	}
	return fmt.Sprintf(`
		SELECT %s
		       model,
		       COUNT(*) AS total,
		       COUNT(*) FILTER (WHERE status IN ('operational','degraded')) AS ok,
		       %s AS avg_latency_ms
		FROM channel_monitor_histories
		WHERE %s
		  AND checked_at >= NOW() - ($2::int || ' days')::interval
		  AND %s
		%s
	`, idCol, availabilityAvgExpr(kind), where, availabilityKindPredicate(kind), group)
}

// availabilityKindPredicate 把可用率统计限制在当前模式的指标上。
// Custom 只计 TTFT，并保留配额行；V1 排除 TTFT，避免两种耗时混进同一百分比。
const channelMonitorLatencyKindSQL = "metric_kind IS DISTINCT FROM 'ttft'"

func availabilityKindPredicate(kind string) string {
	if kind == service.MonitorMetricKindTTFT {
		return "metric_kind IN ('ttft', 'quota')"
	}
	return channelMonitorLatencyKindSQL
}

func availabilityAvgExpr(kind string) string {
	if kind == service.MonitorMetricKindTTFT {
		return `CASE WHEN COUNT(latency_ms) FILTER (WHERE metric_kind = 'ttft' AND status IN ('operational','degraded')) > 0
		        THEN SUM(latency_ms) FILTER (WHERE metric_kind = 'ttft' AND status IN ('operational','degraded'))::float8
		             / COUNT(latency_ms) FILTER (WHERE metric_kind = 'ttft' AND status IN ('operational','degraded'))
		        ELSE NULL END`
	}
	return fmt.Sprintf(`CASE WHEN COUNT(latency_ms) FILTER (WHERE %s) > 0
	        THEN SUM(latency_ms) FILTER (WHERE latency_ms IS NOT NULL AND %s)::float8
	             / COUNT(latency_ms) FILTER (WHERE %s)
	        ELSE NULL END`, channelMonitorLatencyKindSQL, channelMonitorLatencyKindSQL, channelMonitorLatencyKindSQL)
}

func overrideFromNulls(adjust sql.NullBool, multiplier sql.NullFloat64, minMs, maxMs sql.NullInt64, tpsEnabled sql.NullBool, tpsMin, tpsMax sql.NullFloat64) *service.ChannelMonitorCustomOverride {
	ov := &service.ChannelMonitorCustomOverride{}
	if adjust.Valid {
		v := adjust.Bool
		ov.TTFTAdjustEnabled = &v
	}
	if multiplier.Valid {
		v := multiplier.Float64
		ov.TTFTMultiplier = &v
	}
	if minMs.Valid {
		v := int(minMs.Int64)
		ov.TTFTMinMs = &v
	}
	if maxMs.Valid {
		v := int(maxMs.Int64)
		ov.TTFTMaxMs = &v
	}
	if tpsEnabled.Valid {
		v := tpsEnabled.Bool
		ov.TPSEnabled = &v
	}
	if tpsMin.Valid {
		v := tpsMin.Float64
		ov.TPSMin = &v
	}
	if tpsMax.Valid {
		v := tpsMax.Float64
		ov.TPSMax = &v
	}
	return ov
}

func assignNullFloat(dst **float64, n sql.NullFloat64) {
	if !n.Valid {
		return
	}
	v := n.Float64
	*dst = &v
}

func scanMetric(kind *string, tokens **float64, rawKind sql.NullString, rawTokens sql.NullFloat64) {
	if kind != nil {
		if rawKind.Valid && rawKind.String != "" {
			*kind = rawKind.String
		} else {
			*kind = service.MonitorMetricKindLatency
		}
	}
	assignNullFloat(tokens, rawTokens)
}
