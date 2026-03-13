# API Design

## Public Endpoints (Android app)

| Method | Endpoint | Purpose |
|--------|----------|---------|
| `GET` | `/me` | Current user (id, email, name, photo_url). Verify auth. |
| `GET` | `/chat/messages` | Chat history. Lazy-load with `?before=<id>`. Default limit 6. |
| `POST` | `/chat` | Main entry. Text or audio. Server infers intent (log, query, correction, remove, restore, note). |
| `GET` | `/sessions` | List workout sessions (timeline). Optional: `?limit=`. |
| `GET` | `/sessions/{id}` | Session detail with log entries and sets. |
| `GET` | `/query` | History by exercise. Params: `category`, `variant`, `limit`, `from`, `to`. |
| `GET` | `/exercises` | List categories + variants. Includes show_weight, show_reps for UI. |
| `GET` | `/prs` | User's personal records. |
| `GET` | `/prs/{id}/image` | Redirect to presigned PR image URL. 302. |
| `GET` | `/health` | Health check. No auth. |

All log creation goes through POST /chat. No manual write endpoint. See `docs/android-api.md` for full client reference.

## GET /chat/messages

- **Purpose:** Load chat history for display. Initial load returns last N messages (default 6). Lazy-load older messages when user scrolls up.
- **Query params:** `limit` (default 6, max 50), `before` (message UUID for cursor-based pagination; omit for initial load).
- **Response:** `{ "messages": [{ "id": "uuid", "role": "user"|"assistant", "content": "...", "created_at": "..." }] }`
- **Order:** Chronological (oldest first). Append new messages from POST /chat to local list.

## GET /query

- **Query params:** `category` or `exercise` (required), `variant` (default "standard"), `limit` (default 20), `from` (YYYY-MM-DD), `to` (YYYY-MM-DD)
- **Response:** `{ "entries": [...], "exercise_name": "...", "variant_name": "..." }`

## GET /sessions

- **Response:** JSON array of sessions.
- **Example:** `[{ "id": "uuid", "date": "2025-03-06", "created_at": "..." }, ...]`

## GET /sessions/:id

- **Ownership:** Session must belong to the authenticated user. Return 404 if not.
- **Response:** JSON object with session and nested log entries + sets.
- **Server-side join:** Entries include category and variant names from exercise tables. No client lookup.
- **Example:** `{ "id": "uuid", "date": "2025-03-06", "entries": [{ "id": "uuid", "exercise_variant_id": "uuid", "exercise_name": "Bench Press", "variant_name": "close grip", "raw_speech": "close grip bench 140×8", "notes": "...", "sets": [{ "weight": 140, "reps": 8, "set_type": "working" }, ...] }] }`

## POST /chat

- **Request:** `{ "text": "..." }` or `{ "audio_base64": "..." }` (optional `audio_format`, e.g. `"m4a"`)
- **Response (text):** Full response with message, entries, history, prs. Server infers intent via LLM.
- **Response (audio):** Returns immediately with `{ "job_id": "uuid", "text": "transcribed...", "status": "processing" }`. Poll `GET /chat/jobs/{job_id}` for the LLM result. Show `text` in UI right away.
- **Auth:** `Authorization: Bearer <google_id_token>`
- **Throttling:** Per-user rate limits. See `docs/ai-throttling.md`. Set `GYM_OPENAI_TEST_MODE=true` for tests.

## GET /chat/jobs/{id}

- **Purpose:** Poll for async chat job completion (audio input).
- **Response:** `{ "job_id": "...", "text": "...", "status": "processing"|"complete"|"failed", "result": {...}, "error": "..." }`. When `status` is `complete`, `result` has the same shape as POST /chat text response.

**Exercise resolution (log intent):** Resolves exercise names (e.g. "RDL") to category/variant. Order: exact match → user alias lookup → embedding similarity → create new. When we resolve via embedding or create, we store the alias so future lookups skip the LLM.

**Remove intent:** User says "forget that", "remove it", "delete the last bench", "scratch that". Soft-deletes (disables) the matching log entry. If no exercise specified, removes the most recent entry for today.

**Restore intent:** User says "bring that back", "oh sorry bring it back", "restore that" after having removed something. Restores the most recently disabled entry for today.

**Note intent:** User says "remember for RDLs: warm up hamstrings", "note for deadlift: brace core", "reminder: stretch before squats". AI infers note intent from phrases like "remember", "note for", "reminder". Notes are stored per user, optionally scoped to a category or variant. Global notes (no exercise) and variant-specific notes supported.

**AI usage:** Token usage (prompt/completion) for Chat, Transcribe, Embed, and DALL-E is persisted to `ai_usage` per user. Used for cost tracking and admin dashboards.

**Context:** Conversation history stored in `chat_messages`. Last 6 messages passed to parser. Enables follow-ups like "and another one for 150", "change that to 6 reps". See `docs/chat-context.md`.

## GET /prs/{id}/image

- **Purpose:** Redirect to presigned URL for PR image. Returns 302 when ready; 404 while DALL-E is still generating.
- **Auth:** Required. PR must belong to user.
- **Polling:** Client polls until 302 (image ready). Interval 3–5 s, timeout ~60 s. See `docs/android-api.md` for full flow.

## Error Responses

- **Format:** `{ "error": "human message", "code": "not_found", "error_token": "err_abc123" }`
- **error_token:** Unique ID per error. Logged server-side with full context (stacktrace, request, user). Client displays it for bug reports. User screenshots and sends; developer searches logs by token to find the flow.
- **code:** Machine-readable (e.g. `not_found`, `unauthorized`, `invalid_input`).
- **error:** Safe message for display. No stacktrace to client.

## Auth

- No separate login endpoint. Client sends Google ID token with each request.
- Server verifies token and derives user.

### Dev token (automated tests)

When `GYM_DEV_MODE=true`:

- **GET /dev/token** — Returns `{ "token": "dev:<email>" }`. Default email `test@example.com`; override with `?email=...`. Returns 404 when dev mode is off.
- **Bearer dev:\<email\>** — Accepted by RequireAuth. User is looked up by email; if missing, created with `google_id = "dev-" + email`.

Use for Playwright, integration tests, etc. Never enable `GYM_DEV_MODE` in production.

## Admin (separate)

- Different auth. `GET /admin/users`, `GET /admin/usage`, `DELETE /admin/users/:id`, etc.
