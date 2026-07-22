"""FastAPI application assembly for the container browser interface."""

import os
import asyncio
from contextlib import asynccontextmanager
from contextlib import suppress

from fastapi import FastAPI

from .api import jobs_router, public_router
from .broker import TokenBroker
from .browser.fleet import BrowserFleet
from .config import load_browser_config
from .events import emit
from .services import TokenService


@asynccontextmanager
async def lifespan(app: FastAPI):
    broker = TokenBroker()
    browsers = BrowserFleet(broker, load_browser_config())
    await browsers.start()
    app.state.broker = broker
    app.state.browsers = browsers
    app.state.tokens = TokenService(broker, browsers)
    warm_task = asyncio.create_task(browsers.warm())
    emit(
        "server-ready",
        port=int(os.getenv("PORT", "3345")),
        backend="container-extension",
        browserCount=len(browsers.status()),
    )
    yield
    warm_task.cancel()
    with suppress(asyncio.CancelledError):
        await warm_task
    await browsers.close()


app = FastAPI(title="AI Studio Browser Interface", version="1.0.0", lifespan=lifespan)
app.include_router(public_router)
app.include_router(jobs_router)
