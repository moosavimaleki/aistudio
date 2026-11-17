# Headless memory comparison — 2026-11-15

Disposable containers ran sequentially per host. Service containers were not
restarted or changed. Each trial used one fresh browser profile, a localhost
page with 1000 synthetic messages, and a synthetic extension. Extension and page
readiness were checked through CDP before sampling. No account cookies, private
images, or external network were used. Headed included Xvfb, Fluxbox, x11vnc and
websockify; no VNC client was connected. Headless omitted these processes.

Each trial was limited to 0.5 CPU, 768 MiB RAM and 256 MiB shared memory, with no
published ports or network access. After readiness and five seconds of settling,
ten samples at one-second intervals measured cgroup memory.current minus
inactive_file (working set), not the sum of process RSS.

| Host | Headed mean MiB | Headless mean MiB | Difference |
| --- | ---: | ---: | ---: |
| Local, first headed trial | 418.75 | 244.37 | 174.38 MiB / 41.6% |
| Local, repeated headed trial | 386.26 | 244.37 | 141.89 MiB / 36.7% |
| France server 152.228.145.18 | 249.20 | 165.07 | 84.13 MiB / 33.8% |

Local headed variation was substantial. These short runs measure a synthetic
browser/display workload, **not authenticated ChatGPT or total service savings**.
They do not establish that headless improves API latency. Different image/browser
builds and host pressure mean cross-host absolute memory is not a fair benchmark.
The server's live profiles were not ready for a successful-page A/B test at this
time (challenge/startup failures). Server load was about 27 on three vCPUs.

The exact trial script is retained at
`artifacts/browser-performance/synthetic_memory_trial.py`.

## Previous successful API timings

| Stage | Local warm requests, seconds | France warm request, seconds |
| --- | ---: | ---: |
| Browser artifacts | 1.390 / 1.123 | 38.750 |
| Conversation headers | 1.930 / 2.447 | 1.538 |
| Response body | 2.956 / 3.492 | 1.663 |
| Total client time | 6.624 / 7.394 | 42.517 |

The recorded bridge time was 28–35 times larger on the server, while total
response time was 5.8–6.4 times larger. Prompts, account, host load and samples
were different; these are diagnostic observations rather than controlled speed
ratios. Both paths buffered responses. Headers time is not model TTFT. Startup
samples (19.713 local / 114.255 server) were excluded from the warm comparison.

All five disposable trial containers exited and were automatically removed.
