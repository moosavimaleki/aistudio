"""یک process مستقل Chrome را برای یک profile مدیریت می‌کند."""

import asyncio
import json
import os
import shutil
from pathlib import Path
from typing import BinaryIO, Protocol
from shared import upstream_value


class ChromeSpec(Protocol):
    browser_id: str


class ChromeProcess:
    def __init__(self, spec: ChromeSpec, port: int, index: int):
        self.spec = spec
        self.port = port
        self.index = index
        self.process: asyncio.subprocess.Process | None = None
        self._log: BinaryIO | None = None

    @property
    def cdp_url(self) -> str:
        return f"http://127.0.0.1:{self.port}"

    async def start(self) -> None:
        profile = self._browser_path("profiles")
        extension = self._browser_path("extensions")
        self._reset_directory(profile)
        self._reset_directory(extension)
        shutil.copytree(self._extension_source(), extension, dirs_exist_ok=True)
        self._write_extension_config(extension)

        runtime = Path(os.getenv("SELENIUM_RUNTIME_DIR", "/app/selenium/runtime"))
        runtime.mkdir(parents=True, exist_ok=True)
        self._log = (runtime / f"chrome-{self.spec.browser_id}.log").open("ab")
        self.process = await asyncio.create_subprocess_exec(
            os.getenv("CHROME_EXECUTABLE", "/usr/bin/google-chrome"),
            *self._arguments(profile, extension),
            stdout=self._log,
            stderr=asyncio.subprocess.STDOUT,
        )
        await self._wait_until_ready()

    async def stop(self) -> None:
        if self.process and self.process.returncode is None:
            self.process.terminate()
            try:
                await asyncio.wait_for(self.process.wait(), timeout=5)
            except TimeoutError:
                self.process.kill()
                await self.process.wait()
        if self._log:
            self._log.close()
        self.process = None
        self._log = None

    def _arguments(self, profile: Path, extension: Path) -> list[str]:
        arguments = [
            "--no-sandbox",
            "--disable-gpu",
            "--no-first-run",
            "--no-default-browser-check",
            "--disable-search-engine-choice-screen",
            "--remote-debugging-address=127.0.0.1",
            f"--remote-debugging-port={self.port}",
            f"--user-data-dir={profile}",
            f"--disable-extensions-except={extension}",
            f"--load-extension={extension}",
            f"--window-position={20 + self.index * 30},{20 + self.index * 30}",
        ]
        proxy = os.getenv("LAB_PROXY_URL", "").strip()
        if proxy:
            arguments.extend([
                f"--proxy-server={proxy}",
                "--proxy-bypass-list=127.0.0.1;localhost;<-loopback>",
            ])
        return [*arguments, "about:blank"]

    async def _wait_until_ready(self) -> None:
        for _attempt in range(100):
            if self.process and self.process.returncode is not None:
                raise RuntimeError(
                    f"Chrome {self.spec.browser_id} exited with {self.process.returncode}"
                )
            try:
                _reader, writer = await asyncio.open_connection("127.0.0.1", self.port)
                writer.close()
                await writer.wait_closed()
                return
            except OSError:
                await asyncio.sleep(0.1)
        raise RuntimeError(f"Chrome {self.spec.browser_id} CDP endpoint did not become ready")

    def _browser_path(self, category: str) -> Path:
        root = Path(os.getenv("CHROME_RUNTIME_DIR", "/tmp/aistudio-browsers"))
        return root / category / self.spec.browser_id

    @staticmethod
    def _reset_directory(path: Path) -> None:
        shutil.rmtree(path, ignore_errors=True)
        path.mkdir(parents=True, exist_ok=True)

    @staticmethod
    def _extension_source() -> Path:
        return Path(os.getenv("EXTENSION_SOURCE_DIR", "/app/extension"))

    def _write_extension_config(self, extension: Path) -> None:
        config = {
            "browserId": self.spec.browser_id,
            "factoryOrigin": os.getenv("FACTORY_ORIGIN", "http://127.0.0.1:3345"),
            "pageMatch": f'{upstream_value("aistudio", "origin")}/*',
        }
        target = extension / "config" / "runtime-config.js"
        target.parent.mkdir(parents=True, exist_ok=True)
        script = f"globalThis.AISTUDIO_BRIDGE_CONFIG = {json.dumps(config)};\n"
        target.write_text(script, encoding="utf-8")
