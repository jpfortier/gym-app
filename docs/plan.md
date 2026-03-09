# Gym App Plan

## Backend Stack

| Layer | Choice |
|-------|--------|
| **Runtime** | Go |
| **HTTP** | net/http (stdlib) |
| **Database** | Postgres (Fly), pgvector for embeddings, database/sql + sqlx or pgx |
| **Migrations** | golang-migrate |
| **OpenAI** | go-openai (Whisper, GPT-4o, DALL-E 3) |
| **Auth** | Google Sign-In token verification (no Clerk) |
| **Hosting** | Fly.io |
| **Media storage** | Cloudflare R2 |
| **Caching** | None |

## Implementation Order

**Build the backend foundation first. No AI until the core flow works.**

1. **Session** — repo + service (GetOrCreateForDate — today, yesterday, any date)
2. **Log entry** — repo + service (create entry + sets)
3. **Exercise** — repo + basic resolution (exact match; embeddings later)
4. **Read endpoints** — GET /sessions, GET /sessions/:id (JSON responses)
5. **Query service** — fetch history by exercise
6. **GET /exercises, GET /prs** — list endpoints
7. **AI layer** — POST /chat, Whisper, parse, embeddings, PR images, correction

Validate core flow via tests. All creation goes through chat.

## AI Flow

- **Speech-to-text:** Audio → backend → Whisper API → text
- **Parsing:** Text → GPT-4o → structured log entry
- **Default date:** Assume today. LLM prompt instructs: user is logging in the moment unless they specify otherwise.
- **Session creation:** First log of the day creates a new workout session automatically. No confirmation. "Add to yesterday" — create session for that date if it doesn't exist. Support relative dates.
- **PR detection:** When processing a log intent, check if the logged sets establish a new PR (natural set or 1RM). If so, create PR record and trigger DALL-E image generation.
- **PR images:** DALL-E 3 per PR. Background job (~30 sec). Upload to R2 at `pr/{user_id}/{pr_id}.png`. Update `personal_records.image_url`. V1: client polls. V2: FCM.

## PR Image Ready (Client)

- **V1:** Client polls GET /prs/:id until image_url set, or polls on PR screen for ~60 sec.
- **V2:** FCM data message (foreground) or notification (background/closed).

## Notifications (V2)

- **FCM** for all event notifications: PR image ready, Jim's PR, new workout, etc.
- Store device tokens per user. Backend sends to FCM when events occur.

## Architecture (Layered)

Standard Go layering: **Handler → Service → Repository**. No business logic in handlers or repositories.

### Repository (data access only)

- **Pure CRUD.** No "get or create", no "one per day", no business rules.
- Methods like: `Create`, `GetByID`, `GetByUserAndDate`, `List`, `Update`, `Delete`.
- DB enforces integrity (e.g. `UNIQUE(user_id, date)` on `workout_sessions`); repo does not implement rules.
- One package per domain: `internal/session/repo.go`, `internal/logentry/repo.go`, etc.

### Service (business logic)

- **All rules and orchestration live here.** "One session per user per day", "first log creates session", PR detection, etc.
- **SessionService** — `GetOrCreateForDate(userID, date)` — today, yesterday, or any date. Creates session if it doesn't exist. Supports relative dates from LLM ("yesterday", "last Tuesday").
- **LogService** — creates entries, calls SessionService for session, resolves exercises, triggers PR check.
- **QueryService** — resolves exercise, fetches history, formats response.
- **CorrectionService** — finds target, applies changes, soft-delete if needed.
- Services call other services when needed (e.g. LogService → SessionService).

### Handler (HTTP only)

- **Thin.** Parse request, validate, call service, format response. No business logic.
- **ChatHandler** — receives text/audio, calls AI to parse, then delegates by intent:
  - Log → LogService
  - Query → QueryService
  - Correction → CorrectionService
- ChatHandler is an orchestrator, not a monolith. It routes; services do the work.

### Package layout

```
internal/
├── ai/             # OpenAI client, whisper, parse, image, usage
├── auth/           # Google token verify, RequireAuth middleware
├── chat/           # ChatService (orchestrates intents)
├── chatmessages/   # Conversation history for context
├── correction/     # CorrectionService
├── db/             # DB connection, sqlutil
├── env/            # GYM_* env vars
├── exercise/       # Exercise resolution, category/variant repos
├── logentry/       # LogService, log repo (entries + sets)
├── notes/          # User notes repo
├── pr/             # PR detection, PR repo, image trigger
├── query/          # QueryService
├── session/        # SessionService, session repo
├── storage/        # R2
├── usage/          # AI usage persistence
├── user/           # User repo
├── testutil/       # Shared test helpers
└── handler/        # HTTP handlers
```

## Structure

- Minimal layout: `cmd/api/`, `internal/`, `migrations/`
- No router library
- No caching layer
- **One file per handler**—prefer many small files over monoliths

## Logging

- **Library:** `log/slog` (stdlib, Go 1.21+). Structured key-value logging, JSON output.
- **Output:** stdout. Fly.io captures stdout/stderr automatically—no extra setup.
- **Built-in:** `fly logs` for live tail; Grafana log search (30 days retention).
- **External sink:** Deferred. Fly Log Shipper available when needed (Better Stack, S3, etc.).

## AI Service

Single `internal/ai/` package for all OpenAI calls. One file per capability.

```
internal/ai/
├── client.go      # Wraps go-openai, holds UsageTracker
├── whisper.go     # Transcribe audio
├── parse.go       # Parse workout text → structured data
├── image.go       # Generate PR image (DALL-E)
└── usage.go       # Token tracking, cost calc, persistence
```

**Token tracking**
- Read `resp.Usage` from each OpenAI response
- Store in UsageTracker (in-memory + optional DB table)
- Compute cost from model pricing (GPT-4o, Whisper, DALL-E)
- Expose via admin endpoint for dashboard

**Throttling**
- `golang.org/x/time/rate` per user
- Example: 20 requests/min per user
- Return 429 when over limit

## AI Inferring System (Hybrid Approach)

**Don't force the LLM to pick from a fixed list.** Give it context, let it output freely, backend resolves.

### Flow
1. **LLM gets context** — e.g. user's recent exercises (global + user-level) as hints, not constraints
2. **LLM outputs structured JSON** — `{exercise_name, sets: [{weight, reps}, ...], ...}` or `{intent: "query", exercise_ref, scope}`
3. **Backend resolves** — embeddings + cosine similarity against DB (global categories + user's). If no match → create new

### Intent classification
Single LLM call: classify intent (log vs query vs correction) and parse in one step.
- **Log:** `{intent: "log", exercise_name, sets: [{weight, reps, set_type?}, ...], notes?, raw_speech?}` — supports ramp/pyramid. `raw_speech` = exact text user said for this exercise block (per-exercise segment).

### Partial logging (pyramid / build-up)
User may mention only the peak set, or all sets, or the build-up. LLM parses what they say; backend stores exactly that.
- **Peak only:** "squats 195×1" → one set. No need to infer the warm-up.
- **Full pyramid:** "squats 135×8, 155×6, 175×4, 195×1" → four sets.
- **Build-up:** "squats worked up to 195 for 1" → one set (the peak).
- **Query:** `{intent: "query", exercise_ref, scope: "all"|"variation"}`
- **Correction:** `{intent: "correction", target_ref, changes: {...}}` — e.g. "that last one was 165, not 135" or "change my bench yesterday from 140 to 150"

### Exercise resolution
- **Embeddings** — OpenAI embeddings; store vectors for categories + variants; cosine similarity for "RDL" ↔ "Romanian deadlift"
- **Create new** — if no match above threshold, create user-level category/variant
