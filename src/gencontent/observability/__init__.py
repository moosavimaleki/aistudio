"""Decoratorهای observability مخصوص GenContent."""

from .pool import ObservedTabPool
from .service import ObservedGenerateService

__all__ = ["ObservedGenerateService", "ObservedTabPool"]
