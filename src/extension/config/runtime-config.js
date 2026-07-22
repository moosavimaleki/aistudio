"use strict";

// این مقدار پیش‌فرض فقط برای اجرای دستی افزونه است؛ Selenium برای هر profile
// یک نسخهٔ جدا می‌سازد و browserId واقعی را در همان نسخه می‌نویسد.
globalThis.AISTUDIO_BRIDGE_CONFIG = {
  browserId: "default",
  factoryOrigin: "http://127.0.0.1:3345",
};
