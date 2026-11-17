# Local latency measurement, 2026-11-15

The opt-in real-browser test passed on Chromium 146.0.7680.177 with no DISPLAY:
synthetic content-script execution, keyboard input and a button click. No private
images or account cookies were used by that test.

The local service was then recreated with unified headless. Supervisor confirmed
Xvfb, Fluxbox, x11vnc and noVNC were stopped. A real GPT request was measured with
`python3 scripts/measure_chatgpt_latency.py --requests 1 --timeout 180`.

Request `request-1794745453432694421`, model
`chatgpt/gpt-5.6-thinking-off`, returned HTTP 502 in 58.089 seconds:

| Cumulative stage | Milliseconds |
| --- | ---: |
| Conversation routing, including a successful initialization turn | 6975.024 |
| Browser artifacts across calls | 51947.475 |
| Browser extension result wait across calls | 5677.827 |
| Browser page cleanup fallback | 45001.066 |
| Upstream conversation headers | 3552.774 |
| Upstream response body | 2554.257 |
| Upstream parsing | 0.493 |
| Server total | 58059.065 |

Nested stages overlap. These numbers must not be summed. The successful initial
browser preparation took 862.645 ms. The later preparation captured artifacts,
then failed during cleanup because the page displayed “Just a moment”. No
challenge was clicked or solved. Headless was reverted to false in local `.env`
to restore VNC access; the implementation remains available as an option.

This run verifies timing propagation through both services, including failures.
It does **not** establish a successful end-to-end latency improvement or memory
savings. The API still buffers streamed completions, so user-visible first
content time is not the upstream model's first-token latency.

## Follow-up: reuse the mounted page (extension 0.3.11)

Removed post-probe New chat and the Go navigation fallback. The existing tab
remains mounted; every request still obtains fresh browser artifacts.

Three real sequential requests on profile `chatgpt` succeeded:

| Request | Total seconds | Browser artifacts seconds | Conversation routing seconds |
| --- | ---: | ---: | ---: |
| 1 | 19.713 | 2.678, across initialization and completion | 13.196 |
| 2 | 6.624 | 1.390 | 0.000113 |
| 3 | 7.394 | 1.123 | 0.000259 |

CDP observation across the requests recorded **zero main-frame navigations and
zero Document requests**. The page target and `performance.timeOrigin` stayed
identical. There was one ChatGPT page target after the run. A client-side route
change occurred inside that same mounted document; there was no page reload.

These are three local samples, not a latency guarantee. They confirm that the
45-second cleanup path is gone and the warm browser bridge takes approximately
1.1–1.4 seconds in this run. User-visible first content is still delayed by
buffering the upstream completion.
