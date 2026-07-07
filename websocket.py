from fastapi import APIRouter, WebSocket, WebSocketDisconnect

from tasks import get_task, register_ws, unregister_ws

router = APIRouter()


@router.websocket("/ws/scan/{task_id}")
async def scan_progress_ws(websocket: WebSocket, task_id: str):
    task = get_task(task_id)
    if task is None:
        await websocket.close(code=4004, reason="Task not found")
        return

    await websocket.accept()
    register_ws(task_id, websocket)
    try:
        while True:
            data = await websocket.receive_text()
    except WebSocketDisconnect:
        pass
    finally:
        unregister_ws(task_id, websocket)
