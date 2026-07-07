from __future__ import annotations

import sqlite3
import threading
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional

from config import EMBA_LOG_BASE_DIR

_DB_PATH = str(Path(EMBA_LOG_BASE_DIR) / ".emba-api.db")
_lock = threading.Lock()


def _get_conn() -> sqlite3.Connection:
    conn = sqlite3.connect(_DB_PATH, check_same_thread=False)
    conn.row_factory = sqlite3.Row
    return conn


def init_db() -> None:
    Path(EMBA_LOG_BASE_DIR).mkdir(parents=True, exist_ok=True)
    conn = _get_conn()
    conn.execute("""
        CREATE TABLE IF NOT EXISTS tasks (
            task_id        TEXT PRIMARY KEY,
            status         TEXT NOT NULL DEFAULT 'pending',
            log_dir        TEXT NOT NULL,
            created_at     TEXT NOT NULL,
            completed_at   TEXT,
            current_module TEXT,
            elapsed_seconds REAL DEFAULT 0,
            exit_code      INTEGER,
            console_log_tmp TEXT
        )
    """)
    try:
        conn.execute("ALTER TABLE tasks ADD COLUMN console_log_tmp TEXT")
    except sqlite3.OperationalError:
        pass
    conn.commit()
    conn.close()


def db_insert_task(task_id: str, log_dir: str) -> None:
    with _lock:
        conn = _get_conn()
        conn.execute(
            "INSERT INTO tasks (task_id, log_dir, created_at) VALUES (?, ?, ?)",
            (task_id, log_dir, datetime.now(timezone.utc).isoformat()),
        )
        conn.commit()
        conn.close()


def db_update_console_log_tmp(task_id: str, console_log_tmp: str) -> None:
    with _lock:
        conn = _get_conn()
        conn.execute(
            "UPDATE tasks SET console_log_tmp = ? WHERE task_id = ?",
            (console_log_tmp, task_id),
        )
        conn.commit()
        conn.close()


def db_update_status(task_id: str, status: str) -> None:
    with _lock:
        conn = _get_conn()
        conn.execute("UPDATE tasks SET status = ? WHERE task_id = ?", (status, task_id))
        conn.commit()
        conn.close()


def db_update_progress(task_id: str, current_module: str, elapsed_seconds: float) -> None:
    with _lock:
        conn = _get_conn()
        conn.execute(
            "UPDATE tasks SET current_module = ?, elapsed_seconds = ? WHERE task_id = ?",
            (current_module, elapsed_seconds, task_id),
        )
        conn.commit()
        conn.close()


def db_complete_task(task_id: str, exit_code: int, elapsed_seconds: float) -> None:
    with _lock:
        conn = _get_conn()
        conn.execute(
            "UPDATE tasks SET status = ?, completed_at = ?, exit_code = ?, elapsed_seconds = ? WHERE task_id = ?",
            (
                "completed" if exit_code == 0 else "failed",
                datetime.now(timezone.utc).isoformat(),
                exit_code,
                elapsed_seconds,
                task_id,
            ),
        )
        conn.commit()
        conn.close()


def db_get_task(task_id: str) -> Optional[dict]:
    with _lock:
        conn = _get_conn()
        row = conn.execute("SELECT * FROM tasks WHERE task_id = ?", (task_id,)).fetchone()
        conn.close()
        if row is None:
            return None
        return dict(row)


def db_get_all_tasks(page: int = 1, page_size: int = 20) -> dict:
    with _lock:
        conn = _get_conn()
        offset = (page - 1) * page_size
        total = conn.execute("SELECT COUNT(*) AS cnt FROM tasks").fetchone()["cnt"]
        rows = conn.execute(
            "SELECT * FROM tasks ORDER BY created_at DESC LIMIT ? OFFSET ?",
            (page_size, offset),
        ).fetchall()
        conn.close()
        return {"total": total, "page": page, "page_size": page_size, "items": [dict(r) for r in rows]}


def db_count_running() -> int:
    with _lock:
        conn = _get_conn()
        row = conn.execute("SELECT COUNT(*) AS cnt FROM tasks WHERE status = 'running'").fetchone()
        conn.close()
        return row["cnt"]


def db_delete_task(task_id: str) -> bool:
    with _lock:
        conn = _get_conn()
        cursor = conn.execute("DELETE FROM tasks WHERE task_id = ?", (task_id,))
        conn.commit()
        deleted = cursor.rowcount > 0
        conn.close()
        return deleted
