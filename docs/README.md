# Gym App Docs

Lightweight reference for AI and developers. See individual files for detail.

## Layout

```
cmd/api/          Entry point. Server struct, NewServer, Run.
internal/
  ai/             OpenAI: client, throttler, parser, image. Test mode = no real calls.
  auth/           Google token verify, RequireAuth middleware, UserFromContext.
  chat/           ChatService: Process(text|audio) → parse → route by intent.
  chatmessages/   Conversation history for context-aware parsing.
  correction/     CorrectionService: find entry, update set.
  db/             DB connection + sqlutil (NullStr, EnsureV7, etc.).
  env/            GYM_* env var access. Avoids collisions with other projects.
  exercise/       Categories, variants, Resolve(category, variant).
  handler/        HTTP handlers. JSONError for responses.
  logentry/       Log entries + sets. Create, ListBySession, ListByUserAndVariant.
  notes/          User notes. Global or scoped to category/variant.
  pr/             Personal records. Create, CheckAndCreatePRs, image_url.
  query/          History by exercise. Uses logentry + session + exercise.
  session/        One per user per day. GetOrCreateForDate.
  storage/        R2: PutPRImageBytes, PresignPRImage.
  usage/          AI token usage persistence (ai_usage table).
  user/           GetByGoogleID, Create.
  testutil/       Shared test helpers (DBForTest).
migrations/       golang-migrate. Run make migrate-up.
```

## Flow

1. **POST /chat** → Transcribe (if audio) → Parse (LLM) → Log | Query | Correction | Remove | Restore | Note
2. **Log** → Resolve exercise → CreateLogEntry → CheckAndCreatePRs → DALL-E + R2 (if PR)
3. **Query** → QueryService.History → entries by category/variant
4. **Correction** → Resolve exercise → find latest entry → UpdateSet
5. **Remove/Restore** → Soft-delete or restore log entry
6. **Note** → Store reminder scoped to exercise or global

## Key Files

| Area | File | Purpose |
|------|------|---------|
| API | `docs/api.md` | Endpoints, auth, errors |
| Android API | `docs/android-api.md` | Client-facing API spec for Android app |
| DB | `docs/db.md` | Schema, migrations |
| Chat context | `docs/chat-context.md` | Conversation history, sliding window |
| AI | `docs/ai-throttling.md` | Rate limits, test safety |
| Maintenance | `docs/maintenance.md` | Tech debt, improvements to tackle |
| Env | `docs/notes.md` | Env vars, local setup |
| Decisions | `docs/notes.md` | Build order, conventions |

## Conventions

- Handler → Service → Repo. No business logic in handlers.
- All creation via POST /chat. No manual write endpoints.
- `GYM_OPENAI_TEST_MODE=true` for tests. Never hit real API in tests.
- Error responses: `{ error, code, error_token }`. Log error_token server-side.
