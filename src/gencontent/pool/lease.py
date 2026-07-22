"""Value objects and public errors of the tab pool."""

from dataclasses import dataclass


class PoolOverError(RuntimeError):
    """No tab became available before the configured deadline."""


@dataclass(frozen=True)
class TabLease:
    tab_id: str
    token: str
    state: dict | None

    @property
    def is_new(self) -> bool:
        return self.state is None
