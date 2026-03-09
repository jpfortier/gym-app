# Gym App — Android Developer AI Reference

Complete reference for building the Android client. Copy this into your context when working on the Android app.

---

## Environment & Base URL

| Environment | Base URL | Notes |
|-------------|----------|-------|
| Production | `https://gym-app.fly.dev` | Deployed backend |
| Local (emulator) | `https://10.0.2.2:8081` | Emulator → host. Backend uses mkcert TLS. |
| Local (device) | `https://<your-mac-ip>:8081` | Add your IP to mkcert cert if needed |

**Android:** API 28+ blocks cleartext HTTP by default. Use HTTPS. For local dev, backend serves HTTPS with mkcert certs; emulator trusts mkcert roots after `mkcert -install` on host.

---

## Authentication

**Google Sign-In only.** No login endpoint.

- Use Android Google Sign-In SDK to obtain an **ID token** (not access token)
- Send with every request: `Authorization: Bearer <id_token>`
- Server verifies token and derives user. Token expires; refresh and retry on 401
- All endpoints except `/health` require auth

---

## Headers

| Header | Value |
|--------|-------|
| `Authorization` | `Bearer <google_id_token>` |
| `Content-Type` | `application/json` (for POST body) |

---

## Endpoints

### GET /me

**Auth:** Required  
**Purpose:** Verify auth and get current user.

**Response 200:**
```json
{
  "id": "uuid",
  "email": "user@example.com",
  "name": "User Name",
  "photo_url": "https://..." | null
}
```

---

### POST /chat

**Auth:** Required  
**Purpose:** Main entry point. Log workouts, query history, correct, remove, restore, add notes. Server infers intent from natural language.

**Request (text):**
```json
{
  "text": "bench press 135 for 8"
}
```

**Request (audio):**
```json
{
  "audio_base64": "base64-encoded-audio",
  "audio_format": "m4a"
}
```
- `text` and `audio_base64` are mutually exclusive
- `audio_format` optional: `"m4a"`, `"webm"`, etc. Defaults to webm if omitted

**Response:** Varies by `intent`. All include `intent` and usually `message`.

| Intent | Example phrases |
|--------|------------------|
| log | "bench 135 for 8", "squats 185x5", "RDL 135 for 6" |
| query | "what's my last bench", "how much did I deadlift" |
| correction | "change that to 140", "that was 6 reps not 8" |
| remove | "forget that", "remove the last bench" |
| restore | "bring that back", "oh sorry undo" |
| note | "remember for RDLs: warm up hamstrings" |
| unknown | Fallback when intent unclear |

**Log response:**
```json
{
  "intent": "log",
  "message": "Logged.",
  "entries": [
    {
      "exercise_name": "Bench Press",
      "variant_name": "standard",
      "session_date": "2025-03-08",
      "entry_id": "uuid"
    }
  ],
  "prs": [
    {
      "id": "uuid",
      "exercise_name": "Bench Press",
      "variant_name": "standard",
      "weight": 135,
      "reps": 8,
      "pr_type": "natural_set"
    }
  ]
}
```
- `prs` present when new PR(s) detected
- `message` may be `"Logged. N new PR(s)!"` when PRs created

**Query response:**
```json
{
  "intent": "query",
  "history": {
    "exercise_name": "Bench Press",
    "variant_name": "standard",
    "entries": [
      {
        "session_date": "2025-03-08",
        "raw_speech": "bench 135x8",
        "sets": [
          { "weight": 135, "reps": 8, "set_type": "working" }
        ],
        "created_at": "2025-03-08T14:30:00Z"
      }
    ]
  }
}
```

**Correction / Remove / Restore / Note:**
```json
{
  "intent": "correction",
  "message": "Corrected."
}
```
(Similar for remove, restore, note with appropriate message.)

**Unknown:**
```json
{
  "intent": "unknown",
  "message": "I didn't understand. Try logging a workout, asking about your history, correcting a previous entry, or removing something."
}
```

**Context:** Server keeps last 6 messages. Follow-ups like "and another one for 150", "change that to 6 reps" work because parser sees conversation history.

**Throttling:** 429 when over per-user rate limit. Default 10/min, 100/day. Show user-friendly message.

---

### GET /sessions

**Auth:** Required  
**Purpose:** List workout sessions (timeline). Most recent first.

**Query params:** `limit` (int, default 50, max 100)

**Response 200:**
```json
[
  {
    "id": "uuid",
    "date": "2025-03-08",
    "created_at": "2025-03-08T14:00:00Z"
  }
]
```

---

### GET /sessions/{id}

**Auth:** Required  
**Purpose:** Session detail with log entries and sets.

**Path:** `id` = session UUID

**Response 200:**
```json
{
  "id": "uuid",
  "date": "2025-03-08",
  "created_at": "2025-03-08T14:00:00Z",
  "entries": [
    {
      "id": "uuid",
      "exercise_variant_id": "uuid",
      "exercise_name": "Bench Press",
      "variant_name": "close grip",
      "raw_speech": "close grip bench 140x8",
      "notes": "",
      "sets": [
        { "weight": 140, "reps": 8, "set_type": "working" }
      ]
    }
  ]
}
```
- 404 if session not found or not owned by user

---

### GET /query

**Auth:** Required  
**Purpose:** History for a specific exercise (by category/variant).

**Query params:**
| Param | Type | Required | Default |
|-------|------|----------|---------|
| `category` or `exercise` | string | Yes | — |
| `variant` | string | No | "standard" |
| `limit` | int | No | 20, max 50 |
| `from` | YYYY-MM-DD | No | — |
| `to` | YYYY-MM-DD | No | — |

**Response 200:**
```json
{
  "entries": [
    {
      "session_date": "2025-03-08",
      "raw_speech": "bench 135x8",
      "sets": [
        { "weight": 135, "reps": 8, "set_type": "working" }
      ],
      "created_at": "2025-03-08T14:30:00Z"
    }
  ],
  "exercise_name": "Bench Press",
  "variant_name": "standard"
}
```

---

### GET /exercises

**Auth:** Required  
**Purpose:** List all exercise categories and variants (global + user-level).

**Response 200:**
```json
[
  {
    "category_id": "uuid",
    "category_name": "Bench Press",
    "variant_id": "uuid",
    "variant_name": "standard",
    "show_weight": true,
    "show_reps": true
  }
]
```
- `show_weight`, `show_reps` indicate which fields to display in UI

---

### GET /prs

**Auth:** Required  
**Purpose:** User's personal records.

**Response 200:**
```json
[
  {
    "id": "uuid",
    "exercise_variant_id": "uuid",
    "exercise_name": "Bench Press",
    "variant_name": "standard",
    "pr_type": "natural_set",
    "weight": 135,
    "reps": 8,
    "image_url": "pr/user-id/pr-id.png",
    "created_at": "2025-03-08T14:30:00Z"
  }
]
```
- `image_url` may be `null` if DALL-E image not yet ready (poll GET /prs/{id}/image)

---

### GET /prs/{id}/image

**Auth:** Required  
**Purpose:** Redirect to presigned URL for PR image. Returns 302.

**Path:** `id` = PR UUID

**Response:** 302 redirect to R2 presigned URL (1 hour expiry). Follow redirect to load image.

**Errors:** 404 if PR not found, not owned by user, or image not ready.

**PR image flow:** DALL-E generates (~30 sec) after PR created. Poll GET /prs until `image_url` is set, or poll this endpoint.

---

### GET /health

**Auth:** None  
**Purpose:** Health check. Pings database.

**Response 200:** `{"status":"ok"}`  
**Response 503:** Database down

---

## Error Responses

All errors (except 404/503 for specific cases) return:

```json
{
  "error": "Human-readable message",
  "code": "machine_readable_code",
  "error_token": "err_abc123"
}
```

**Codes:** `unauthorized`, `invalid_input`, `not_found`, `internal_error`, `method_not_allowed`, `missing_auth`, `invalid_token`

**error_token:** Unique per error. Display for bug reports; developer searches logs by token.

**HTTP status:** 400, 401, 404, 429, 500, 503 as appropriate.

---

## Data Types

| Field | Type | Notes |
|-------|------|-------|
| `id`, `entry_id`, etc. | UUID | RFC 4122 format |
| `date`, `session_date` | string | YYYY-MM-DD |
| `created_at` | string | ISO 8601 (e.g. 2025-03-08T14:30:00Z) |
| `weight` | number | Pounds, nullable for bodyweight |
| `reps` | int | |
| `set_type` | string | "warm-up", "working", "drop", etc. |
| `pr_type` | string | "natural_set", etc. |

---

## Summary: Client Actions

| Action | Endpoint | Notes |
|--------|----------|-------|
| Verify auth | GET /me | After Google Sign-In |
| Log workout | POST /chat | Text or audio; server infers |
| Query history | POST /chat or GET /query | Chat: natural language. Query: direct params |
| Correct entry | POST /chat | "change that to 140" |
| Remove entry | POST /chat | "forget that" |
| Restore entry | POST /chat | "bring that back" |
| Add note | POST /chat | "remember for RDLs: warm up" |
| List sessions | GET /sessions | Timeline |
| Session detail | GET /sessions/{id} | Entries + sets |
| Exercise history | GET /query | By category/variant |
| List exercises | GET /exercises | Categories + variants |
| List PRs | GET /prs | With image_url |
| PR image | GET /prs/{id}/image | 302 redirect; follow to load |
| Health check | GET /health | No auth |

---

## Android-Specific Notes

1. **Base URL:** Use `https://10.0.2.2:8081` for emulator. Use build config or BuildConfig to switch prod vs local.
2. **HTTPS:** Required. No cleartext. Local backend uses mkcert; emulator trusts after `mkcert -install` on dev machine.
3. **Google Sign-In:** Use server client ID (Web client type) for ID token verification, or Android client ID if backend is configured for it. Check `GYM_GOOGLE_CLIENT_ID` matches your OAuth client.
4. **Token refresh:** ID tokens expire. Use `requestIdToken` / token refresh before each request or on 401.
5. **PR images:** GET /prs/{id}/image returns 302. Use HTTP client that follows redirects. Image URL is presigned, time-limited.
6. **Error display:** Show `error` to user. Optionally show `error_token` for support/debug.
