"""FastAPI application assembly for the container browser interface."""

import os
import asyncio
from contextlib import asynccontextmanager
from contextlib import suppress

from fastapi import FastAPI

from lab_metrics import MetricsMiddleware
from lab_metrics.config import create_metric_store

from .api import jobs_router, public_router
from .broker import TokenBroker
from .browser.fleet import BrowserFleet
from .config import load_browser_config
from .events import emit, set_sink
from .observability import BrowserEventMetrics
from .services import TokenService


@asynccontextmanager
async def lifespan(app: FastAPI):
    metrics = create_metric_store()
    set_sink(BrowserEventMetrics(metrics))
    broker = TokenBroker()
    browsers = BrowserFleet(broker, load_browser_config())
    await browsers.start()
    app.state.broker = broker
    app.state.browsers = browsers
    app.state.tokens = TokenService(broker, browsers)
    app.state.metrics = metrics
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
    set_sink(None)
    metrics.redis.close()


app = FastAPI(title="AI Studio Browser Interface", version="1.0.0", lifespan=lifespan)
app.add_middleware(
    MetricsMiddleware,
    excluded_prefixes=("/health", "/internal/jobs"),
)
app.include_router(public_router)
app.include_router(jobs_router)
