from __future__ import annotations

from typing import Optional

from pydantic import BaseModel


class ScanCreateResponse(BaseModel):
    task_id: str
    status: str
    message: str


class VersionResponse(BaseModel):
    version: str
    emba_path: str
