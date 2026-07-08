from __future__ import annotations

import asyncio
import shutil
import subprocess
import tempfile
import threading
import time
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional

from config import EMBA_LOG_BASE_DIR, EMBA_PATH
from database import (
    db_complete_task,
    db_count_running,
    db_delete_task,
    db_get_all_tasks,
    db_get_task,
    db_insert_task,
    db_update_console_log_tmp,
    db_update_firmware_tmp_dir,
    db_update_progress,
    db_update_status,
)
from utils.log_parser import LogReader

_runtime: dict[str, "RuntimeInfo"] = {}
_lock = threading.Lock()


class RuntimeInfo:
    __slots__ = ("process", "console_log_fh", "console_log_tmp", "created_at")

    def __init__(self) -> None:
        self.process: Optional[subprocess.Popen] = None
        self.console_log_fh = None
        self.console_log_tmp: Optional[str] = None
        self.created_at: datetime = datetime.now(timezone.utc)


def get_task(task_id: str) -> Optional[dict]:
    return db_get_task(task_id)


def get_all_tasks(page: int = 1, page_size: int = 20) -> dict:
    return db_get_all_tasks(page, page_size)


def count_running() -> int:
    return db_count_running()


def delete_task(task_id: str) -> bool:
    with _lock:
        ri = _runtime.pop(task_id, None)
        _message_buffer.pop(task_id, None)
    task = db_get_task(task_id)
    if ri is None and task is None:
        return False
    if ri is not None:
        if ri.process and ri.process.poll() is None:
            ri.process.terminate()
            try:
                ri.process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                ri.process.kill()
        if ri.console_log_tmp and Path(ri.console_log_tmp).exists():
            Path(ri.console_log_tmp).unlink(missing_ok=True)
    if task:
        tmp = task.get("console_log_tmp")
        if tmp and Path(tmp).exists():
            Path(tmp).unlink(missing_ok=True)
        fw_tmp = task.get("firmware_tmp_dir")
        if fw_tmp and Path(fw_tmp).exists():
            shutil.rmtree(fw_tmp, ignore_errors=True)
        if task.get("log_dir") and Path(task["log_dir"]).exists():
            shutil.rmtree(task["log_dir"], ignore_errors=True)
    db_delete_task(task_id)
    return True


def start_scan(
    firmware_path: str,
    firmware_tmp_dir: Optional[str] = None,
    modules: Optional[str] = None,
    profile: Optional[str] = None,
    arch: Optional[str] = None,
) -> dict:
    task_id = uuid.uuid4().hex[:12]
    log_dir = str(Path(EMBA_LOG_BASE_DIR) / task_id)

    db_insert_task(task_id, log_dir)
    if firmware_tmp_dir:
        db_update_firmware_tmp_dir(task_id, firmware_tmp_dir)

    cmd = ["sudo", str(Path(EMBA_PATH) / "emba"), "-f", firmware_path, "-l", log_dir]
    if profile:
        cmd.extend(["-p", profile])
    if modules:
        for m in modules.split(","):
            cmd.extend(["-m", m.strip()])
    if arch:
        cmd.extend(["-a", arch])

    console_log_tmp = tempfile.NamedTemporaryFile(
        prefix="emba_console_", suffix=".log", delete=False
    )
    console_log_tmp.write(f"cmd: {' '.join(cmd)}\n".encode())
    console_log_tmp.flush()

    ri = RuntimeInfo()
    ri.console_log_tmp = console_log_tmp.name
    ri.console_log_fh = console_log_tmp

    db_update_console_log_tmp(task_id, console_log_tmp.name)

    proc = subprocess.Popen(
        cmd,
        cwd=EMBA_PATH,
        stdout=console_log_tmp,
        stderr=subprocess.STDOUT,
        text=False,
    )
    ri.process = proc

    with _lock:
        _runtime[task_id] = ri

    db_update_status(task_id, "running")

    t = threading.Thread(target=_monitor, args=(task_id,), daemon=True)
    t.start()

    task = db_get_task(task_id)
    assert task is not None
    return task


def _monitor(task_id: str) -> None:
    with _lock:
        ri = _runtime.get(task_id)
    if ri is None:
        return

    task = db_get_task(task_id)
    if task is None:
        return

    log_path = str(Path(task["log_dir"]) / "emba.log")
    log_reader = LogReader(log_path)
    _notify_ws(task_id, "progress", "Scan started")

    while True:
        time.sleep(2)
        now = time.time()
        elapsed = round(now - ri.created_at.timestamp(), 1)

        if ri.process and ri.process.poll() is not None:
            exit_code = ri.process.returncode
            if ri.console_log_fh:
                ri.console_log_fh.close()
                ri.console_log_fh = None
            if ri.console_log_tmp:
                dest = str(Path(task["log_dir"]) / "emba.console.log")
                shutil.move(ri.console_log_tmp, dest)
                ri.console_log_tmp = None
            fw_tmp = task.get("firmware_tmp_dir")
            if fw_tmp and Path(fw_tmp).exists():
                shutil.rmtree(fw_tmp, ignore_errors=True)
            db_complete_task(task_id, exit_code, elapsed)
            db_update_console_log_tmp(task_id, "")
            _notify_ws(task_id, "completed", f"Scan finished (exit={exit_code})")
            return

        new_lines = log_reader.read_new_lines()
        for line in new_lines:
            _notify_ws(task_id, "log", line)

        db_update_progress(task_id, elapsed)


_ws_connections: dict[str, list] = {}
_event_loop: Optional[asyncio.AbstractEventLoop] = None
_message_buffer: dict[str, list] = {}
_BUFFER_MAX = 200


def set_event_loop(loop: asyncio.AbstractEventLoop) -> None:
    global _event_loop
    _event_loop = loop


def get_message_buffer(task_id: str) -> list:
    with _lock:
        return list(_message_buffer.get(task_id, []))


def _buffer_message(task_id: str, payload: str) -> None:
    with _lock:
        buf = _message_buffer.setdefault(task_id, [])
        buf.append(payload)
        if len(buf) > _BUFFER_MAX:
            _message_buffer[task_id] = buf[-_BUFFER_MAX:]


def register_ws(task_id: str, ws) -> None:
    with _lock:
        _ws_connections.setdefault(task_id, []).append(ws)


def unregister_ws(task_id: str, ws) -> None:
    with _lock:
        conns = _ws_connections.get(task_id, [])
        if ws in conns:
            conns.remove(ws)


def _notify_ws(task_id: str, msg_type: str, message: str) -> None:
    import json

    from datetime import datetime as dt

    payload = json.dumps(
        {
            "type": msg_type,
            "message": message,
            "timestamp": dt.now(timezone.utc).isoformat(),
        }
    )
    _buffer_message(task_id, payload)
    with _lock:
        conns = list(_ws_connections.get(task_id, []))
    for ws in conns:
        try:
            coro = ws.send_text(payload)
            if _event_loop and _event_loop.is_running():
                asyncio.run_coroutine_threadsafe(coro, _event_loop)
        except Exception:
            pass
