-- 账号管理 #2 的模拟数据。与真实账号、分组、代理表完全独立，不被任何真实业务查询引用。
-- 回滚：DROP TABLE account_sim_accounts, account_sim_meta;
CREATE TABLE IF NOT EXISTS account_sim_accounts (
    id         BIGINT PRIMARY KEY,
    data       JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- key 为 state（分组、代理、ID 计数与演进游标）或 config（全部配置项）。
CREATE TABLE IF NOT EXISTS account_sim_meta (
    key        VARCHAR(32) PRIMARY KEY,
    value      JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
