"""FastAPI application assembly."""

from contextlib import asynccontextmanager

from fastapi import FastAPI

from aistudio_client.config import Settings
from aistudio_client.errors import ClientError
from lab_metrics import MetricsMiddleware
from lab_metrics.config import create_metric_store

from .api.routes import client_error_handler, pool_over_handler, router, value_error_handler
from .config import create_pool
from .pool import PoolOverError
from .profile_settings import ProfileSettings
from .profiles import BrowserProfiles
from .service import GenerateContentService
from .observability import ObservedGenerateService, ObservedTabPool


@asynccontextmanager
async def lifespan(app: FastAPI):
    raw_pool = create_pool()
    metrics = create_metric_store(raw_pool.redis)
    pool = ObservedTabPool(raw_pool, metrics)
    settings = Settings.load()
    if not settings.token_factory_url:
        raise RuntimeError("TOKEN_FACTORY_URL is required")
    service = GenerateContentService(
        settings,
        pool,
        BrowserProfiles(settings.token_factory_url),
        ProfileSettings(settings),
    )
    app.state.metrics = metrics
    app.state.pool = pool
    app.state.service = ObservedGenerateService(service, metrics)
    yield
    raw_pool.redis.close()


app = FastAPI(title="AI Studio GenContent", version="1.0.0", lifespan=lifespan)
app.add_middleware(
    MetricsMiddleware,
    excluded_paths=("/", "/health"),
    excluded_prefixes=("/dashboard",),
)
app.include_router(router)
app.add_exception_handler(PoolOverError, pool_over_handler)
app.add_exception_handler(ClientError, client_error_handler)
app.add_exception_handler(ValueError, value_error_handler)
