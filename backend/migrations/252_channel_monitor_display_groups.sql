-- 渠道状态页展示分组。归属仍是 channel_monitors.group_name。
-- 回滚（仅本地演练，不作为正向迁移执行）：
--   DELETE FROM schema_migrations WHERE filename = '252_channel_monitor_display_groups.sql';
--   ALTER TABLE channel_monitors DROP COLUMN IF EXISTS display_sort;
--   DROP TABLE IF EXISTS channel_monitor_display_groups;

ALTER TABLE channel_monitors
    ADD COLUMN IF NOT EXISTS display_sort INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS channel_monitor_display_groups (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT channel_monitor_display_groups_name_not_blank
        CHECK (length(btrim(name)) > 0),
    CONSTRAINT channel_monitor_display_groups_name_key UNIQUE (name)
);

UPDATE channel_monitors
SET group_name = TRIM(group_name)
WHERE group_name <> TRIM(group_name);

INSERT INTO channel_monitor_display_groups (name, sort_order)
SELECT group_name, (ROW_NUMBER() OVER (ORDER BY MIN(id)) - 1)::int
FROM channel_monitors
WHERE group_name <> ''
GROUP BY group_name;

CREATE INDEX IF NOT EXISTS idx_channel_monitor_display_groups_sort
    ON channel_monitor_display_groups (sort_order, id);
