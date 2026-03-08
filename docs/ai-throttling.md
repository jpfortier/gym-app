# AI API Throttling & Cost Safety

## Policy

All OpenAI API calls (Whisper, GPT, DALL-E) are throttled and guarded to prevent accidental credit burn.

## Safeguards

### 1. Tests never call real APIs

- When `GYM_OPENAI_TEST_MODE=true`, all AI calls use mocks and return deterministic responses.
- Tests must set `GYM_OPENAI_TEST_MODE=true` or leave `GYM_OPENAI_API_KEY` unset so no real calls occur.
- `make test` sources `.env`; ensure `GYM_OPENAI_TEST_MODE=true` in `.env` when running tests locally.

### 2. Per-user rate limits

- **Per minute:** `GYM_OPENAI_RATE_PER_MINUTE` (default 10). Limits chat/Whisper requests per user per minute.
- **Per day:** `GYM_OPENAI_DAILY_LIMIT` (default 100). Hard cap on AI requests per user per day.

### 3. DALL-E throttling

- DALL-E is expensive. Capped at `GYM_OPENAI_DALLE_DAILY_LIMIT` (default 5) per user per day.
- PR image generation is background; if limit hit, PR is still saved, image_url stays empty.

### 4. Env vars

| Var | Default | Purpose |
|-----|---------|---------|
| `GYM_OPENAI_TEST_MODE` | `false` | When true, skip real API calls. Use mocks. **Set true for tests.** |
| `GYM_OPENAI_RATE_PER_MINUTE` | 10 | Max AI requests per user per minute |
| `GYM_OPENAI_DAILY_LIMIT` | 100 | Max AI requests per user per day |
| `GYM_OPENAI_DALLE_DAILY_LIMIT` | 5 | Max DALL-E generations per user per day |

## Test safety checklist

- [ ] `GYM_OPENAI_TEST_MODE=true` in `.env` when running `make test`
- [ ] Or `GYM_OPENAI_API_KEY` unset in test environment
- [ ] AI tests use mock clients; no `t.Run` that hits real OpenAI
