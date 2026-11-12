#!/usr/bin/env python3
"""Propose the latest official upstream release. Never merges or publishes."""

import json
import re
import subprocess
import sys
from pathlib import Path

UPSTREAM_REPO = "Wei-Shaw/sub2api"
UPSTREAM_URL = "https://github.com/Wei-Shaw/sub2api.git"
BRANCH = "sync/upstream"
TAG_RE = re.compile(r"^v\d+\.\d+\.\d+$")
MARKER_RE = re.compile(r"<!-- upstream-sync: release=(v\d+\.\d+\.\d+) commit=([0-9a-f]{40}) -->")
CLOSING_RE = re.compile(
    r"(?i)\b(close[sd]?|fix(?:e[sd])?|resolve[sd]?)"
    r"(\s*:?\s*)"
    r"(#\d+|https://github\.com/\S+/issues/\d+|[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+#\d+)"
)
BODY_LIMIT = 60000
DIFF_LIMIT = 12000
BASE_REF = "refs/remotes/origin/main"

COMMENTS = {
    "already-contained": "上游正式 Release 已经包含在 main，本次不推送分支。",
    "cleanup-in-progress": "同一 Release 已有整理提交，本次不推送分支。",
    "cleanup-blocks-overwrite": "同步分支已有整理提交。更新的正式 Release 只评论，不覆盖。",
    "not-successor": "新的正式 Release 不是当前提案的后继，停止覆盖。",
    "unrecognized-pr": "现有同步 PR 没有可识别的上游标记，本次不推送分支。",
}


def parse_releases(raw):
    """Parse one JSON object per line from paginated gh output.

    A JSON array, or an array of pages, is still accepted.
    """
    raw = (raw or "").strip()
    if not raw:
        return []
    if raw.startswith("["):
        parsed = json.loads(raw)
        if parsed and isinstance(parsed[0], list):
            return [item for page in parsed for item in page]
        return parsed
    return [json.loads(line) for line in raw.splitlines() if line.strip()]


def select_latest(releases):
    stable = []
    for item in releases:
        tag = item.get("tag_name") or ""
        if item.get("draft") or item.get("prerelease") or not TAG_RE.fullmatch(tag):
            continue
        stable.append(item)
    if not stable:
        return None
    return max(stable, key=lambda item: item.get("published_at") or "")


def parse_marker(body):
    match = MARKER_RE.search(body or "")
    if not match:
        return None
    return {"release": match.group(1), "commit": match.group(2)}


def decide(*, latest_commit, contained, open_pr, head, successor):
    if contained:
        if open_pr:
            return {"action": "comment", "reason": "already-contained"}
        return {"action": "none", "reason": "already-contained"}
    if open_pr is None:
        return {"action": "open", "reason": "no-pr"}
    if not open_pr.get("commit"):
        return {"action": "comment", "reason": "unrecognized-pr"}
    cleanup = head != open_pr["commit"]
    if open_pr["commit"] == latest_commit:
        if cleanup:
            return {"action": "comment", "reason": "cleanup-in-progress"}
        return {"action": "none", "reason": "already-proposed"}
    if cleanup:
        return {"action": "comment", "reason": "cleanup-blocks-overwrite"}
    if not successor:
        return {"action": "comment", "reason": "not-successor"}
    return {"action": "fast-forward", "reason": "newer-release"}


def comment_for(reason):
    return COMMENTS[reason]


def pr_title(tag):
    return f"sync: 上游 {tag}"


def neutralize_closing_keywords(text):
    """保留上游 Issue 编号，避免合并时关掉本仓 Issue。"""
    return CLOSING_RE.sub(lambda match: f"`{match.group(1)}`{match.group(2)}{match.group(3)}", text)


def render_body(tag, commit, notes, diff_stat):
    notes = neutralize_closing_keywords((notes or "").strip()) or "上游没有附带版本说明。"
    diff_stat = (diff_stat or "").strip() or "没有文件差异。"
    note = ""
    if len(diff_stat) > DIFF_LIMIT:
        diff_stat = diff_stat[:DIFF_LIMIT]
        note = "\n\n已截断。完整差异以 PR 的 Files changed 为准。"
    body = (
        f"<!-- upstream-sync: release={tag} commit={commit} -->\n\n"
        f"上游正式 Release `{tag}`。\n\n"
        "## 版本说明\n\n"
        f"{notes}\n\n"
        "## 改动范围\n\n"
        "相对 `main` 的 diff stat：\n\n"
        f"```\n{diff_stat}\n```{note}\n\n"
        "冲突留到合并时处理。本任务不合并、不解决冲突、不推 tag、不发镜像。\n"
    )
    if len(body) > BODY_LIMIT:
        body = body[:BODY_LIMIT] + "\n\n已截断。\n"
    return body


def _run(args, cwd):
    return subprocess.check_output(args, cwd=cwd, text=True).strip()


def _try_run(args, cwd):
    completed = subprocess.run(args, cwd=cwd, text=True, capture_output=True)
    if completed.returncode != 0:
        return None
    return completed.stdout.strip()


def apply(root):
    raw = _run(
        ["gh", "api", "--paginate", f"repos/{UPSTREAM_REPO}/releases", "--jq", ".[]"],
        root,
    )
    selected = select_latest(parse_releases(raw))
    if selected is None:
        print(json.dumps({"action": "none", "reason": "no-stable-release"}))
        return 0
    tag = selected["tag_name"]
    _run(["git", "fetch", "--no-tags", "origin", f"refs/heads/main:{BASE_REF}"], root)
    base = _run(["git", "rev-parse", BASE_REF], root)
    _run(["git", "fetch", "--no-tags", UPSTREAM_URL, f"refs/tags/{tag}"], root)
    commit = _run(["git", "rev-parse", "FETCH_HEAD^{commit}"], root)
    contained = subprocess.run(
        ["git", "merge-base", "--is-ancestor", commit, base], cwd=root
    ).returncode == 0
    prs = json.loads(
        _run(
            ["gh", "pr", "list", "--head", BRANCH, "--base", "main", "--state", "open",
             "--json", "number,body,headRefOid"],
            root,
        )
    )
    open_pr = None
    if prs:
        marker = parse_marker(prs[0].get("body"))
        open_pr = {"number": prs[0]["number"], "commit": None if marker is None else marker["commit"]}
    head = _try_run(["git", "rev-parse", "--verify", "--quiet", f"origin/{BRANCH}"], root)
    if head is None:
        fetched = _try_run(["git", "fetch", "--no-tags", "origin", f"{BRANCH}:refs/remotes/origin/{BRANCH}"], root)
        if fetched is not None:
            head = _try_run(["git", "rev-parse", "--verify", "--quiet", f"origin/{BRANCH}"], root)
    successor = False
    if open_pr and open_pr.get("commit"):
        if _try_run(["git", "cat-file", "-e", f"{open_pr['commit']}^{{commit}}"], root) is None:
            _run(["git", "fetch", "--no-tags", UPSTREAM_URL, open_pr["commit"]], root)
        successor = subprocess.run(
            ["git", "merge-base", "--is-ancestor", open_pr["commit"], commit], cwd=root
        ).returncode == 0
    decision = decide(
        latest_commit=commit, contained=contained, open_pr=open_pr, head=head, successor=successor
    )
    decision.update({"tag": tag, "commit": commit})
    if decision["action"] in ("open", "fast-forward"):
        diff_stat = _run(["git", "diff", "--stat", f"{base}...{commit}"], root)
        body = render_body(tag, commit, selected.get("body") or "", diff_stat)
        body_path = Path("/tmp") / f"upstream-sync-{tag}.md"
        body_path.write_text(body)
        _run(["git", "push", "origin", f"{commit}:refs/heads/{BRANCH}"], root)
        if decision["action"] == "open":
            url = _run(
                ["gh", "pr", "create", "--base", "main", "--head", BRANCH,
                 "--title", pr_title(tag), "--body-file", str(body_path)],
                root,
            )
            decision["url"] = url
        else:
            _run(
                ["gh", "pr", "edit", str(open_pr["number"]),
                 "--title", pr_title(tag), "--body-file", str(body_path)],
                root,
            )
    elif decision["action"] == "comment":
        _run(
            ["gh", "pr", "comment", str(open_pr["number"]), "--body", comment_for(decision["reason"])],
            root,
        )
    print(json.dumps(decision, ensure_ascii=False))
    return 0


def main(argv):
    if argv[1:] != ["apply"]:
        raise SystemExit("usage: plan.py apply")
    return apply(Path.cwd())


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
