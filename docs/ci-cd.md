# CI 与发布边界

本文件只写当前已经运行的行为，以及尚未落地的仓库设置。工作流文件是执行真相。尚未合入的改造不写成已经生效。

## 当前检查

`CI`（`.github/workflows/backend-ci.yml`）在任意 `push` 和 `pull_request` 上运行：

| Job | 运行位置 | 内容 |
| --- | --- | --- |
| `shell` | `macos-15` | 部署脚本语法与相关测试。`deploy/tests/apple-container-test.sh` 使用 BSD `stat -f`，所以留在 macOS |
| `test` | `ubuntu-latest` | Go 单元测试和集成测试。集成测试需要 Docker |
| `frontend` | `ubuntu-latest` | 前端类型检查和关键测试 |
| `golangci-lint` | `ubuntu-latest` | 后端 lint |
| `release-helpers` | `ubuntu-latest` | 发布辅助脚本的单元测试和 `bash -n` |

同一 PR 的新推送不会取消旧运行。工作流末尾还没有 `pr-status`。

`Security Scan` 在任意 `push`、`pull_request` 和每周一 `03:00 UTC` 运行。末尾还没有 `security-status`。

`cla.yml` 只在 `github.repository == 'Wei-Shaw/sub2api'` 时执行。本仓不把它列为必过检查。冲突解决时保留这行守卫。

## 发布

`release.yml` 由 `v*` tag 和已有的 `workflow_dispatch` 触发。本仓还没有 GitHub Release，也没有 GHCR 包。

镜像发布到此为止。`baima-api` 自己选择版本并部署。本仓库不改生产。

fork 版本号、浮动标签守卫、镜像三态和应用内更新拦截尚未进入工作流。它们属于后续 Issue，不在本文假装已经生效。官方 `dry_run` 按 `backend/cmd/server/VERSION` 做构建验证，不代表 fork 预览。

## 仓库设置

合并方式、两层 `main` 保护和两套 `v*` tag 规则已经生效。必过检查尚未写入。发布 App 尚未配置。

- 只允许 Merge Commit。squash、rebase 和自动合并已关闭。合并后删除 head。
- 第一层绕过名单为空：禁止删除 `main`、禁止 force-push、必须经 PR，合并方式只有 merge。这一层还没有必过检查。
- 第二层：管理员只能在 PR 上绕过他人审批。至少 1 名非作者批准，新提交使旧批准失效，评审对话要解决，合并方式只有 merge。
- `pr-status` 和 `security-status` 要等对应 job 真实出现、并能变红变绿后，再写入第一层。
- 创建 `v*` tag 目前只允许维护者绕过。修改和删除只允许维护者绕过。发布 App 建立后才加入创建名单，不加入修改和删除名单。
- 同步身份不进入任何绕过名单。
