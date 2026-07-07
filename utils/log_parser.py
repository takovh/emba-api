from __future__ import annotations

import re
from datetime import datetime
from typing import Optional

MODULE_PATTERN = re.compile(
    r"\[\*\]\s+\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}\s+-\s+Module\s+(\S+)"
)
PHASE_STARTED = re.compile(r"(\w[\w\s]*phase)\s+started", re.IGNORECASE)
PHASE_ENDED = re.compile(r"(\w[\w\s]*phase)\s+ended", re.IGNORECASE)
TEST_ENDED = re.compile(r"Test ended on", re.IGNORECASE)


def parse_log_line(line: str) -> Optional[str]:
    m = MODULE_PATTERN.search(line)
    if m:
        return m.group(1)
    return None


def is_scan_finished(line: str) -> bool:
    return bool(PHASE_ENDED.search(line) or TEST_ENDED.search(line))


def get_last_log_line(log_path: str) -> str:
    try:
        with open(log_path, "rb") as f:
            f.seek(0, 2)
            size = f.tell()
            if size == 0:
                return ""
            f.seek(max(0, size - 4096))
            data = f.read().decode("utf-8", errors="replace")
            lines = data.strip().splitlines()
            return lines[-1] if lines else ""
    except FileNotFoundError:
        return ""


def read_log_tail(log_path: str, n: int = 50) -> list[str]:
    try:
        with open(log_path, "r", errors="replace") as f:
            lines = f.readlines()
            return [l.rstrip("\n") for l in lines[-n:]]
    except FileNotFoundError:
        return []
