# مدل‌های GPT-6

نگاشت بر اساس payloadهای وب ارسال‌شده توسط مالک پروژه است؛ دسترسی upstream
به این مدل‌ها به حساب انتخاب‌شده وابسته است.

| نام API | model در upstream | thinking_effort |
| --- | --- | --- |
| `chatgpt/gpt-6` | `gpt-6` | ارسال نمی‌شود |
| `chatgpt/gpt-6-thinking-standard` | `gpt-6-thinking` | `standard` |
| `chatgpt/gpt-6-thinking-extended` | `gpt-6-thinking` | `extended` |

هر سه در `GET /v1/models` و مسیر جزئیات مدل وجود دارند. سطح تفکر از نام مدل
انتخاب می‌شود و نیازی به فیلد جداگانه نیست.

```bash
curl -sS http://127.0.0.1:3346/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"chatgpt/gpt-6-thinking-standard","messages":[{"role":"user","content":"سلام، کوتاه جواب بده."}]}'
```
