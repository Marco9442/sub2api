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
| `release-helpers` | `ubuntu-latest` | 发布辅助脚本、上游同步决策的单元测试，以及 `bash -n` |

同一 PR 的新推送不会取消旧运行。工作流末尾还没有 `pr-status`。

`Security Scan` 在任意 `push`、`pull_request` 和每周一 `03:00 UTC` 运行。末尾还没有 `security-status`。

`cla.yml` 只在 `github.repository == 'Wei-Shaw/sub2api'` 时执行。本仓不把它列为必过检查。冲突解决时保留这行守卫。

本地测试与 CI 的 `release-helpers` 相同，在仓库根执行。先安装 `.github/release-tools/requirements-release.txt`：

```bash
python -m unittest discover -s .github/release-tools -p 'test_release_matrix.py'
python -m unittest discover -s .github/upstream-sync -p 'test_plan.py'
bash -n .github/release-tools/release-images.sh
```

不要把测试文件路径直接传给 `python -m unittest`。单个测试进入对应目录后指定 `模块.类.方法`：

```bash
cd .github/upstream-sync && python3 -m unittest test_plan.WorkflowContractTest.<method>
```

`unittest -k` 会跑出 0 个测试。`bash -n` 使用 bash 4 或更高；macOS 自带的 `/bin/bash` 是 3.2。Go 测试在 `backend` 目录执行。

`TestFilterThinkingBlocksSignedHistorySkipsDecode`、`TestInflightEstimate_AccountMappingNoDBAndBoundedMemory`，以及 `TestEasyPayRefundResponseErrors` 的偶发失败，保持原断言。

## 上游同步

`Upstream Sync`（`.github/workflows/upstream-sync.yml`）每天 `02:17 UTC` 运行，也可以手动触发。它只查看 `Wei-Shaw/sub2api` 的最新正式 Release，把该 Release 指向的源码提交提案到长期分支 `sync/upstream`。

PR 附版本说明和相对 `main` 的改动范围。不试合并，不生成冲突清单，不合并，不解决冲突，不推 tag，不发镜像。冲突可以留在 PR 上，任务仍然成功。

已经包含在 `main` 的正式 Release、draft、prerelease，以及没有正式 Release 的上游提交，都不产生 PR。落后多个正式 Release 时只提案最新一个。

同一 Release 已有整理提交时，本次不推送分支。更新的正式 Release 若没有整理提交则快进；已有整理提交，或新 Release 不是当前提案的后继，则只评论、不覆盖。

推送身份是仅安装在本仓的 GitHub App `sub2api-upstream-sync`。它只有 contents、pull requests、workflows 的写权限，不在任何绕过名单。作业 `GITHUB_TOKEN` 永远没有 workflows 权限，只用于检出和读取上游公开 Release。shell 不会自动带上它，读取时必须在步骤里显式传入。私钥放在 environment `upstream-sync`，部署分支只有 `main`。App 推送分支和打开 PR 会触发其他 workflow。

读取用的作业 `GITHUB_TOKEN` 和推送用的 App token 必须是两个值。作业 token 不能推送 workflow 文件，也不能推 `v*` tag。权限键写 `permission-workflows:`。行首缩进的 `workflows:` 不是权限键。

`sync/upstream` 不追加空提交。同步 PR 不改上游工具链，也不放宽 `govulncheck`。GitHub cron 可能晚于 `02:17 UTC`。延迟本身不用再手动触发。最新正式版已经在 `main` 时，运行正常结束，不开新 PR。

用户令牌访问 `repos/Marco9442/sub2api/installation` 会得到 401。该接口要 App JWT，不能当成安装丢失。App 是否可用，以 `upstream-sync` 作业能否取出 token 为准。

## 发布

`release.yml` 由 `v*` tag 和已有的 `workflow_dispatch` 触发。推 `v*` tag 时，运行的是该 tag 上的 workflow 和 `.github/release-tools`。守卫先进入这个提交。

`v0.2.21` 和 `v0.2.21-fork.1` 已经发布。`v0.2.21-fork.1` 打在 `backend/cmd/server/VERSION` 仍为 `0.2.21` 的提交上。GHCR 的 `0.2.21`、`latest`、`0.2`、`0` 仍是 `v0.2.21` 那一次发布。下一次同一 `VERSION` 的 fork tag 是 `v0.2.21-fork.2`。上游版本变化后，序号从 `1` 起，不补零。

仓库里的版本文件只有 `backend/cmd/server/VERSION`。fork 发布不把 `X.Y.Z-fork.N` 写进这个文件。正式发布时，`plan()` 把 tag 版本写进作业工作区的该文件，并上传 `version-file`。git 里的文件保持上游版本。

fork 发布不移动 `latest`、major、major.minor。镜像三态是：目标标签全部不存在才构建上传；全部存在，且每个标签（含精确版本标签的每个平台）的来源、提交、架构一致才跳过；缺一部分或不一致则失败。只有标签本身不存在才算缺失。`image_state.py` 把 `{ref}: not found`，或不含 `sha256:` 的 `manifest unknown`，当成缺失。错误含 `content at ` 或 `sha256:` 时失败。`image linux/amd64 not found` 同样失败。

缺一部分时保持失败。恢复用同一个 git tag 再跑一次。不递增 fork 序号。删除 GHCR 包要另行授权，缺标签本身不是删除理由。

核对公开镜像用 `docker buildx imagetools inspect --format '{{json .}}'`。没有 `read:packages` 时，列包 API 返回 403。核对镜像不加这个 scope。多架构标签的 `revision` 和 `source` 在 `image["linux/amd64"].config.Labels` 与 `image["linux/arm64"].config.Labels`。单架构标签在扁平的 `image.config.Labels`。版本标签的 digest 是 index，架构标签的 digest 是单平台镜像。

版本含 `-fork.` 时，应用内更新和回滚关闭。本仓不是 `Wei-Shaw/sub2api` 时，版本同步直接跳过。官方 `dry_run` 会检出传入的 ref，版本字符串仍取该检出里的 `backend/cmd/server/VERSION`。它只做构建验证，不代表 fork 预览。仓库 Actions variables 里没有 `SIMPLE_RELEASE`。fork 发布走完整矩阵。

两份 GoReleaser 配置都是 `prerelease: auto`。含 `-` 的 tag 在 GitHub 上是 prerelease。`/releases/latest` 不返回它。`deploy/install.sh` 的 `GITHUB_REPO` 仍是 `Wei-Shaw/sub2api`。

推 `v*` tag 使用维护者身份和显式 refspec。不用作业 `GITHUB_TOKEN`。GitHub 提示 `Bypassed rule violations`，且内容是 creations being restricted 时，维护者绕过了创建限制，这次推送已经生效。

已完成作业的日志含 ANSI 时，`gh run view --log` 需要 `--allow-escape-sequences`。运行中的 job 还没有这份日志。

镜像发布到此为止。`baima-api` 自己选择版本并部署。本仓库不改生产。仓库公开后，GHCR 镜像也公开。

## 仓库设置

合并方式、两层 `main` 保护和两套 `v*` tag 规则已经生效。必过检查尚未写入。发布 App 尚未配置。

- 只允许 Merge Commit。squash、rebase 和自动合并已关闭。合并后删除 head。
- 第一层绕过名单为空：禁止删除 `main`、禁止 force-push、必须经 PR，合并方式只有 merge。这一层还没有必过检查。
- 第二层：管理员只能在 PR 上绕过他人审批。至少 1 名非作者批准，新提交使旧批准失效，评审对话要解决，合并方式只有 merge。
- `pr-status` 和 `security-status` 要等对应 job 真实出现、并能变红变绿后，再写入第一层。
- 创建 `v*` tag 目前只允许维护者绕过。修改和删除只允许维护者绕过。发布 App 建立后才加入创建名单，不加入修改和删除名单。
- 同步身份不进入任何绕过名单。个人访问令牌不写入仓库 secret。
- 关闭 `allow_forking` 的 API 返回 422。这项保持开启。
