#!/usr/bin/env python3
"""Decide whether a release may write VERSION back to the default branch."""

import os
import sys


def main(argv):
    if argv not in ([], ["decide"]):
        print("usage: version_sync.py decide", file=sys.stderr)
        return 1
    repository = os.environ.get("GITHUB_REPOSITORY", "")
    version = os.environ.get("RELEASE_VERSION", "")
    if not repository or not version:
        print("GITHUB_REPOSITORY and RELEASE_VERSION are required", file=sys.stderr)
        return 1
    if repository != "Wei-Shaw/sub2api" or "-fork." in version:
        print("skip VERSION sync for this repository or fork release", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
