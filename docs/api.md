# API Design

## Public Endpoints (Android app)

| Method | Endpoint | Purpose |
|--------|----------|---------|
| `POST` | `/chat` | Main entry. Sends text (or audio). Server infers intent (log, query, correction) via LLM. Returns log confirmation, query results, or correction confirmation. *(AI layer—build after core flow works.)* |
| `GET` | `/sessions` | List workout sessions (timeline). Optional: `?from=&to=` or `?limit=`. |
| `GET` | `/sessions/:id` | Session detail with log entries and sets. |
| `GET` | `/exercises` | List categories + variants. Includes show_weight, show_reps for UI (which fields to display). |
| `GET` | `/prs` | User's personal records. |

All log creation goes through POST /chat. No manual write endpoint.

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
- **Response:** Varies by intent (log, query, correction, remove). Server infers via LLM.
- **Auth:** `Authorization: Bearer <google_id_token>`
- **Throttling:** Per-user rate limits. See `docs/ai-throttling.md`. Set `OPENAI_TEST_MODE=true` for tests.

**Exercise resolution (log intent):** Resolves exercise names (e.g. "RDL") to category/variant. Order: exact match → user alias lookup → embedding similarity → create new. When we resolve via embedding or create, we store the alias so future lookups skip the LLM.

**Remove intent:** User says "forget that", "remove it", "delete the last bench", "scratch that". Soft-deletes (disables) the matching log entry. If no exercise specified, removes the most recent entry for today.

**Restore intent:** User says "bring that back", "oh sorry bring it back", "restore that" after having removed something. Restores the most recently disabled entry for today.

## GET /prs/{id}/image

- **Purpose:** Redirect to presigned URL for PR image. Returns 302.
- **Auth:** Required. PR must belong to user.

## Error Responses

- **Format:** `{ "error": "human message", "code": "not_found", "error_token": "err_abc123" }`
- **error_token:** Unique ID per error. Logged server-side with full context (stacktrace, request, user). Client displays it for bug reports. User screenshots and sends; developer searches logs by token to find the flow.
- **code:** Machine-readable (e.g. `not_found`, `unauthorized`, `invalid_input`).
- **error:** Safe message for display. No stacktrace to client.

## Auth

- No separate login endpoint. Client sends Google ID token with each request.
- Server verifies token and derives user.

## Admin (separate)

- Different auth. `GET /admin/users`, `GET /admin/usage`, `DELETE /admin/users/:id`, etc.
