from __future__ import annotations

import asyncio
import os
import shutil
import tempfile
import zipfile
from pathlib import Path
from typing import Optional

from fastapi import APIRouter, File, Form, Header, HTTPException, UploadFile
from fastapi.responses import FileResponse, PlainTextResponse, StreamingResponse
from starlette.background import BackgroundTask

from config import EMBA_PATH, EMBA_VERSION_FILE
from models import ScanCreateResponse, VersionResponse
from tasks import (
    count_running,
    delete_task,
    get_all_history,
    get_all_tasks,
    get_history_after,
    get_task,
    register_sse,
    start_scan,
    unregister_sse,
)

router = APIRouter(prefix="/api")


@router.get("/version", response_model=VersionResponse)
def get_version() -> VersionResponse:
    try:
        version = Path(EMBA_VERSION_FILE).read_text().strip()
    except FileNotFoundError:
        version = "unknown"
    return VersionResponse(version=version, emba_path=EMBA_PATH)


@router.get("/scan/profiles")
def list_profiles() -> list[str]:
    profiles_dir = Path(str(Path(EMBA_PATH) / "scan-profiles"))
    if not profiles_dir.exists():
        return []
    return sorted(p.name for p in profiles_dir.glob("*.emba"))


@router.post("/scan", response_model=ScanCreateResponse, status_code=201)
async def create_scan(
    firmware: UploadFile = File(...),
    modules: Optional[str] = Form(None),
    profile: Optional[str] = Form(None),
    arch: Optional[str] = Form(None),
    name: Optional[str] = Form(None),
) -> ScanCreateResponse:
    from config import EMBA_MAX_CONCURRENT_SCANS

    if count_running() >= EMBA_MAX_CONCURRENT_SCANS:
        raise HTTPException(
            status_code=503,
            detail=f"Max concurrent scans ({EMBA_MAX_CONCURRENT_SCANS}) reached",
        )

    import tempfile

    tmp = tempfile.mkdtemp(prefix="emba-fw-")
    fw_path = str(Path(tmp) / firmware.filename)
    with open(fw_path, "wb") as f:
        content = await firmware.read()
        f.write(content)

    task = start_scan(
        firmware_path=fw_path,
        firmware_tmp_dir=tmp,
        modules=modules,
        profile=profile,
        arch=arch,
        name=name,
    )
    return ScanCreateResponse(
        task_id=task["task_id"],
        status=task["status"],
        message="Scan task created",
    )


@router.get("/scan/{task_id}")
def get_scan_status(task_id: str):
    task = get_task(task_id)
    if task is None:
        raise HTTPException(status_code=404, detail="Task not found")
    return {
        "task_id": task["task_id"],
        "name": task["name"],
        "status": task["status"],
        "elapsed_seconds": task["elapsed_seconds"],
        "created_at": task["created_at"],
        "completed_at": task["completed_at"],
        "exit_code": task["exit_code"],
    }


@router.get("/scan/{task_id}/logs")
def get_scan_logs(task_id: str) -> PlainTextResponse:
    task = get_task(task_id)
    if task is None:
        raise HTTPException(status_code=404, detail="Task not found")
    log_path = Path(task["log_dir"]) / "emba.log"
    if log_path.exists():
        return PlainTextResponse(content=log_path.read_text(errors="replace"))
    raise HTTPException(status_code=404, detail="Log file not found")


@router.get("/scan/{task_id}/report")
def get_scan_report(task_id: str) -> FileResponse:
    task = get_task(task_id)
    if task is None:
        raise HTTPException(status_code=404, detail="Task not found")
    log_dir = Path(task["log_dir"])
    if not log_dir.exists():
        raise HTTPException(status_code=404, detail="Log directory not found")

    tmp_fd, tmp_path = tempfile.mkstemp(prefix="emba-log-", suffix=".zip")
    os.close(tmp_fd)
    with zipfile.ZipFile(tmp_path, "w", zipfile.ZIP_DEFLATED) as zf:
        for file in log_dir.rglob("*"):
            if file.is_file():
                zf.write(file, file.relative_to(log_dir))
    return FileResponse(
        path=tmp_path,
        media_type="application/zip",
        filename=f"emba-log-{task_id}.zip",
        background=BackgroundTask(os.remove, tmp_path),
    )


@router.get("/scan/{task_id}/sbom")
def get_scan_sbom(task_id: str) -> FileResponse:
    task = get_task(task_id)
    if task is None:
        raise HTTPException(status_code=404, detail="Task not found")
    sbom_path = Path(task["log_dir"]) / "SBOM" / "EMBA_cyclonedx_sbom.json"
    if not sbom_path.exists():
        raise HTTPException(status_code=404, detail="SBOM not found")
    return FileResponse(
        path=str(sbom_path),
        media_type="application/json",
        filename=f"emba-sbom-{task_id}.json",
    )


@router.delete("/scan/{task_id}")
def remove_scan(task_id: str):
    if not delete_task(task_id):
        raise HTTPException(status_code=404, detail="Task not found")
    return {"message": "Task deleted"}


@router.get("/scan")
def list_scans(page: int = 1, page_size: int = 20):
    result = get_all_tasks(page, page_size)
    return {
        "total": result["total"],
        "page": result["page"],
        "page_size": result["page_size"],
        "items": [
            {
                "task_id": t["task_id"],
                "name": t["name"],
                "status": t["status"],
                "elapsed_seconds": t["elapsed_seconds"],
                "created_at": t["created_at"],
                "completed_at": t["completed_at"],
            }
            for t in result["items"]
        ],
    }


@router.get("/scan/{task_id}/events")
async def scan_events(
    task_id: str,
    last_event_id: Optional[str] = Header(None),
):
    task = get_task(task_id)
    if task is None:
        raise HTTPException(status_code=404, detail="Task not found")

    queue: asyncio.Queue[tuple[int, str]] = asyncio.Queue()
    client_id = register_sse(task_id, queue)

    async def event_stream():
        try:
            if last_event_id:
                after_id = int(last_event_id)
                for eid, payload in get_history_after(task_id, after_id):
                    yield f"id: {eid}\ndata: {payload}\n\n"
            else:
                for eid, payload in get_all_history(task_id):
                    yield f"id: {eid}\ndata: {payload}\n\n"

            while True:
                eid, msg = await queue.get()
                yield f"id: {eid}\ndata: {msg}\n\n"
        except asyncio.CancelledError:
            pass
        finally:
            unregister_sse(task_id, client_id)

    return StreamingResponse(
        event_stream(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "X-Accel-Buffering": "no",
        },
    )
