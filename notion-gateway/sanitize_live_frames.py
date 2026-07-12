#!/usr/bin/env python3
"""Create a minimal, redacted live-stream fixture from a local JSON/NDJSON export.

The script never reads browser profiles, cookies, environment variables, or network
traffic. Give it a manually saved response-body export only. It removes credential-
shaped keys/values and replaces text, IDs, and emails with typed placeholders.
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any

FORBIDDEN_KEY_PARTS = {
    "authorization", "cookie", "token", "password", "secret", "api_key",
    "access_token", "refresh_token", "session", "email",
}
TEXT_KEYS = {"content", "text", "message", "prompt", "input", "output", "title", "description"}
ID_KEYS = {"id", "userid", "spaceid", "workspaceid", "threadid", "traceid", "accountid"}
STRUCTURAL_KEYS = {"type", "o", "op", "p", "path", "status", "role", "kind", "method"}
EMAIL = re.compile(r"\b[^\s@]+@[^\s@]+\.[^\s@]+\b")
UUID = re.compile(r"\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b")
BEARER = re.compile(r"(?i)\bbearer\s+[a-z0-9._~+/=-]{8,}")
TOKEN = re.compile(r"(?i)(token_v2|sk-[a-z0-9_-]{16,}|gh[pousr]_[a-z0-9_]{20,}|xox[baprs]-[a-z0-9-]{10,})")


def normalized_key(key: str) -> str:
    return key.lower().replace("-", "_").replace(" ", "_")


def forbidden_key(key: str) -> bool:
    normalized = normalized_key(key)
    return any(part in normalized for part in FORBIDDEN_KEY_PARTS)


def redact_string(value: str, key: str | None) -> str:
    if BEARER.search(value) or TOKEN.search(value):
        return "<redacted-credential>"
    if EMAIL.search(value):
        return "<redacted-email>"
    if UUID.search(value):
        return "<redacted-id>"
    key_normalized = normalized_key(key or "")
    if key_normalized in ID_KEYS or key_normalized.endswith("_id"):
        return "<redacted-id>"
    if key_normalized in TEXT_KEYS or key_normalized.endswith("_text"):
        return "<redacted-text>"
    if key_normalized in STRUCTURAL_KEYS:
        return value
    # Preserve only protocol-routing values. All other strings can carry
    # prompts, paths, tool arguments, model names, or workspace metadata.
    return "<redacted-text>"


def redact(value: Any, key: str | None = None) -> Any:
    if isinstance(value, dict):
        output: dict[str, Any] = {}
        for child_key, child_value in value.items():
            if forbidden_key(child_key):
                output[child_key] = "<redacted>"
            else:
                output[child_key] = redact(child_value, child_key)
        return output
    if isinstance(value, list):
        return [redact(item, key) for item in value]
    if isinstance(value, str):
        return redact_string(value, key)
    return value


def load_frames(path: Path) -> list[Any]:
    raw = path.read_text(encoding="utf-8")
    try:
        value = json.loads(raw)
    except json.JSONDecodeError:
        frames: list[Any] = []
        for number, line in enumerate(raw.splitlines(), start=1):
            line = line.strip()
            if not line:
                continue
            try:
                frames.append(json.loads(line))
            except json.JSONDecodeError as exc:
                raise SystemExit(f"line {number}: invalid JSON: {exc}") from exc
        if not frames:
            raise SystemExit("input contains no JSON frames")
        return frames
    return value if isinstance(value, list) else [value]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("input", type=Path, help="local JSON or NDJSON response-body export")
    parser.add_argument("output", type=Path, help="redacted fixture JSON to create")
    parser.add_argument("--max-frames", type=int, default=200, help="maximum frames retained (default: 200)")
    args = parser.parse_args()
    if args.max_frames < 1:
        raise SystemExit("--max-frames must be positive")
    if not args.input.is_file():
        raise SystemExit("input must be a regular local file")

    frames = load_frames(args.input)[: args.max_frames]
    fixture = {
        "schema_version": 1,
        "direction": "event",
        "transport": "browser-stream-redacted",
        "metadata": {
            "purpose": "live-stream-terminal-debug",
            "frame_count": str(len(frames)),
        },
        "payload": {"frames": redact(frames)},
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(fixture, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(f"Wrote redacted fixture with {len(frames)} frame(s): {args.output}")
    print("Manually review the output before sharing it.")
    return 0


if __name__ == "__main__":
    sys.exit(main())