from __future__ import annotations

import shutil
import zipfile
from pathlib import Path
from typing import Optional

from fastapi import APIRouter, File, Form, HTTPException, UploadFile
from fastapi.responses import FileResponse, PlainTextResponse

from config import EMBA_PATH, EMBA_SCAN_PROFILES_DIR, EMBA_VERSION_FILE
from models import ScanCreateResponse, VersionResponse
from tasks import count_running, delete_task, get_all_tasks, get_task, start_scan

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
    profiles_dir = Path(EMBA_SCAN_PROFILES_DIR)
    if not profiles_dir.exists():
        return []
    return sorted(p.name for p in profiles_dir.glob("*.emba"))


@router.post("/scan", response_model=ScanCreateResponse, status_code=201)
async def create_scan(
    firmware: UploadFile = File(...),
    modules: Optional[str] = Form(None),
    profile: Optional[str] = Form(None),
    arch: Optional[str] = Form(None),
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
        modules=modules,
        profile=profile,
        arch=arch,
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
        "status": task["status"],
        "progress": {
            "current_module": task["current_module"],
            "elapsed_seconds": task["elapsed_seconds"],
        },
        "created_at": task["created_at"],
        "completed_at": task["completed_at"],
        "exit_code": task["exit_code"],
    }


@router.get("/scan/{task_id}/logs")
def get_scan_logs(task_id: str) -> PlainTextResponse:
    task = get_task(task_id)
    if task is None:
        raise HTTPException(status_code=404, detail="Task not found")
    final_path = Path(task["log_dir"]) / "emba.console.log"
    if final_path.exists():
        return PlainTextResponse(content=final_path.read_text(errors="replace"))
    if task.get("console_log_tmp") and Path(task["console_log_tmp"]).exists():
        return PlainTextResponse(content=Path(task["console_log_tmp"]).read_text(errors="replace"))
    raise HTTPException(status_code=404, detail="Log file not found")


@router.get("/scan/{task_id}/report")
def get_scan_report(task_id: str) -> FileResponse:
    task = get_task(task_id)
    if task is None:
        raise HTTPException(status_code=404, detail="Task not found")
    report_dir = Path(task["log_dir"]) / "html-report"
    if not report_dir.exists():
        raise HTTPException(status_code=404, detail="HTML report not found")

    zip_path = Path(task["log_dir"]) / "html-report.zip"
    if not zip_path.exists():
        with zipfile.ZipFile(zip_path, "w", zipfile.ZIP_DEFLATED) as zf:
            for file in report_dir.rglob("*"):
                if file.is_file():
                    zf.write(file, file.relative_to(report_dir))
    return FileResponse(
        path=str(zip_path),
        media_type="application/zip",
        filename=f"emba-report-{task_id}.zip",
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
                "status": t["status"],
                "created_at": t["created_at"],
                "completed_at": t["completed_at"],
                "progress": {
                    "current_module": t["current_module"],
                    "elapsed_seconds": t["elapsed_seconds"],
                },
            }
            for t in result["items"]
        ],
    }
