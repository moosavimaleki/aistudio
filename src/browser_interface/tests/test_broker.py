import asyncio
from unittest import IsolatedAsyncioTestCase

from browser_interface.broker import TokenBroker


class BrokerTests(IsolatedAsyncioTestCase):
    async def test_extension_completes_pending_job(self):
        broker = TokenBroker(timeout=1)
        pending = asyncio.create_task(broker.request({"digest": "abc"}))
        await asyncio.sleep(0)

        job = broker.next()
        self.assertEqual(job["digest"], "abc")
        self.assertTrue(broker.complete(job["id"], {"token": "fresh"}))
        self.assertEqual(await pending, {"token": "fresh"})

    async def test_job_times_out_and_is_removed(self):
        broker = TokenBroker(timeout=0.01)
        with self.assertRaisesRegex(RuntimeError, "before timeout"):
            await broker.request({"digest": "abc"})
        self.assertEqual(broker.health()["pendingJobs"], 0)

    async def test_each_extension_receives_only_its_browser_jobs(self):
        broker = TokenBroker(timeout=1)
        first = asyncio.create_task(
            broker.request({"digest": "first"}, "default")
        )
        second = asyncio.create_task(
            broker.request({"digest": "second"}, "browser2")
        )
        await asyncio.sleep(0)

        second_job = broker.next("browser2")
        self.assertEqual(second_job["digest"], "second")
        self.assertTrue(
            broker.complete(second_job["id"], {"token": "two"}, "browser2")
        )
        first_job = broker.next("default")
        self.assertEqual(first_job["digest"], "first")
        self.assertTrue(
            broker.complete(first_job["id"], {"token": "one"}, "default")
        )

        self.assertEqual(await first, {"token": "one"})
        self.assertEqual(await second, {"token": "two"})
