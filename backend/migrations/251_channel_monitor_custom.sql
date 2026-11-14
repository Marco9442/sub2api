-- Custom 渠道监控：历史行区分 V1 总耗时与 Custom TTFT，并保存单条监控的指标覆盖。
-- 回滚（仅本地演练，不作为正向迁移执行）：
--   DELETE FROM schema_migrations WHERE filename = '251_channel_monitor_custom.sql';
--   ALTER TABLE channel_monitor_histories DROP CONSTRAINT IF EXISTS channel_monitor_histories_metric_kind_check;
--   ALTER TABLE channel_monitor_histories DROP COLUMN IF EXISTS tokens_per_sec;
--   ALTER TABLE channel_monitor_histories DROP COLUMN IF EXISTS metric_kind;
--   DROP TABLE IF EXISTS channel_monitor_custom_overrides;

ALTER TABLE channel_monitor_histories
    ADD COLUMN IF NOT EXISTS metric_kind VARCHAR(16) NOT NULL DEFAULT 'latency';

ALTER TABLE channel_monitor_histories
    ADD COLUMN IF NOT EXISTS tokens_per_sec DOUBLE PRECISION;

-- 纯配额行没有延迟，且带 quota 快照。失败的探活也可能没有延迟，不能只凭 latency 为空改写。
UPDATE channel_monitor_histories
SET metric_kind = 'quota'
WHERE latency_ms IS NULL
  AND quota IS NOT NULL
  AND metric_kind = 'latency';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'channel_monitor_histories_metric_kind_check'
    ) THEN
        ALTER TABLE channel_monitor_histories
            ADD CONSTRAINT channel_monitor_histories_metric_kind_check
            CHECK (metric_kind IN ('latency', 'ttft', 'quota'));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS channel_monitor_custom_overrides (
    monitor_id BIGINT PRIMARY KEY REFERENCES channel_monitors(id) ON DELETE CASCADE,
    ttft_adjust_enabled BOOLEAN,
    ttft_multiplier DOUBLE PRECISION,
    ttft_min_ms INTEGER,
    ttft_max_ms INTEGER,
    tps_enabled BOOLEAN,
    tps_min DOUBLE PRECISION,
    tps_max DOUBLE PRECISION,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
