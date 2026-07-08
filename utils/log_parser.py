from __future__ import annotations


class LogReader:
    def __init__(self, log_path: str) -> None:
        self.log_path = log_path
        self._offset = 0

    def read_new_lines(self) -> list[str]:
        try:
            with open(self.log_path, "rb") as f:
                f.seek(self._offset)
                data = f.read().decode("utf-8", errors="replace")
                self._offset = f.tell()
                if not data:
                    return []
                lines = data.splitlines()
                return [l for l in lines if l]
        except FileNotFoundError:
            return []


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
