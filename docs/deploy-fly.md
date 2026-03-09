# Deploy to Fly.io

## Prerequisites

- `flyctl` installed and logged in (`fly auth login`)
- Postgres app `gym-app-pg` (or create with `fly postgres create`)

## 1. Attach Postgres (if not already)

```bash
fly postgres attach gym-app-pg --app gym-app
```

This sets `DATABASE_URL` automatically. The app uses `GYM_DATABASE_URL` or falls back to `DATABASE_URL`.

## 2. Set secrets (API keys, etc.)

Secrets are encrypted and injected as env vars at runtime. **Never commit these.**

```bash
# Required
fly secrets set GYM_GOOGLE_CLIENT_ID="your-oauth-client-id.apps.googleusercontent.com" -a gym-app
fly secrets set GYM_OPENAI_API_KEY="sk-proj-..." -a gym-app

# Production: use real AI (not mocks)
fly secrets set GYM_OPENAI_TEST_MODE="false" -a gym-app

# Optional: R2 for PR images
fly secrets set GYM_R2_ACCOUNT_ID="23acac6fd2f9179de96adf9599129074" -a gym-app
fly secrets set GYM_R2_ACCESS_KEY_ID="..." -a gym-app
fly secrets set GYM_R2_SECRET_ACCESS_KEY="..." -a gym-app
fly secrets set GYM_R2_BUCKET="gym-app" -a gym-app
```

**Set multiple at once:**

```bash
fly secrets set \
  GYM_GOOGLE_CLIENT_ID="..." \
  GYM_OPENAI_API_KEY="sk-..." \
  GYM_OPENAI_TEST_MODE="false" \
  -a gym-app
```

## 3. Deploy

```bash
fly deploy -a gym-app
```

Migrations run automatically via `release_command` in `fly.toml` before each deploy.

## 4. Verify

```bash
fly open -a gym-app
fly logs -a gym-app
fly secrets list -a gym-app   # names only, not values
```

## Notes

- **GYM_DATABASE_URL:** If you attach Postgres, Fly sets `DATABASE_URL`. The app prefers `GYM_DATABASE_URL`; if unset, it uses `DATABASE_URL`.
- **GYM_OPENAI_TEST_MODE:** Must be `false` in production so the real LLM is used. Omit or set explicitly.
- **GYM_DEV_MODE:** Never set in production. Enables dev token; only for local/test.
- **fly.toml [env]:** Non-sensitive vars (e.g. `PRIMARY_REGION`, `PORT`) can go in `fly.toml` under `[env]`. Use `fly secrets set` for sensitive values.
