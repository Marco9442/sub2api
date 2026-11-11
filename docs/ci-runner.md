# 本仓 Runner

本文件写注册契约。在线状态以 GitHub Actions 的 runner 列表和 #3 的冒烟记录为准。本文不声称 runner 已经在线。

## 契约

- 在现有主机新增 1 个只属于本仓库的注册，名称 `evo-sub2api-1`，标签 `sub2api-ci`。
- 独立工作目录和凭据。不使用 `evo-runner`，也不复用 clodex 或 clodex-client 的 runner 进程。
- 沿用用户 `gha` 和主机现有磁盘清理，不改清理脚本，不新建用户，不加 cgroup。
- 重 job 超时 45–60 分钟。容器名和项目名带 `sub2api` 与 runner 名。
- 冒烟用一次临时 `workflow_dispatch`，证明任务落在 `evo-sub2api-1`、Docker 可用、没有占用其他仓库的 runner。冒烟工作流不留在仓库，结果记在 Issue。
- `runs-on: sub2api-ci` 只能在冒烟通过之后合入。排队成为常态再加第 2 个，不预建。
