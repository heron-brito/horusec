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

"""Update Horusec scanner image tags in internal/enums/images/images.go.

This script fetches tags from Docker Hub and bumps each scanner image to the
latest stable semantic version (vMAJOR.MINOR.PATCH).
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Dict, Iterable, List, Optional, Tuple
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

DOCKER_HUB_NAMESPACE = "horuszup"
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
    return parser.parse_args()


def parse_semver(tag: str) -> Optional[Tuple[int, int, int]]:
    match = SEMVER_RE.match(tag)
    if not match:
        return None
    return (int(match.group(1)), int(match.group(2)), int(match.group(3)))


def fetch_tags(repository: str, timeout: int) -> List[str]:
    tags: List[str] = []
    next_url = (
        f"https://hub.docker.com/v2/namespaces/{DOCKER_HUB_NAMESPACE}/"
        f"repositories/{repository}/tags?page_size=100"
    )

    while next_url:
        request = Request(next_url, headers={"Accept": "application/json"})
        with urlopen(request, timeout=timeout) as response:
            payload = json.load(response)

        for result in payload.get("results", []):
            name = result.get("name")
            if isinstance(name, str):
                tags.append(name)

        next_url = payload.get("next")

    return tags


def fetch_latest_semver_tag(repository: str, timeout: int) -> str:
    tags = fetch_tags(repository, timeout)
    versions = [version for tag in tags if (version := parse_semver(tag)) is not None]
    if not versions:
        raise ValueError(f"No stable semantic tag found for {repository}")

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


def replace_image_tag(value: str, new_tag: str) -> str:
    if ":" not in value:
        raise ValueError(f"Invalid image reference format: {value}")
    image_name = value.rsplit(":", 1)[0]
    return f"{image_name}:{new_tag}"


def compute_updates(
    constants: Dict[str, Tuple[int, str]],
    timeout: int,
) -> List[ImageUpdate]:
    updates: List[ImageUpdate] = []

    for const_name, repository in TARGET_CONST_TO_REPOSITORY.items():
        if const_name not in constants:
            raise KeyError(f"Constant {const_name} not found in images.go")

        _, value = constants[const_name]
        current_tag = extract_image_tag(value)
        latest_tag = fetch_latest_semver_tag(repository, timeout)

        if parse_semver(latest_tag) is None:
            raise ValueError(f"Latest tag is not stable semver: {repository}:{latest_tag}")

        if current_tag != latest_tag:
            updates.append(
                ImageUpdate(
                    const_name=const_name,
                    repository=repository,
                    from_tag=current_tag,
                    to_tag=latest_tag,
                )
            )

    return sorted(updates, key=lambda item: item.const_name)


def apply_updates(lines: List[str], constants: Dict[str, Tuple[int, str]], updates: List[ImageUpdate]) -> List[str]:
    updated_lines = list(lines)
    for update in updates:
        line_index, current_value = constants[update.const_name]
        new_value = replace_image_tag(current_value, update.to_tag)
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
                f"- `{DOCKER_HUB_NAMESPACE}/{update.repository}`: `{update.from_tag}` -> `{update.to_tag}`"
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
        original_lines = args.images_file.read_text(encoding="utf-8").splitlines(keepends=True)
        constants = parse_image_constants(original_lines)
        updates = compute_updates(constants, args.timeout)
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
