# 协作规范

本仓库是 `Wei-Shaw/sub2api` 的私有二开，不是 GitHub fork。开始前阅读 [AGENTS.md](AGENTS.md)。

交流、Issue、PR、Review 和合并说明使用简体中文。代码、标识符、路径保持英文。

当前 CI、发布边界和仓库设置以 [docs/ci-cd.md](docs/ci-cd.md) 为准。Runner 注册契约以 [docs/ci-runner.md](docs/ci-runner.md) 为准。

## Issue、分支、提交

每项人工变更先建 Issue，原则上一个 Issue 一个主 PR。已有 Issue 正文不回改。

新 Issue 标题 `<type>: <简短中文描述>`。正文为：目标 → 范围 → 非目标 → 完成定义 → 验证方式。模板见 [缺陷](.github/ISSUE_TEMPLATE/bug.md)、[功能](.github/ISSUE_TEMPLATE/feature.md)、[工程任务](.github/ISSUE_TEMPLATE/task.md)。空白 Issue 已关闭。

分支从最新 `origin/main` 拉出，格式 `<type>/<Issue编号>-<简短中文说明>`。

一个提交只含一个原子变更。标题 `<type>(<scope>): <简短中文描述>`，不超过 72 字符。没有合适 scope 时可以省略括号。

## PR 与合并

PR 目标 `main`。标题同样使用 `<type>: <简短中文描述>`。正文为：关联 Issue（`Closes #N`）→ 变更说明 → 验证结果 → 影响与限制。同步 PR 不写 `Closes`。

直接开正式 PR。不使用 Draft / Ready，也不把检查分成两阶段。

非作者至少一人批准。作者自己的 Review、Agent 自审和评论不算批准。`CODEOWNERS` 只提示维护者，不作为必过的代码所有者批准。

管理员必须走 PR，不能直推 `main`。他人审批可以在 PR 上绕过；PR 本身和已写入的必过检查不能绕过。必过检查的写入条件见 [docs/ci-cd.md](docs/ci-cd.md)。

合并只用 Merge Commit。

## 授权

用户明确要求提交、推送或创建 PR 时，才执行对应动作。

代推发布 tag 需要用户确认该次发布预览。合并、解决冲突、合并前整理分支，由用户自己做，或单独授权 Agent 做这一次。每日同步身份不做这些。

删除远程分支、修改保护规则、`git push --force-with-lease`、`git reset --hard`、删除镜像、移动或删除 tag，都要另行授权。禁止 `git push --force`。

## 发布与部署

发镜像、部署生产、合并代码，三者分开确认。本仓库不部署生产。发布现状以 [docs/ci-cd.md](docs/ci-cd.md) 为准，不要把尚未落地的发布规则写成当前工作流已经具备。
