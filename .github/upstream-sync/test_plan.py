import importlib.util
import json
import re
import unittest
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
        self.assertNotIn("permissions:", text)
        self.assertNotIn("workflows:", text)
        self.assertIn("不写 permissions", text)
        self.assertIn("ref: main", text)
        self.assertNotIn("packages:", text)
        script = Path(__file__).with_name("plan.py").read_text()
        self.assertIn("refs/remotes/origin/main", script)
        self.assertNotIn('["git", "merge-base", "--is-ancestor", commit, "HEAD"]', script)
        self.assertNotIn("Closes #", text)
