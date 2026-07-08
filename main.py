import asyncio
import sys
from contextlib import asynccontextmanager
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))

from fastapi import FastAPI
from fastapi.staticfiles import StaticFiles

from config import EMBA_HOST, EMBA_PORT
from database import init_db
from routers.scan import router as scan_router
from tasks import set_event_loop
from websocket import router as ws_router


@asynccontextmanager
async def lifespan(app: FastAPI):
    init_db()
    set_event_loop(asyncio.get_running_loop())
    yield


app = FastAPI(title="EMBA Scanner API", version="1.0.0", lifespan=lifespan)

app.include_router(scan_router)
app.include_router(ws_router)

static_dir = Path(__file__).parent / "static"
app.mount("/", StaticFiles(directory=str(static_dir), html=True), name="static")

if __name__ == "__main__":
    import uvicorn

    uvicorn.run("main:app", host=EMBA_HOST, port=EMBA_PORT, reload=True)
