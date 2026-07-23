"""Application service that leases, uses and returns virtual tabs."""

from __future__ import annotations

from dataclasses import dataclass, replace

from aistudio_client.config import Settings
from aistudio_client.errors import ClientError
from aistudio_client.models import GenerateInput, GenerateResult
from aistudio_client.tab import AIStudioTab, TabState, invalidates_tab
from aistudio_client.tab_snapshot import dump_tab, restore_tab

from .pool import RedisTabPool, TabLease
from .attachments import resolve_inline_data
from .profile_settings import ProfileSettings
from .profiles import BrowserProfiles


@dataclass(frozen=True)
class GenerateOutcome:
    tab_id: str
    browser_id: str
    auth_user: str
    generate_count: int
    result: GenerateResult


class GenerateContentService:
    def __init__(
        self,
        settings: Settings,
        pool: RedisTabPool,
        profiles: BrowserProfiles,
        profile_settings: ProfileSettings,
    ) -> None:
        self.settings = settings
        self.pool = pool
        self.profiles = profiles
        self.profile_settings = profile_settings

    def generate(self, input: GenerateInput) -> GenerateOutcome:
        # یک retry فقط برای tab خراب مجاز است. خطاهای model/quota باعث حذف tab
        # آماده نمی‌شوند و همان state برای درخواست بعدی به pool برمی‌گردد.
        for attempt in range(2):
            lease = self.pool.acquire()
            tab = None
            try:
                try:
                    tab = self._materialize(lease)
                except Exception as error:
                    self.pool.discard(lease)
                    if attempt == 0:
                        continue
                    raise

                try:
                    prepared = replace(input, contents=resolve_inline_data(input.contents, tab))
                    result = tab.generate(prepared)
                except Exception as error:
                    # خطای auth ممکن است پیش از tab.generate و هنگام upload رخ
                    # دهد؛ چنین tabای تعمیر یا refresh نمی‌شود و باید حذف شود.
                    if (
                        tab.state in {TabState.INVALID, TabState.FAILED}
                        or invalidates_tab(error)
                    ):
                        self.pool.discard(lease)
                    else:
                        self.pool.release(lease, dump_tab(tab))
                    if invalidates_tab(error) and attempt == 0:
                        continue
                    raise

                self.pool.release(lease, dump_tab(tab))
                return GenerateOutcome(
                    tab_id=tab.id,
                    browser_id=str(tab.settings.browser_id),
                    auth_user=str(tab.settings.auth_user),
                    generate_count=tab.generate_count,
                    result=result,
                )
            finally:
                if tab is not None:
                    tab.close()
        raise ClientError("Unable to replace invalid tab", phase="AUTH")

    def _materialize(self, lease: TabLease) -> AIStudioTab:
        if lease.is_new:
            profile = self.profiles.choose()
            settings = self.profile_settings.build(profile)
            return AIStudioTab(settings, tab_id=lease.tab_id).initialize()

        state = lease.state or {}
        browser_id = state.get("browserId")
        auth_user = state.get("authUser")
        if not browser_id or auth_user is None:
            raise ClientError("Persisted tab has no Chrome profile", phase="CONFIG")
        settings = replace(
            self.settings,
            browser_id=str(browser_id),
            auth_user=str(auth_user),
        )
        return restore_tab(settings, state)
