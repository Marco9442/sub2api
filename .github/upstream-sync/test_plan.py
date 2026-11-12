import importlib.util
import json
import os
import re
import unittest
import unittest.mock
from pathlib import Path

spec = importlib.util.spec_from_file_location("plan", Path(__file__).with_name("plan.py"))
plan = importlib.util.module_from_spec(spec)
spec.loader.exec_module(plan)

COMMIT_A = "10ba8f9b4dc2e6023d4a9879ab77b21841babd10"
COMMIT_B = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"


def release(tag, published, **extra):
    item = {
        "tag_name": tag,
        "published_at": published,
        "draft": False,
        "prerelease": False,
        "name": tag,
        "body": f"notes for {tag}",
    }
    item.update(extra)
    return item


class ParseReleasesTest(unittest.TestCase):
    def test_parses_json_lines_from_gh_jq(self):
        raw = "\n".join(
            [
                json.dumps(release("v0.2.20", "2026-11-07T00:00:00Z")),
                json.dumps(release("v0.2.21", "2026-11-11T02:21:55Z")),
            ]
        )
        chosen = plan.select_latest(plan.parse_releases(raw))
        self.assertEqual(chosen["tag_name"], "v0.2.21")

    def test_flattens_paginated_arrays(self):
        raw = json.dumps(
            [
                [release("v0.2.20", "2026-11-07T00:00:00Z")],
                [release("v0.2.21", "2026-11-11T02:21:55Z")],
            ]
        )
        self.assertEqual(len(plan.parse_releases(raw)), 2)

    def test_apply_command_does_not_combine_slurp_and_jq(self):
        text = Path(__file__).with_name("plan.py").read_text()
        self.assertNotIn("--slurp", text)


class SelectLatestTest(unittest.TestCase):
    def test_ignores_draft_prerelease_and_non_release_tags(self):
        chosen = plan.select_latest(
            [
                release("v0.2.20", "2026-11-07T00:00:00Z"),
                release("v0.2.21-rc.1", "2026-11-12T00:00:00Z", prerelease=True),
                release("v0.2.22", "2026-11-13T00:00:00Z", draft=True),
                release("nightly", "2026-11-14T00:00:00Z"),
                release("v0.2.21", "2026-11-11T02:21:55Z"),
            ]
        )
        self.assertEqual(chosen["tag_name"], "v0.2.21")

    def test_none_when_no_stable_release(self):
        self.assertIsNone(plan.select_latest([release("v0.2.21", "2026-11-11T00:00:00Z", draft=True)]))


class DecideTest(unittest.TestCase):
    def test_already_contained_without_pr(self):
        decision = plan.decide(latest_commit=COMMIT_A, contained=True, open_pr=None, head=None, successor=False)
        self.assertEqual(decision["action"], "none")

    def test_already_contained_comments_open_pr(self):
        decision = plan.decide(
            latest_commit=COMMIT_A,
            contained=True,
            open_pr={"number": 4, "commit": COMMIT_A},
            head=COMMIT_A,
            successor=True,
        )
        self.assertEqual(decision["action"], "comment")
        self.assertEqual(decision["reason"], "already-contained")

    def test_opens_when_missing(self):
        decision = plan.decide(latest_commit=COMMIT_A, contained=False, open_pr=None, head=None, successor=False)
        self.assertEqual(decision, {"action": "open", "reason": "no-pr"})

    def test_same_release_without_cleanup_is_idle(self):
        decision = plan.decide(
            latest_commit=COMMIT_A,
            contained=False,
            open_pr={"number": 4, "commit": COMMIT_A},
            head=COMMIT_A,
            successor=True,
        )
        self.assertEqual(decision["action"], "none")

    def test_same_release_with_cleanup_does_not_push(self):
        decision = plan.decide(
            latest_commit=COMMIT_A,
            contained=False,
            open_pr={"number": 4, "commit": COMMIT_A},
            head=COMMIT_B,
            successor=True,
        )
        self.assertEqual(decision["action"], "comment")
        self.assertEqual(decision["reason"], "cleanup-in-progress")

    def test_newer_release_fast_forwards_without_cleanup(self):
        decision = plan.decide(
            latest_commit=COMMIT_B,
            contained=False,
            open_pr={"number": 4, "commit": COMMIT_A},
            head=COMMIT_A,
            successor=True,
        )
        self.assertEqual(decision, {"action": "fast-forward", "reason": "newer-release"})

    def test_newer_release_with_cleanup_only_comments(self):
        decision = plan.decide(
            latest_commit=COMMIT_B,
            contained=False,
            open_pr={"number": 4, "commit": COMMIT_A},
            head="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
            successor=True,
        )
        self.assertEqual(decision["reason"], "cleanup-blocks-overwrite")

    def test_non_successor_stops(self):
        decision = plan.decide(
            latest_commit=COMMIT_B,
            contained=False,
            open_pr={"number": 4, "commit": COMMIT_A},
            head=COMMIT_A,
            successor=False,
        )
        self.assertEqual(decision["reason"], "not-successor")

    def test_unrecognized_pr_is_not_overwritten(self):
        decision = plan.decide(
            latest_commit=COMMIT_A,
            contained=False,
            open_pr={"number": 4, "commit": None},
            head=COMMIT_B,
            successor=False,
        )
        self.assertEqual(decision["reason"], "unrecognized-pr")


class BodyTest(unittest.TestCase):
    def test_body_has_marker_notes_and_scope_without_closes(self):
        body = plan.render_body("v0.2.21", COMMIT_A, "新增模型", "go.mod | 2 +-")
        self.assertIn(f"<!-- upstream-sync: release=v0.2.21 commit={COMMIT_A} -->", body)
        self.assertIn("新增模型", body)
        self.assertIn("go.mod | 2 +-", body)
        self.assertNotIn("Closes", body)
        parsed = plan.parse_marker(body)
        self.assertEqual(parsed, {"release": "v0.2.21", "commit": COMMIT_A})

    def test_upstream_closing_keywords_do_not_stay_active(self):
        notes = (
            "Closes #1\n"
            "Fixes Wei-Shaw/sub2api#2\n"
            "resolved: https://github.com/Wei-Shaw/sub2api/issues/3\n"
            "close the connection"
        )
        body = plan.render_body("v0.2.21", COMMIT_A, notes, "go.mod | 2 +-")
        self.assertNotIn("Closes #1", body)
        self.assertIn("`Closes` #1", body)
        self.assertNotIn("Fixes Wei-Shaw/sub2api#2", body)
        self.assertNotIn("resolved: https://github.com/Wei-Shaw/sub2api/issues/3", body)
        self.assertIn("close the connection", body)

    def test_fast_forward_edit_includes_the_new_title(self):
        text = Path(__file__).with_name("plan.py").read_text()
        edit = re.search(r'\["gh", "pr", "edit".{0,240}', text, re.S)
        self.assertIsNotNone(edit)
        self.assertIn("--title", edit.group(0))
        self.assertIn("pr_title(tag)", edit.group(0))
        self.assertEqual(plan.pr_title("v0.2.21"), "sync: 上游 v0.2.21")

    def test_long_diff_stat_is_truncated(self):
        body = plan.render_body("v0.2.21", COMMIT_A, "notes", "x\n" * 20000)
        self.assertLess(len(body), 65536)
        self.assertIn("已截断", body)

    def test_comment_text_exists_for_every_comment_reason(self):
        for reason in (
            "already-contained",
            "cleanup-in-progress",
            "cleanup-blocks-overwrite",
            "not-successor",
            "unrecognized-pr",
        ):
            self.assertTrue(plan.comment_for(reason))


class WorkflowContractTest(unittest.TestCase):
    def test_workflow_matches_the_agreed_schedule_and_permissions(self):
        text = Path(__file__).resolve().parents[1].joinpath("workflows", "upstream-sync.yml").read_text()
        self.assertIn("cron: '17 2 * * *'", text)
        self.assertIn("workflow_dispatch:", text)
        self.assertIn("group: upstream-sync", text)
        self.assertIn("cancel-in-progress: false", text)
        self.assertIn("environment: upstream-sync", text)
        self.assertIn("contents: read", text)
        self.assertIn("actions/create-github-app-token@v2", text)
        self.assertIn("UPSTREAM_SYNC_APP_ID", text)
        self.assertIn("UPSTREAM_SYNC_APP_PRIVATE_KEY", text)
        self.assertIn("repositories: sub2api", text)
        self.assertIn("permission-contents: write", text)
        self.assertIn("permission-pull-requests: write", text)
        self.assertIn("permission-workflows: write", text)
        self.assertIn("persist-credentials: false", text)
        self.assertIn("steps.app.outputs.token", text)
        self.assertIn("ref: main", text)
        self.assertNotRegex(text, r"(?m)^[ ]+workflows:\s")
        self.assertNotIn("github.token", text)
        self.assertIn("GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}", text)
        self.assertNotIn("GITHUB_TOKEN: ${{ steps.app.outputs.token }}", text)
        self.assertNotIn("packages:", text)
        script = Path(__file__).with_name("plan.py").read_text()
        self.assertIn("refs/remotes/origin/main", script)
        self.assertNotIn('["git", "push", "origin"', script)
        self.assertNotIn('["git", "merge-base", "--is-ancestor", commit, "HEAD"]', script)
        self.assertNotIn("Closes #", text)
        docs = Path(__file__).resolve().parents[2].joinpath("docs", "ci-cd.md").read_text()
        self.assertIn("sub2api-upstream-sync", docs)
        self.assertIn("environment `upstream-sync`", docs)
        self.assertNotIn("省略该键后，这个作业可以推送", docs)
        self.assertIn("发布 App 尚未配置", docs)

    def test_docs_record_the_release_and_push_constraints(self):
        root = Path(__file__).resolve().parents[2]
        docs = root.joinpath("docs", "ci-cd.md").read_text()
        for phrase in (
            "v0.2.21-fork.1",
            "v0.2.21-fork.2",
            "docker buildx imagetools inspect",
            "config.Labels",
            "content at ",
            "Bypassed rule violations",
            "read:packages",
            "prerelease: auto",
            "同一个 git tag",
            "govulncheck",
            "allow_forking",
            "unittest -k",
            "SIMPLE_RELEASE",
            "不能当成安装丢失",
            "discover -s .github/upstream-sync",
            "test_plan.WorkflowContractTest",
            "不要把测试文件路径直接传给",
        ):
            self.assertIn(phrase, docs)
        contributing = root.joinpath("CONTRIBUTING.md").read_text()
        for phrase in (
            "credential.helper",
            "insteadOf",
            "match-head-commit",
            "git config、upstream 和 URL 保持原样",
        ):
            self.assertIn(phrase, contributing)

    def test_docs_keep_agent_routing_and_drop_sponsorship(self):
        root = Path(__file__).resolve().parents[2]
        license_text = root.joinpath("LICENSE").read_text()
        self.assertIn("GNU AFFERO GENERAL PUBLIC LICENSE", license_text)
        self.assertNotIn("LESSER GENERAL PUBLIC LICENSE", license_text)

        for name in ("README.md", "README_CN.md", "README_JA.md"):
            text = root.joinpath(name).read_text()
            for banned in (
                "Sponsors",
                "赞助商",
                "スポンサー",
                "Star History",
                "スター履歴",
                "trendshift.io",
                "assets/partners",
                "Wesley Liddick",
            ):
                self.assertNotIn(banned, text, name)
            self.assertNotRegex(text, r"(?m)^## (License|许可证|ライセンス)\s*$")

        readme = root.joinpath("README.md").read_text()
        for phrase in (
            "## ⚠️ Important Notice",
            "## Grok / xAI Support",
            "## Simple Mode",
            "## TypeSafe / Jev Support",
            "## Antigravity Support",
            "OpenAI Responses WebSocket ingress limits",
            "Force OpenAI upstream HTTP/SSE",
            "deploy/README.md",
        ):
            self.assertIn(phrase, readme)
        readme_cn = root.joinpath("README_CN.md").read_text()
        for phrase in (
            "## Codex Fast/Flex 策略说明",
            "网关防御纵深建议",
            "HTTP/2 (h2c)",
            "## OpenAI 图片模型",
            "deploy/DATAMANAGEMENTD_CN.md",
            "deploy/README.md",
        ):
            self.assertIn(phrase, readme_cn)

        agents = root.joinpath("AGENTS.md").read_text()
        for phrase in (
            "CONTRIBUTING.md",
            "docs/ci-cd.md",
            "docs/ci-runner.md",
            "AGPL-3.0",
            "deploy/README.md",
            "docs/agent-resource-safety.md",
            "DEV_GUIDE.md",
        ):
            self.assertIn(phrase, agents)

        docs = root.joinpath("docs", "ci-cd.md").read_text()
        self.assertIn("AGPL-3.0", docs)
        self.assertIn("assets/partners", docs)
        self.assertIn("docs/legal/", docs)
        self.assertIn("docs/ci-cd.md", root.joinpath("deploy", "DOCKER.md").read_text())
        self.assertIn("docs/ci-cd.md", root.joinpath(".github", "release-tools", "README.md").read_text())
        self.assertIn("CONTRIBUTING.md", root.joinpath("DEV_GUIDE.md").read_text())

        self.assertFalse(root.joinpath("CLA.md").exists())
        self.assertFalse(root.joinpath("assets", "partners").exists())
        for relative in (
            "frontend/src/components/common/README.md",
            "frontend/src/components/layout/EXAMPLES.md",
            "frontend/src/components/layout/INTEGRATION.md",
            "frontend/src/router/README.md",
            "frontend/src/stores/README.md",
            "frontend/src/views/auth/README.md",
            "frontend/src/views/auth/USAGE_EXAMPLES.md",
            "frontend/src/views/auth/VISUAL_GUIDE.md",
        ):
            self.assertFalse(root.joinpath(*relative.split("/")).exists(), relative)

        for name in ("docs/PAYMENT.md", "docs/PAYMENT_CN.md"):
            text = root.joinpath(name).read_text()
            self.assertNotIn("z-pay.cn", text, name)
            self.assertNotIn("kyrenpay.com", text, name)
            self.assertNotIn("uid=23808", text, name)
            self.assertNotIn("code=SUB2API", text, name)


class PushCommandTest(unittest.TestCase):
    def test_push_command_keeps_the_token_out_of_the_url(self):
        commit = "b" * 40
        command = plan.push_command("Marco9442/sub2api", commit)
        rendered = " ".join(command)
        self.assertIn("https://github.com/Marco9442/sub2api.git", command)
        self.assertIn(f"{commit}:refs/heads/sync/upstream", command)
        self.assertNotIn("--force", command)
        self.assertNotIn("ghs_", rendered)
        self.assertNotIn("gho_", rendered)
        self.assertNotIn("ghp_", rendered)
        self.assertIn("password=$GH_TOKEN", rendered)
        self.assertTrue(any(part == "credential.helper=" for part in command))
        self.assertEqual(
            plan.clear_checkout_credentials_command(),
            ["git", "config", "--local", "--unset-all", "http.https://github.com/.extraheader"],
        )

    def test_push_command_rejects_a_repository_outside_the_expected_shape(self):
        with self.assertRaises(SystemExit):
            plan.push_command("https://user:token@github.com/evil/repo", "c" * 40)

    def test_upstream_releases_are_read_with_the_job_token(self):
        script = Path(__file__).with_name("plan.py").read_text()
        window = re.search(r"def apply\(root\):.{0,700}", script, re.S)
        self.assertIsNotNone(window)
        self.assertIn("_run_with_token(upstream_releases_command(), root, releases_read_token())", window.group(0))
        self.assertNotIn("GH_TOKEN", window.group(0))
        self.assertEqual(
            plan.upstream_releases_command(),
            ["gh", "api", "--paginate", "repos/Wei-Shaw/sub2api/releases", "--jq", ".[]"],
        )

    def test_releases_read_token_rejects_the_app_token(self):
        with unittest.mock.patch.dict(os.environ, {"GITHUB_TOKEN": "job", "GH_TOKEN": "app"}, clear=False):
            self.assertEqual(plan.releases_read_token(), "job")
        with unittest.mock.patch.dict(os.environ, {"GITHUB_TOKEN": "same", "GH_TOKEN": "same"}, clear=False):
            with self.assertRaises(SystemExit):
                plan.releases_read_token()
        with unittest.mock.patch.dict(os.environ, {"GITHUB_TOKEN": "", "GH_TOKEN": "app"}, clear=False):
            with self.assertRaises(SystemExit):
                plan.releases_read_token()
