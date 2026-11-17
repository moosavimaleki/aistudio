# France deployment — 2026-11-15

- Host: `152.228.145.18`, project `/opt/services/aistudio-api`.
- Public endpoint: `https://aistudio.s123.shekarchi-app.ir`.
- Image: `sha256:fb6fb2949dc28dd0ee81121d25f3aaac52471ebc73d7d048ced763391c1c7e81`.
- Extension: `0.3.11`; post-probe New chat and page reload removed.
- Container memory limit preserved at 2147483648 bytes. Headed/VNC mode retained.
- Boot account set to `chatgpt3`; all other profiles retained and addressable.
- Added guard against a known blocked account parking healthy browsers.
- CDP readiness/submit evaluation deadline raised from 2 to 10 seconds for
  the busy server. This is a deadline, not an added sleep.
- Sources, binaries, extension JavaScript/JSON and runtime scripts updated.
  Existing cookie directories, private extension images and credentials retained.
- Rollback image: `aistudio-api:before-reuse-20261115`.
- Original Compose override backup:
  `docker-compose.override.yml.before-reuse-20261115`.

Initial smoke tests failed during browser startup/queue waits. After selecting
the previously usable boot profile and applying the small browser stability
fixes, two consecutive authenticated requests through the public HTTPS endpoint
succeeded on `chatgpt3`:

| Request | Total client seconds | Browser artifacts seconds | Conversation routing seconds |
| --- | ---: | ---: | ---: |
| 1 | 114.255 | 103.391 across calls | 79.955 |
| 2 | 42.517 | 38.750 | 0.001 |

For request 2, browser submit took 31.138 seconds and cookie transport extraction
took 6.009 seconds. Upstream conversation headers took 1.538 seconds and the
response body took 1.663 seconds. These spans can overlap with parent spans.
This deployment is functional for the tested text path, but **does not meet the
latency target**. The browser bridge remains the dominant measured delay.

Observed host load was around 15 on three vCPUs; this is supporting context,
not proof of the sole cause. No OOM or unexpected container restart was observed.
AI Studio still reported its pre-existing lifecycle `Promise was collected`
error; this deployment did not establish Gemini readiness.
