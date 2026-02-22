#!/usr/bin/env python3
# Copyright 2026 ZUP IT SERVICOS EM TECNOLOGIA E INOVACAO SA
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

"""Update Horusec scanner images in internal/enums/images/images.go.

This script fetches versions from GHCR and keeps each scanner image pinned to
the latest stable semantic tag (vMAJOR.MINOR.PATCH) under
ghcr.io/<owner>/horusec-*.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Dict, Iterable, List, Optional, Tuple
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

GHCR_HOST = "ghcr.io"
DEFAULT_GHCR_OWNER = "heron-brito"
DEFAULT_IMAGES_FILE = Path("internal/enums/images/images.go")
DEFAULT_REPORT_FILE = Path(".scanner-governance-report.md")

SEMVER_RE = re.compile(r"^v(\d+)\.(\d+)\.(\d+)$")
CONST_LINE_RE = re.compile(r'^(?P<indent>\s*)(?P<const>[A-Za-z_][A-Za-z0-9_]*)\s*=\s*"(?P<value>[^"]+)"')

# Go const names from internal/enums/images/images.go
TARGET_CONST_TO_REPOSITORY = {
    "C": "horusec-c",
    "Csharp": "horusec-csharp",
    "Elixir": "horusec-elixir",
    "Generic": "horusec-generic",
    "Go": "horusec-go",
    "HCL": "horusec-hcl",
    "Javascript": "horusec-js",
    "Leaks": "horusec-leaks",
    "PHP": "horusec-php",
    "Python": "horusec-python",
    "Ruby": "horusec-ruby",
    "Shell": "horusec-shell",
}


@dataclass(frozen=True)
class ImageUpdate:
    const_name: str
    repository: str
    from_image: str
    to_image: str
    from_tag: str
    to_tag: str


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Update scanner images in images.go")
    parser.add_argument(
        "--images-file",
        type=Path,
        default=DEFAULT_IMAGES_FILE,
        help=f"Path to images.go file (default: {DEFAULT_IMAGES_FILE})",
    )
    parser.add_argument(
        "--report-file",
        type=Path,
        default=DEFAULT_REPORT_FILE,
        help=f"Path to markdown report used in PR body (default: {DEFAULT_REPORT_FILE})",
    )
    parser.add_argument(
        "--timeout",
        type=int,
        default=15,
        help="HTTP timeout in seconds (default: 15)",
    )
    parser.add_argument(
        "--ghcr-owner",
        default=DEFAULT_GHCR_OWNER,
        help=f"GHCR owner/namespace (default: {DEFAULT_GHCR_OWNER})",
    )
    return parser.parse_args()


def parse_semver(tag: str) -> Optional[Tuple[int, int, int]]:
    match = SEMVER_RE.match(tag)
    if not match:
        return None
    return (int(match.group(1)), int(match.group(2)), int(match.group(3)))


def github_api_request(url: str, timeout: int, github_token: Optional[str]) -> object:
    headers = {
        "Accept": "application/vnd.github+json",
        "User-Agent": "horusec-scanner-governance",
    }
    if github_token:
        headers["Authorization"] = f"Bearer {github_token}"
    request = Request(url, headers=headers)
    with urlopen(request, timeout=timeout) as response:
        return json.load(response)


def fetch_tags_from_versions_endpoint(
    base_url: str, timeout: int, github_token: Optional[str]
) -> List[str]:
    tags: set[str] = set()
    page = 1

    while True:
        payload = github_api_request(
            f"{base_url}?per_page=100&page={page}", timeout, github_token
        )
        if not isinstance(payload, list) or not payload:
            break

        for version in payload:
            if not isinstance(version, dict):
                continue
            metadata = version.get("metadata")
            if not isinstance(metadata, dict):
                continue
            container = metadata.get("container")
            if not isinstance(container, dict):
                continue
            version_tags = container.get("tags")
            if not isinstance(version_tags, list):
                continue
            for tag in version_tags:
                if isinstance(tag, str):
                    tags.add(tag)

        if len(payload) < 100:
            break

        page += 1

    return sorted(tags)


def fetch_tags(repository: str, owner: str, timeout: int, github_token: Optional[str]) -> List[str]:
    api_paths = [
        f"https://api.github.com/users/{owner}/packages/container/{repository}/versions",
        f"https://api.github.com/orgs/{owner}/packages/container/{repository}/versions",
    ]

    last_error: Optional[Exception] = None
    for api_path in api_paths:
        try:
            tags = fetch_tags_from_versions_endpoint(api_path, timeout, github_token)
            if tags:
                return tags
        except HTTPError as err:
            if err.code != 404:
                raise
            last_error = err

    if last_error:
        raise last_error

    return []


def fetch_latest_semver_tag(repository: str, owner: str, timeout: int, github_token: Optional[str]) -> str:
    tags = fetch_tags(repository, owner, timeout, github_token)
    versions = [version for tag in tags if (version := parse_semver(tag)) is not None]
    if not versions:
        raise ValueError(f"No stable semantic tag found for ghcr.io/{owner}/{repository}")

    latest = max(versions)
    return f"v{latest[0]}.{latest[1]}.{latest[2]}"


def parse_image_constants(lines: Iterable[str]) -> Dict[str, Tuple[int, str]]:
    constants: Dict[str, Tuple[int, str]] = {}
    for index, line in enumerate(lines):
        match = CONST_LINE_RE.match(line)
        if not match:
            continue
        const_name = match.group("const")
        value = match.group("value")
        constants[const_name] = (index, value)
    return constants


def extract_image_tag(value: str) -> str:
    if ":" not in value:
        raise ValueError(f"Invalid image reference format: {value}")
    return value.rsplit(":", 1)[1]


def extract_image_name(value: str) -> str:
    if ":" not in value:
        raise ValueError(f"Invalid image reference format: {value}")
    return value.rsplit(":", 1)[0]


def replace_image_reference(value: str, new_image: str, new_tag: str) -> str:
    if ":" not in value:
        raise ValueError(f"Invalid image reference format: {value}")
    return f"{new_image}:{new_tag}"


def compute_updates(
    constants: Dict[str, Tuple[int, str]],
    timeout: int,
    ghcr_owner: str,
    github_token: Optional[str],
) -> List[ImageUpdate]:
    updates: List[ImageUpdate] = []

    for const_name, repository in TARGET_CONST_TO_REPOSITORY.items():
        if const_name not in constants:
            raise KeyError(f"Constant {const_name} not found in images.go")

        _, value = constants[const_name]
        current_image = extract_image_name(value)
        current_tag = extract_image_tag(value)
        latest_tag = fetch_latest_semver_tag(repository, ghcr_owner, timeout, github_token)
        expected_image = f"{ghcr_owner}/{repository}"

        if parse_semver(latest_tag) is None:
            raise ValueError(f"Latest tag is not stable semver: {repository}:{latest_tag}")

        if current_image != expected_image or current_tag != latest_tag:
            updates.append(
                ImageUpdate(
                    const_name=const_name,
                    repository=repository,
                    from_image=current_image,
                    to_image=expected_image,
                    from_tag=current_tag,
                    to_tag=latest_tag,
                )
            )

    return sorted(updates, key=lambda item: item.const_name)


def apply_updates(lines: List[str], constants: Dict[str, Tuple[int, str]], updates: List[ImageUpdate]) -> List[str]:
    updated_lines = list(lines)
    for update in updates:
        line_index, current_value = constants[update.const_name]
        new_value = replace_image_reference(current_value, update.to_image, update.to_tag)
        updated_lines[line_index] = updated_lines[line_index].replace(current_value, new_value, 1)
    return updated_lines


def write_report(report_file: Path, updates: List[ImageUpdate]) -> None:
    timestamp = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%SZ")

    lines = [
        "## Scanner Governance",
        "",
        f"Generated at: `{timestamp}`",
        "",
    ]

    if not updates:
        lines.extend(
            [
                "No scanner image updates were detected.",
                "",
            ]
        )
    else:
        lines.extend(
            [
                "Updated image tags:",
                "",
            ]
        )
        for update in updates:
            lines.append(
                (
                    f"- `{GHCR_HOST}/{update.from_image}:{update.from_tag}` -> "
                    f"`{GHCR_HOST}/{update.to_image}:{update.to_tag}`"
                )
            )
        lines.extend(
            [
                "",
                "Regression validation is executed by CI before opening this PR.",
                "",
            ]
        )

    report_file.write_text("\n".join(lines), encoding="utf-8")


def main() -> int:
    args = parse_args()

    if not args.images_file.exists():
        print(f"images file not found: {args.images_file}", file=sys.stderr)
        return 1

    try:
        github_token = os.getenv("GITHUB_TOKEN")
        original_lines = args.images_file.read_text(encoding="utf-8").splitlines(keepends=True)
        constants = parse_image_constants(original_lines)
        updates = compute_updates(constants, args.timeout, args.ghcr_owner, github_token)
        updated_lines = apply_updates(original_lines, constants, updates)

        if updates:
            args.images_file.write_text("".join(updated_lines), encoding="utf-8")

        write_report(args.report_file, updates)

        if updates:
            print("Scanner images updated:")
            for update in updates:
                print(f"- {update.const_name}: {update.from_tag} -> {update.to_tag}")
        else:
            print("No scanner image updates found.")

        return 0
    except (HTTPError, URLError, ValueError, KeyError) as err:
        print(f"update failed: {err}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
