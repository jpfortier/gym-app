# Notes & Decisions

## Decisions

- **Build order:** Backend foundation first (sessions, log entries, exercises, endpoints). No AI until the core flow works. Validate via tests.
- **Log creation:** All creation via POST /chat. No manual write endpoint.
- **Display names:** Server-side join. Session, query, and PR responses include category/variant names from exercise tables. No client lookup.
- **Session ownership:** GET /sessions/:id verifies the session belongs to the authenticated user. Return 404 if not.
- **Health check:** GET /health pings the database. Returns 503 if DB is down.
- **Display flags:** exercise_categories has show_weight, show_reps per field. Tells UI which fields are relevant. Weight nullable for bodyweight.
- **Past dates:** "Add to yesterday's log" — create session for that date if it doesn't exist. Support relative dates (yesterday, last Tuesday, etc.).
- **Error tokens:** Every error response includes `error_token` (e.g. `err_abc123`). Logged server-side with full context. Client shows token for user to report; developer searches logs to find stacktrace without exposing it.
- **Testing:** Test early, test often. Add tests as soon as something is testable. At least one assertion of value per test.
- **Auth:** Google Sign-In only. No Clerk, no magic link.
- **Distribution:** APK via S3 or R2 for now. In-app update flow. Play Store when ID verification completes.
- **Exercise resolution:** Exact match → user alias lookup → embedding similarity → create new. Aliases learned when we resolve via embedding or create (e.g. "RDL" → Deadlift/RDL); future lookups skip the LLM.
- **Corrections:** Via chat, not a separate PATCH endpoint. "Change my bench yesterday from 140 to 150" → LLM infers correction.
- **Remove/undo:** Via chat. "Forget that", "remove the last bench", "scratch that" → soft-delete (disabled_at) the matching entry. "Bring that back", "oh sorry bring it back" → restore (undo the remove).
- **Intent:** Client never chooses. Single POST /chat; LLM infers log vs query vs correction vs note.
- **Note intent:** Phrases like "remember for RDLs: warm up hamstrings", "note for deadlift: brace core" infer note intent. Notes stored in `notes` table, scoped to user and optionally category/variant. Discussion point for AI to surface relevant notes when user logs that exercise.
- **AI usage:** Token usage persisted to `ai_usage` per user for Chat, Transcribe, Embed, DALL-E. Cost tracking and admin dashboards.
- **Chat context:** Conversation history in `chat_messages` for context-aware parsing. Sliding window (last 6 messages) passed to parser. Enables "and another one for 150", "change that", "remove that". Retention: keep forever. See `docs/chat-context.md`.
- **Log structure:** Block + sets (log_entries + log_entry_sets). Supports ramp/pyramid.
- **raw_speech:** Store the exact text the user said for each exercise block (per-exercise segment, not full paragraph). Enables reprocessing if parsing improves.
- **Partial logging:** User may say only the peak ("squats 195×1") or all sets. LLM parses what they say; we store exactly that. No inferring warm-up sets.
- **Exercises:** Global seed list + user-level. Variants flexible, AI creates as needed.
- **Local port:** 8081 (8080 often in use).
- **Media storage:** Cloudflare R2. Bucket `gym-app`, account `23acac6fd2f9179de96adf9599129074`. PR images at `pr/{user_id}/{pr_id}.png`, user photos in R2. Future (not v1): workout list photos, routine screenshots.
- **Upload:** Never client → R2 directly. Backend proxy only. Client sends to our API; backend uploads to R2.
- **Download:** Direct from R2 via presigned URLs. Backend validates ownership, returns time-limited URL. Client fetches from R2.
- **Sharing:** Same object stays in R2; no copy. Sharing = backend generates presigned URL with longer expiry. Private bucket.
- **PR image flow:** DALL-E generates (~30 sec) → background job uploads to R2 → updates `personal_records.image_url` → notifies client.
- **Notifications:** V2. FCM for PR image ready, Jim's PR, new workout, etc.
- **PR image ready (V1):** Client polls GET /prs/:id until image_url set. V2: FCM.
- **Admin panel:** Alpine.js + Go templates. Same backend. Server-rendered HTML, Alpine for interactivity. Dashboard (higher-level views) + raw table CRUD.
- **Roles:** `users.role` — 'user' | 'coach' | 'owner' | 'admin'. No boolean flags. One user (me) has 'admin'. Same Google Sign-In; middleware checks role for admin routes. Set admin manually: `UPDATE users SET role = 'admin' WHERE email = 'your@email.com';`

## Development workflow

Build segment by segment. Each segment gets a test before moving on.

1. **Create one segment/block** — Implement the feature.
2. **Write a test for it** — Test must pass before proceeding.
3. **Create the next segment** — Implement.
4. **Write a test for the new segment** — Test must pass.
5. **Write an integration test** — Test the two segments together.
6. **Repeat** — Each feature needs a test. Run tests continuously.

**Test structure:**
- **Shared setup** — Authenticate/login once at the beginning of a test file or suite. Don't duplicate auth/setup across every test.
- **Cleanup** — Clean up after each test (e.g. delete created data, reset state). Tests should not leave side effects.
- **Logout/teardown** — If you logged in, log out or tear down at the end.

## Migrations (release command)

Migrations run automatically before each deploy via `release_command`. Requires `GYM_DATABASE_URL` or `DATABASE_URL` (Fly postgres attach sets `DATABASE_URL`). Set with `fly postgres attach gym-app-pg --app gym-app` or `fly secrets set GYM_DATABASE_URL="postgres://..." -a gym-app`.

**Deploy to Fly:** See `docs/deploy-fly.md` for secrets, env vars, and deployment steps.

## Local DB setup

Fly Postgres does not accept direct connections from outside Fly (prtracks.com rejects connections). Use the Fly proxy for local dev:

1. Start the proxy: `fly proxy 15432:5432 -a gym-app-pg` (port 15432 avoids conflicts with local Postgres on 5432)
2. Copy `.env.example` to `.env` and set `GYM_DATABASE_URL=postgres://postgres:PASSWORD@localhost:15432/postgres?sslmode=disable`
3. Run migrations: `make migrate-up` (or `migrate -path migrations -database $GYM_DATABASE_URL up`)

Keep the proxy running in a terminal while developing. Stop with Ctrl+C.

**Migrate CLI:** Install with postgres driver: `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`. Ensure `$(go env GOPATH)/bin` is in PATH.

**pgvector (migration 000008):** Embeddings require the pgvector extension. For Fly Postgres Flex: use `ziadm/postgres-flex-pgvector:17.2` image (`fly image update --image ziadm/postgres-flex-pgvector:17.2 -a gym-app-pg -y`), then `CREATE EXTENSION vector` and run migrations. For Fly MPG: enable in dashboard → Postgres (Beta) → Extensions. For local: `brew install pgvector`.

**GYM_ prefix:** All gym env vars use `GYM_` prefix (e.g. `GYM_DATABASE_URL`) to avoid collisions with other projects. If you have `DATABASE_URL` set in your shell for worklist or another project, gym will use `GYM_DATABASE_URL` from `.env` instead.

**Reset password** (if needed): `printf 'ALTER USER postgres PASSWORD '\''newpass'\'';\n\\q\n' | fly postgres connect -a gym-app-pg`. If that fails with "password authentication failed", use SSH + unix socket: `fly ssh console -a gym-app-pg -C "su postgres -c \"psql -h /var/run/postgresql -p 5433 -d postgres -c \\\"ALTER USER postgres PASSWORD 'newpass';\\\"\""` (postgres-flex uses port 5433 for the socket).

## Env vars

| Var | Required | Purpose |
|-----|----------|---------|
| `GYM_DATABASE_URL` | Yes | Postgres connection string |
| `GYM_GOOGLE_CLIENT_ID` | Yes | OAuth2 client ID for Google Sign-In |
| `GYM_PORT` | No | HTTP/HTTPS port (default 8081; Fly sets PORT) |
| `GYM_TLS_CERT_FILE` | When HTTPS | Path to TLS cert. With TLS_KEY_FILE, server uses HTTPS. |
| `GYM_TLS_KEY_FILE` | When HTTPS | Path to TLS key. With TLS_CERT_FILE, server uses HTTPS. |
| `GYM_R2_ACCOUNT_ID` | When R2 | Cloudflare account ID |
| `GYM_R2_ACCESS_KEY_ID` | When R2 | R2 API token |
| `GYM_R2_SECRET_ACCESS_KEY` | When R2 | R2 API secret |
| `GYM_R2_BUCKET` | When R2 | Bucket name (gym-app) |
| `GYM_FCM_CREDENTIALS_PATH` | When FCM | Path to Firebase service account JSON |
| `GYM_OPENAI_API_KEY` | When AI | OpenAI API key. Verify: `make verify-openai` |
| `GYM_OPENAI_TEST_MODE` | When AI | Set `true` for tests. Skips real API calls; uses mocks. |
| `GYM_OPENAI_RATE_PER_MINUTE` | No | Per-user rate limit (default 10). |
| `GYM_OPENAI_DAILY_LIMIT` | No | Per-user daily cap (default 100). |
| `GYM_OPENAI_DALLE_DAILY_LIMIT` | No | Per-user DALL-E cap (default 5). |

**AI throttling:** See `docs/ai-throttling.md`. Tests must never call real OpenAI. Set `GYM_OPENAI_TEST_MODE=true` in `.env` when running `make test`.

Copy `.env.example` to `.env`. Unset optional vars are ignored; app works without R2/FCM/OpenAI until those features are used.

**HTTPS (local):** Set `GYM_TLS_CERT_FILE` and `GYM_TLS_KEY_FILE` for HTTPS. Use mkcert: `mkcert -install && mkcert -cert-file certs/cert.pem -key-file certs/key.pem localhost 127.0.0.1 10.0.2.2` (10.0.2.2 = Android emulator host). Android emulator trusts mkcert roots when you run `mkcert -install`.

## Gotchas

- **GYM_GOOGLE_CLIENT_ID:** Required for auth. OAuth2 client ID from Google Cloud Console (Android or Web client).
- **Fly Postgres connection:** Use `/postgres` in URL for default DB: `...@host:port/postgres?sslmode=disable`
- **DBeaver:** Run `fly proxy 15432:5432 -a gym-app-pg`, then connect to `localhost:15432`

## Deferred

- Play Store distribution (ID verification)
- pgvector/embeddings migration (add when ready)

## Sample Routines (real)

Reference for DB design and set_type variety:

- Close Grip Bench Press — 4 × 8 — For Weight
- Deadlift — 6 × 3 — Speed Pull / For Weight
- Back Squat — 5 × 6 — CAT 6S / For Weight
- Close Grip Bench Press — 5 × 6 — CAT 6S / For Weight
- Barbell RDL — 4 × 6 — Heavy Pull / For Weight
- Heel Elevated Back Squats — 8, 6, 4, MAX — PUMP / For Weight

## Repo structure

- **gym-app** — Backend (Go, API, migrations). https://github.com/jpfortier/gym-app
- **gym-app-android** — Android app. https://github.com/jpfortier/gym-app-android

## Context

- Dev workflow: one thing at a time, scratchpad for tracking
- User: 20+ years web dev, fewer mobile apps
- Audience: self + friends, not a major public project
