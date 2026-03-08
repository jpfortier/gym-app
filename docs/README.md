# Gym App Docs

Lightweight reference for AI and developers. See individual files for detail.

## Layout

```
cmd/api/          Entry point. Wires handlers, auth, services.
internal/
  ai/             OpenAI: client, throttler, parser, image. Test mode = no real calls.
  auth/           Google token verify, RequireAuth middleware, UserFromContext.
  chat/           ChatService: Process(text|audio) → parse → route by intent.
  correction/     CorrectionService: find entry, update set.
  db/             DB connection + sqlutil (NullStr, EnsureV7, etc.).
  exercise/       Categories, variants, Resolve(category, variant).
  handler/        HTTP handlers. JSONError for responses.
  logentry/       Log entries + sets. Create, ListBySession, ListByUserAndVariant.
  pr/             Personal records. Create, CheckAndCreatePRs, image_url.
  query/          History by exercise. Uses logentry + session + exercise.
  session/        One per user per day. GetOrCreateForDate.
  storage/        R2: PutPRImageBytes, PresignPRImage.
  user/           GetByGoogleID, Create.
migrations/       golang-migrate. Run make migrate-up.
```

## Flow

1. **POST /chat** → Transcribe (if audio) → Parse (LLM) → Log | Query | Correction
2. **Log** → Resolve exercise → CreateLogEntry → CheckAndCreatePRs → DALL-E + R2 (if PR)
3. **Query** → QueryService.History → entries by category/variant
4. **Correction** → Resolve exercise → find latest entry → UpdateSet

## Key Files

| Area | File | Purpose |
|------|------|---------|
| API | `docs/api.md` | Endpoints, auth, errors |
| DB | `docs/db.md` | Schema, migrations |
| AI | `docs/ai-throttling.md` | Rate limits, test safety |
| Env | `docs/notes.md` | Env vars, local setup |
| Decisions | `docs/notes.md` | Build order, conventions |

## Conventions

- Handler → Service → Repo. No business logic in handlers.
- All creation via POST /chat. No manual write endpoints.
- `OPENAI_TEST_MODE=true` for tests. Never hit real API in tests.
- Error responses: `{ error, code, error_token }`. Log error_token server-side.
