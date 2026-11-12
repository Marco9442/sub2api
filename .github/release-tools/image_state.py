#!/usr/bin/env python3
"""Decide whether fork image tags are absent, reusable, or unsafe to publish."""

import argparse
import json
import subprocess
import sys

REVISION = "org.opencontainers.image.revision"
SOURCE = "org.opencontainers.image.source"


def main(argv):
    parser = argparse.ArgumentParser()
    parser.add_argument("command")
    parser.add_argument("--version", required=True)
    parser.add_argument("--sha", required=True)
    parser.add_argument("--repository", required=True)
    parser.add_argument("--registries", required=True)
    parser.add_argument("--arches", required=True)
    args = parser.parse_args(argv)
    if args.command != "decide":
        parser.error("unknown command")
    registries = [item for item in args.registries.split(",") if item]
    arches = [item for item in args.arches.split(",") if item]
    states = [
        inspect(ref, expected, args.sha, args.repository, check_identity)
        for ref, expected, check_identity in target_refs(registries, args.version, arches)
    ]
    if any(state == "error" for state in states):
        print("image inspect failed or was unreadable", file=sys.stderr)
        return 1
    absent = sum(state == "absent" for state in states)
    if absent == len(states):
        return 0
    if absent or any(state != "match" for state in states):
        print("existing images are partial or do not match this commit", file=sys.stderr)
        return 1
    return 10


def target_refs(registries, version, arches):
    refs = []
    for registry in registries:
        refs.append((f"{registry}:{version}", arches, False))
        for arch in arches:
            refs.append((f"{registry}:{version}-{arch}", [arch], True))
    return refs


def inspect(ref, expected_arches, sha, repository, check_identity):
    try:
        proc = subprocess.run(
            ["docker", "buildx", "imagetools", "inspect", "--format", "{{json .}}", ref],
            capture_output=True,
            text=True,
        )
    except FileNotFoundError:
        return "error"
    combined = f"{proc.stdout}\n{proc.stderr}".lower()
    if proc.returncode != 0:
        if "not found" in combined or "manifest unknown" in combined:
            return "absent"
        return "error"
    if not proc.stdout.strip():
        return "error"
    try:
        body = json.loads(proc.stdout)
    except json.JSONDecodeError:
        return "error"
    if not isinstance(body, dict) or not body:
        return "error"
    expected = {("linux", arch) for arch in expected_arches}
    if platforms_of(body) != expected:
        return "mismatch"
    if not check_identity:
        return "match"
    labels = labels_of(body)
    source = f"https://github.com/{repository}"
    if labels.get(REVISION) != sha or labels.get(SOURCE) != source:
        return "mismatch"
    return "match"


def platforms_of(body):
    manifests = (body.get("manifest") or {}).get("manifests") or []
    platforms = set()
    if manifests:
        for item in manifests:
            add_platform(platforms, (item.get("platform") or {}))
        return platforms
    add_platform(platforms, body.get("image") or {})
    return platforms


def add_platform(platforms, platform):
    os_name = platform.get("os") or ""
    architecture = platform.get("architecture") or ""
    if os_name in ("", "unknown") or architecture in ("", "unknown"):
        return
    platforms.add((os_name, architecture))


def labels_of(body):
    config = ((body.get("image") or {}).get("config") or {})
    for key in ("Labels", "labels"):
        value = config.get(key)
        if isinstance(value, dict):
            return value
    return {}


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
