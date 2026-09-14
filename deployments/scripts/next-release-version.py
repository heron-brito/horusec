#!/usr/bin/env python3
# Copyright 2021 ZUP IT SERVICOS EM TECNOLOGIA E INOVACAO SA
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Resolve the next release, beta and rc versions for the release workflows.

Replaces `mage -v upVersions`, which derives the next version from whatever
release GitHub flags as "Latest". That flag is neither the highest release nor
aware of what has already been published to GHCR, so the next release can land
*below* versions that are already in use. On this repository the latest release
is v2.9.1 while v2.10.0 is a released tag and horusec-cli images are past
v2.10.16, so a patch release would have produced v2.9.2.

The base version here is the highest stable tag found anywhere it could already
be taken: GitHub releases, git tags, and the horusec-cli image tags on GHCR.

Emits the same output names the mage target did, so the workflows only had to
swap the step body:

    actualReleaseVersion, nextReleaseVersion, nextReleaseVersionStripped,
    nextReleaseBranchName, actualBetaVersion, nextBetaVersion,
    actualRCVersion, nextRCVersion
"""

import argparse
import json
import os
import re
import subprocess
import sys
from typing import List, Optional, Sequence, Tuple
from urllib.error import HTTPError
from urllib.request import Request, urlopen

DEFAULT_TIMEOUT = 30
DEFAULT_CLI_IMAGE = "horusec-cli"

# Same codes update-scanner-images.py retries on: the user and org package
# routes answer one or the other depending on the namespace and the token.
RETRYABLE_HTTP_CODES = (401, 403, 404)

STABLE_RE = re.compile(r"^v?(\d+)\.(\d+)\.(\d+)$")
PRERELEASE_RE = re.compile(r"^v?(\d+)\.(\d+)\.(\d+)-(beta|rc)\.(\d+)$")

PATCH, MINOR, MAJOR = "patch", "minor", "major"
ABBREVIATED = {"p": PATCH, "m": MINOR, "M": MAJOR}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "release_type",
        help="patch/p, minor/m or major/M",
    )
    parser.add_argument(
        "--github-repo",
        default=os.getenv("GITHUB_REPOSITORY", ""),
        help="owner/name of the repository holding the releases and tags",
    )
    parser.add_argument(
        "--ghcr-owner",
        default="",
        help="GHCR namespace to read the CLI image tags from (default: the repo owner)",
    )
    parser.add_argument(
        "--cli-image",
        default=DEFAULT_CLI_IMAGE,
        help=f"CLI image to consider when picking the base version (default: {DEFAULT_CLI_IMAGE})",
    )
    parser.add_argument(
        "--skip-ghcr",
        action="store_true",
        help="ignore published image tags and consider releases and git tags only",
    )
    parser.add_argument("--timeout", type=int, default=DEFAULT_TIMEOUT)
    return parser.parse_args()


def normalize_release_type(release_type: str) -> str:
    resolved = ABBREVIATED.get(release_type, release_type).lower()
    if resolved not in (PATCH, MINOR, MAJOR):
        raise SystemExit(
            f"{release_type} isn't a valid release type, inform (p or patch), (m or minor), (M or major)"
        )
    return resolved


def parse_stable(tag: str) -> Optional[Tuple[int, int, int]]:
    match = STABLE_RE.match(tag.strip())
    if not match:
        return None
    return (int(match.group(1)), int(match.group(2)), int(match.group(3)))


def parse_prerelease(tag: str, kind: str) -> Optional[Tuple[int, int, int, int]]:
    match = PRERELEASE_RE.match(tag.strip())
    if not match or match.group(4) != kind:
        return None
    return (
        int(match.group(1)),
        int(match.group(2)),
        int(match.group(3)),
        int(match.group(5)),
    )


def format_version(version: Sequence[int]) -> str:
    return "v{}.{}.{}".format(*version)


def resolve_github_token() -> Optional[str]:
    return os.getenv("GH_PACKAGES_TOKEN") or os.getenv("GITHUB_TOKEN")


def github_api_request(url: str, timeout: int, token: Optional[str]) -> object:
    headers = {
        "Accept": "application/vnd.github+json",
        "User-Agent": "horusec-next-release-version",
    }
    if token:
        headers["Authorization"] = f"Bearer {token}"
    with urlopen(Request(url, headers=headers), timeout=timeout) as response:
        return json.load(response)


def paginate(base_url: str, timeout: int, token: Optional[str]) -> List[dict]:
    items: List[dict] = []
    page = 1

    while True:
        payload = github_api_request(f"{base_url}?per_page=100&page={page}", timeout, token)
        if not isinstance(payload, list) or not payload:
            break

        items.extend(entry for entry in payload if isinstance(entry, dict))

        if len(payload) < 100:
            break

        page += 1

    return items


def fetch_release_and_tag_names(repo: str, timeout: int, token: Optional[str]) -> List[str]:
    """Every release name/tag and every git tag on the repository.

    Unlike the mage target this does not ask for /releases/latest: that endpoint
    returns whichever release carries the Latest flag, which is set by hand and
    on this repository points at v2.9.1 even though v2.10.0 was released after.
    """
    names: List[str] = []

    for release in paginate(f"https://api.github.com/repos/{repo}/releases", timeout, token):
        for key in ("tag_name", "name"):
            value = release.get(key)
            if isinstance(value, str):
                names.append(value)

    for tag in paginate(f"https://api.github.com/repos/{repo}/tags", timeout, token):
        value = tag.get("name")
        if isinstance(value, str):
            names.append(value)

    return names


def fetch_local_git_tags() -> List[str]:
    """Tags in the checkout, so a tag pushed but not yet released still counts."""
    try:
        output = subprocess.run(
            ["git", "tag", "--list"],
            capture_output=True,
            text=True,
            check=True,
        ).stdout
    except (OSError, subprocess.CalledProcessError):
        return []

    return [line.strip() for line in output.splitlines() if line.strip()]


def fetch_ghcr_tags(owner: str, image: str, timeout: int, token: Optional[str]) -> List[str]:
    api_paths = [
        f"https://api.github.com/users/{owner}/packages/container/{image}/versions",
        f"https://api.github.com/orgs/{owner}/packages/container/{image}/versions",
    ]

    last_error: Optional[HTTPError] = None
    for api_path in api_paths:
        try:
            tags: List[str] = []
            for version in paginate(api_path, timeout, token):
                metadata = version.get("metadata")
                if not isinstance(metadata, dict):
                    continue
                container = metadata.get("container")
                if not isinstance(container, dict):
                    continue
                for tag in container.get("tags") or []:
                    if isinstance(tag, str):
                        tags.append(tag)
            if tags:
                return tags
        except HTTPError as err:
            if err.code not in RETRYABLE_HTTP_CODES:
                raise
            last_error = err

    if last_error is not None:
        print(
            f"warning: could not read {image} tags from GHCR ({last_error.code}); "
            "using releases and git tags only",
            file=sys.stderr,
        )

    return []


def bump(base: Tuple[int, int, int], release_type: str) -> Tuple[int, int, int]:
    major, minor, patch = base
    if release_type == PATCH:
        return (major, minor, patch + 1)
    if release_type == MINOR:
        return (major, minor + 1, 0)
    return (major + 1, 0, 0)


def highest_prerelease(
    names: Sequence[str], kind: str
) -> Tuple[Optional[str], Optional[Tuple[int, int, int, int]]]:
    best_name: Optional[str] = None
    best: Optional[Tuple[int, int, int, int]] = None

    for name in names:
        parsed = parse_prerelease(name, kind)
        if parsed is None:
            continue
        if best is None or parsed > best:
            best, best_name = parsed, name

    return best_name, best


def next_prerelease(
    base_release: Tuple[int, int, int],
    next_release: Tuple[int, int, int],
    actual: Optional[Tuple[int, int, int, int]],
    kind: str,
) -> str:
    """Continue the current prerelease line, or open a new one.

    A prerelease whose release part is at or below the base release belongs to a
    version that already shipped, so the next one starts over at .1 against the
    version being prepared.
    """
    if actual is None or actual[:3] <= base_release:
        return f"{format_version(next_release)}-{kind}.1"

    return f"{format_version(actual[:3])}-{kind}.{actual[3] + 1}"


def write_outputs(outputs: dict) -> None:
    rendered = "\n".join(f"{key}={value}" for key, value in outputs.items())

    github_output = os.getenv("GITHUB_OUTPUT")
    if github_output:
        with open(github_output, "a", encoding="utf-8") as handle:
            handle.write(rendered + "\n")

    print(rendered)


def main() -> int:
    args = parse_args()
    release_type = normalize_release_type(args.release_type)

    if not args.github_repo or "/" not in args.github_repo:
        raise SystemExit("missing --github-repo (owner/name), or GITHUB_REPOSITORY in the environment")

    owner = args.ghcr_owner or args.github_repo.split("/", 1)[0]
    token = resolve_github_token()

    names = fetch_release_and_tag_names(args.github_repo, args.timeout, token)
    names.extend(fetch_local_git_tags())

    if not args.skip_ghcr:
        names.extend(fetch_ghcr_tags(owner, args.cli_image, args.timeout, token))

    stable = [parsed for name in names if (parsed := parse_stable(name)) is not None]
    if not stable:
        raise SystemExit("no stable vMAJOR.MINOR.PATCH version found in releases, tags or images")

    base = max(stable)
    next_release = bump(base, release_type)

    actual_beta_name, actual_beta = highest_prerelease(names, "beta")
    actual_rc_name, actual_rc = highest_prerelease(names, "rc")

    outputs = {
        "actualReleaseVersion": format_version(base),
        "nextReleaseVersion": format_version(next_release),
        "nextReleaseVersionStripped": "{}.{}.{}".format(*next_release),
        "nextReleaseBranchName": "release/v{}.{}".format(next_release[0], next_release[1]),
        "actualBetaVersion": actual_beta_name or "",
        "nextBetaVersion": next_prerelease(base, next_release, actual_beta, "beta"),
        "actualRCVersion": actual_rc_name or "",
        "nextRCVersion": next_prerelease(base, next_release, actual_rc, "rc"),
    }

    write_outputs(outputs)

    return 0


if __name__ == "__main__":
    sys.exit(main())
