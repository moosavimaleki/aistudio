# Optional visual click check

Place a PNG named `click.png` directly in this directory, then rebuild/reload the
extension. While an AI Studio or ChatGPT tab is visible, the extension checks its
visible viewport every 30 seconds. A match above the similarity threshold (0.86,
configured in `content/image-click.js`) is clicked once; the image must disappear
before it can trigger another click. Without `click.png`, this feature does nothing.

Only the visible area of the active tab is checked. The click is a DOM click in the
top-level page; browser chrome and content inside cross-origin iframes cannot be
clicked this way. The click is synthetic (`isTrusted` is false), so controls that
require a physical user gesture will not respond.
