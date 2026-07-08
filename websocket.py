from fastapi import APIRouter, WebSocket, WebSocketDisconnect

from tasks import get_message_buffer, get_task, register_ws, unregister_ws

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
        buffered = get_message_buffer(task_id)
        for msg in buffered:
            await websocket.send_text(msg)
        while True:
            data = await websocket.receive_text()
    except WebSocketDisconnect:
        pass
    finally:
        unregister_ws(task_id, websocket)
