# AGENTS.md

本文件只做规则路由。细则不在这里重复。

## 规则路由

| 规则域 | 唯一细则来源 |
| --- | --- |
| Issue、分支、提交、PR、Review、合并、授权 | [CONTRIBUTING.md](CONTRIBUTING.md) |
| 当前 CI、发布边界、仓库设置 | [docs/ci-cd.md](docs/ci-cd.md) |
| 本仓 Runner 注册契约 | [docs/ci-runner.md](docs/ci-runner.md) |

`DEV_GUIDE.md` 和上游 README 不是本仓规则源。不要为对齐规则去改它们。

## 阅读路由

先看任务，再打开对应文件。不要从根 README 或 `DEV_GUIDE.md` 开始。

| 任务 | 读 |
| --- | --- |
| 产品功能 | [README.md](README.md)。中文独有节在 [README_CN.md](README_CN.md)：Codex Fast/Flex、网关防御纵深、HTTP/2 (h2c)、OpenAI 图片模型 |
| 安装、Compose、二进制、升级 | [deploy/README.md](deploy/README.md)。脚本指向 `Wei-Shaw/sub2api`，不是本仓发布 |
| Apple container | [deploy/APPLE_CONTAINER.md](deploy/APPLE_CONTAINER.md) |
| 反代、可信 IP、请求体 | [deploy/EDGE_SECURITY.md](deploy/EDGE_SECURITY.md) |
| 数据管理 | [deploy/DATAMANAGEMENTD_CN.md](deploy/DATAMANAGEMENTD_CN.md) |
| 改请求、流、计费、缓存 | [docs/agent-resource-safety.md](docs/agent-resource-safety.md) |
| 后台支付配置 | [docs/PAYMENT.md](docs/PAYMENT.md) 或 [docs/PAYMENT_CN.md](docs/PAYMENT_CN.md) |
| 外部充值对接 | [docs/ADMIN_PAYMENT_INTEGRATION_API.md](docs/ADMIN_PAYMENT_INTEGRATION_API.md) |
| 插件 | [backend/pkg/pluginapi/README.md](backend/pkg/pluginapi/README.md)。教程 [docs/PLUGIN_DEVELOPMENT.md](docs/PLUGIN_DEVELOPMENT.md) 不是协议源 |
| 管理 CLI | [skills/sub2api-admin/SKILL.md](skills/sub2api-admin/SKILL.md) |
| 仓库许可 | 根目录 [LICENSE](LICENSE)，AGPL-3.0。核对全文时才打开 |

不要读：`README_JA.md`、`DEV_GUIDE.md`、`deploy/DOCKER.md` 的 Tags、`openspec/changes/`（已落地的历史规格，行为以代码和测试为准）、`docs/legal/`（运行时原文，不要当规则改）、`.github/SECURITY.md`（报告入口仍指向 `Wei-Shaw/sub2api`）、`.github/release-tools/README.md` 的 `dry_run`（发布语义以 `docs/ci-cd.md` 为准）。

功能专题在改对应功能时再打开 `docs/` 下同名文件，不从这里进入。
