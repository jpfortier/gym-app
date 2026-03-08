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
- **Remove/undo:** Via chat. "Forget that", "remove the last bench", "scratch that" → soft-delete (disabled_at) the matching entry.
- **Intent:** Client never chooses. Single POST /chat; LLM infers log vs query vs correction.
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

Migrations run automatically before each deploy via `release_command`. Requires `DATABASE_URL` secret on gym-app. Set with `fly postgres attach gym-app-pg --app gym-app` (creates secret) or `fly secrets set DATABASE_URL="postgres://..." -a gym-app`.

## Local DB setup

1. Start proxy: `fly proxy 15432:5432 -a gym-app-pg`
2. Copy `.env.example` to `.env` and set `DATABASE_URL=postgres://postgres:gym-dev-2025@localhost:15432/postgres?sslmode=disable`
3. Run migrations: `make migrate-up` (or `migrate -path migrations -database $DATABASE_URL up`)

**Migrate CLI:** Install with postgres driver: `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`. Ensure `$(go env GOPATH)/bin` is in PATH.

**pgvector (migration 000008):** Embeddings require the pgvector extension. For Fly Postgres Flex: use `ziadm/postgres-flex-pgvector:17.2` image (`fly image update --image ziadm/postgres-flex-pgvector:17.2 -a gym-app-pg -y`), then `CREATE EXTENSION vector` and run migrations. For Fly MPG: enable in dashboard → Postgres (Beta) → Extensions. For local: `brew install pgvector`.

**DATABASE_URL conflict:** If you have `DATABASE_URL` set in your shell (e.g. for another project), it overrides `.env`. Use `make migrate-up` and `make test` — both source `.env` so gym's DATABASE_URL takes precedence.

**Reset password** (if needed): `printf 'ALTER USER postgres PASSWORD '\''newpass'\'';\n\\q\n' | fly postgres connect -a gym-app-pg`

## Env vars

| Var | Required | Purpose |
|-----|----------|---------|
| `DATABASE_URL` | Yes | Postgres connection string |
| `GOOGLE_CLIENT_ID` | Yes | OAuth2 client ID for Google Sign-In |
| `PORT` | No | HTTP port (default 8081; Fly sets automatically) |
| `R2_ACCOUNT_ID` | When R2 | Cloudflare account ID |
| `R2_ACCESS_KEY_ID` | When R2 | R2 API token |
| `R2_SECRET_ACCESS_KEY` | When R2 | R2 API secret |
| `R2_BUCKET` | When R2 | Bucket name (gym-app) |
| `FCM_CREDENTIALS_PATH` | When FCM | Path to Firebase service account JSON |
| `OPENAI_API_KEY` | When AI | OpenAI API key. Create at https://platform.openai.com/api-keys. Add to .env. Verify: `make verify-openai` |
| `OPENAI_TEST_MODE` | When AI | Set `true` for tests. Skips real API calls; uses mocks. **Prevents test suites from burning credits.** |
| `OPENAI_RATE_PER_MINUTE` | No | Per-user rate limit (default 10). |
| `OPENAI_DAILY_LIMIT` | No | Per-user daily cap (default 100). |
| `OPENAI_DALLE_DAILY_LIMIT` | No | Per-user DALL-E cap (default 5). |

**AI throttling:** See `docs/ai-throttling.md`. Tests must never call real OpenAI. Set `OPENAI_TEST_MODE=true` in `.env` when running `make test`.

Copy `.env.example` to `.env`. Unset optional vars are ignored; app works without R2/FCM/OpenAI until those features are used.

## Gotchas

- **GOOGLE_CLIENT_ID:** Required for auth. OAuth2 client ID from Google Cloud Console (Android or Web client).
- **Fly Postgres connection:** Use `/postgres` in URL for default DB: `...@host:port/postgres?sslmode=disable`
- **DBeaver:** Run `fly proxy 15432:5432 -a gym-app-pg`, connect to localhost:15432

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
