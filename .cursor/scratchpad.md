# Gym App Scratchpad

## Current Status

- [x] Migrations created and run (000001_init, 000002_seed_categories)
- [x] Auth - Google Sign-In verification
- [x] Session repo + service
- [x] Log entry repo + service
- [x] Exercise repo + basic resolution
- [x] GET /sessions, GET /sessions/:id
- [x] Query service, GET /exercises, GET /prs
- [x] AI layer (POST /chat, Whisper, parse, etc.)
- [x] PR detection, DALL-E + R2

## Next

V2: FCM, notifications. Admin panel implemented.

## Architecture Backlog (from changes.md)

Planned improvements to align with LLM + Rule Engine architecture. See `changes.md` for full detail.

### Assumption Ledger + Ambiguity System (priority) — IMPLEMENTED

LLM outputs `assumptions` and `ambiguities`. Code uses them to decide confirm vs execute.

- **LLM outputs:** `ambiguities: []` (e.g. `["target_unclear", "multiple_targets"]`), `assumptions: []` (explicit vs inferred)
- **Code rule:** `len(ambiguities) > 0` → require confirmation; otherwise → auto-execute
- **Flow:** Extend ParsedIntent schema → update parse prompt → handler checks ambiguities before executing destructive/ambiguous actions

**Done:** `internal/workoutcontext/` builds active session, recent sessions, ref objects, aliases. Chat service passes workout context to parser. ParsedIntent has Assumptions, Ambiguities, UIText. Parse prompt updated. When correction/remove has ambiguities, returns `needs_confirmation: true` and does not execute.

### Other change suggestions

| Area | Summary |
|------|---------|
| **Pipeline** | Build workout context before LLM, add validation layer, execution policy (confirm vs execute), audit logging |
| **Workout context** | Send active session, recent sessions, reference objects (last set, last exercise), aliases, user_defaults to LLM — DONE |
| **Command DSL** | APPEND_SET, UPDATE_SET with targetRef, DELETE_SET; incremental logging "bench" → "140 for 8" → "145 for 6" |
| **Query DSL** | Metrics (max_weight, estimated_1rm, total_volume), scopes (most_recent, best, trend) |
| **Validation** | Deterministic checks: target refs resolve, command valid, units valid; output resolvedTargets + issues |
| **Confirmation policy** | Auto-execute: append/create, target unique. Confirm: delete, multiple targets, ambiguous |
| **Scope expansion** | Fallback: active session → today → yesterday → last 7 days (code-owned, not LLM) |
| **Error repair** | On TARGET_NOT_FOUND, try scope expansion; confirm if repair changes scope |
| **Domain model** | Session: notes, location, tags. Sets: unit, rir, tempo, partially specified (reps nullable) |
| **Success messages** | Generate from execution results: "Logged bench press — 140 lb for today." |

## Segments (V1)

**Backend foundation**
1. Session repo
2. Session service
3. Log entry repo
4. Log entry service
5. Exercise repo
6. Exercise resolution
7. GET /sessions
8. GET /sessions/:id
9. Query service
10. GET /exercises
11. PR repo
12. GET /prs

**AI layer**
13. POST /chat handler
14. Whisper
15. Parse (LLM)
16. Log intent
17. Query intent
18. Correction intent
19. PR detection
20. DALL-E + R2

**V2:** FCM, notifications. Admin panel done.

## Development workflow (Executor)

Segment by segment. For each segment: implement → test → verify. Then integration test with previous segment. Shared setup (login once), cleanup after each test.

## Recent decisions (documented in docs/)

- Migration 000006: users.role. Run `make migrate-up` when DB is available.

- Migration 000008 (embeddings): Requires pgvector. **Fly Postgres Flex:** Use `ziadm/postgres-flex-pgvector:17.2` image. **Local:** `brew install pgvector`. No fallback—if extension missing, embedding ops fail (by design).

- **User exercise aliases (migration 000009):** When we resolve "RDL" → Deadlift/RDL via embedding or create, we store (user_id, alias_key, variant_id). Next time the user says "RDL", alias lookup returns the variant directly—no LLM/embedding call.

- R2 for PR images, user photos. Path `pr/{user_id}/{pr_id}.png`. Backend proxy uploads only; presigned URLs for download.
- FCM for notifications. PR image ready: foreground = update UI (no notification); background/closed = system notification.

## Executor progress

**LLM Handler DSL Upgrade (complete):**
- Query DSL backend: Done. `internal/query/service.go` — `Query()` with scopes and metrics.
- Command DSL backend: Done. `internal/command/` — Executor with ENSURE_SESSION, CREATE_EXERCISE_ENTRY, APPEND_SET, UPDATE_SET, DELETE_SET, RESTORE_ENTRY, SET_NAME, UPDATE_NAME, CREATE_NOTE, DISABLE_ENTRY.
- LLM tools: Done. `internal/ai/tools.go` — query_history, execute_commands for OpenAI function calling.
- Agent loop: Done. `internal/chat/service.go` — Process() uses agent loop with tools (max 3 iterations).
- Response schema: Dropped intent. Response: message (Markdown), entries, history, prs.
- System prompt: `internal/ai/agent_prompt.go` — Jacked Street tone, tool usage.
- Docs: `docs/android-api.md` — unified response shape, Markdown note for clients (e.g. Markwon).
- Cleanup: Removed Parser from chat config and server.

Segments 1–6 done: Session repo+service, Log entry repo+service, Exercise repo+resolution. All tests pass. `make test` sources .env for GYM_DATABASE_URL.

Admin panel: RequireAdmin middleware, cookie-based user picker, dashboard, users, sessions, session detail, PRs, usage, notes. Login page for browser auth. Tests for RequireAdmin (403 non-admin, 200 admin). Admin handler tests require DB.

## Auth (completed)

- `internal/auth/verify.go` — VerifyGoogleToken using google.golang.org/api/idtoken
- `internal/auth/middleware.go` — RequireAuth middleware, get-or-create user, UserFromContext
- `internal/user/` — User struct, Repo (GetByGoogleID, Create, GetByEmail)
- `GET /me` — protected endpoint returning current user (for testing auth)
- Env: GYM_GOOGLE_CLIENT_ID (OAuth2 client ID), GYM_DATABASE_URL

## Dev token (completed)

- `GYM_DEV_MODE=true` — Enables GET /dev/token and Bearer dev:\<email\> auth
- `GET /dev/token` — Returns `{ "token": "dev:<email>" }` (default test@example.com; override with ?email=)
- Bearer dev:\<email\> — Middleware accepts; user looked up by email, created if missing (google_id = "dev-" + email)
- Documented in docs/api.md, .env.example

## Lessons

- **Gzip + ETag:** Gzip middleware compresses responses when client sends Accept-Encoding: gzip. ETag middleware on cacheable GET endpoints (sessions, exercises, query, prs) computes SHA256-based ETag and returns 304 when If-None-Match matches. Cache-Control: private, max-age=0, must-revalidate for user-specific data.

- **Fly Postgres password reset (postgres-flex):** When `fly postgres connect` fails with "password authentication failed", the postgres user password is out of sync with OPERATOR_PASSWORD/SU_PASSWORD. To reset: SSH in and use unix socket (peer auth). The socket is at `/var/run/postgresql/.s.PGSQL.5433` (port 5433, not 5432). Run: `fly ssh console -a gym-app-pg -C "su postgres -c \"psql -h /var/run/postgresql -p 5433 -d postgres -c \\\"ALTER USER postgres PASSWORD 'gym-dev-2025';\\\"\""` Then update .env to use that password. The docs' suggested `gym-dev-2025` only works after you've run this reset.
