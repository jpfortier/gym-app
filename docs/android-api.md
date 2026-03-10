# Gym App API — Android Client Reference

Reference for the Android app. All endpoints require authentication unless noted.

## Base URL

- **Production:** `https://gym-app.fly.dev` (or your deployed URL)
- **Local (HTTP):** `http://10.0.2.2:8081` (Android emulator → host)
- **Local (HTTPS):** `https://10.0.2.2:8081` when `GYM_TLS_CERT_FILE` and `GYM_TLS_KEY_FILE` are set. Use mkcert for trusted certs; see `docs/notes.md` Env vars.

## Authentication

**Google Sign-In only.** No separate login endpoint.

- Obtain a Google ID token from the Android Google Sign-In SDK
- Send with every request: `Authorization: Bearer <id_token>`
- Server verifies token and derives user. Token expires; refresh as needed

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

### GET /chat/messages

**Auth:** Required

**Purpose:** Load chat history. Call on app open for initial messages. Lazy-load older messages when user scrolls up.

**Query params:**
| Param | Type | Default | Description |
|-------|------|---------|--------------|
| `limit` | int | 6 | Max messages (1–50) |
| `before` | uuid | — | Cursor for older messages. Omit for initial load. |

**Response 200:**
```json
{
  "messages": [
    {
      "id": "uuid",
      "role": "user",
      "content": "bench press 135 for 8",
      "created_at": "2025-03-08T14:30:00Z"
    },
    {
      "id": "uuid",
      "role": "assistant",
      "content": "Logged bench press 135×8.",
      "created_at": "2025-03-08T14:30:01Z"
    }
  ]
}
```
- **Initial load:** `GET /chat/messages?limit=6` — last 6 messages, chronological.
- **Scroll up:** `GET /chat/messages?before=<oldest_message_id>&limit=6` — 6 older messages. Prepend to list.

---

### POST /chat

**Auth:** Required

**Purpose:** Main entry point. Log workouts, query history, correct entries, remove, restore, add notes. Server infers intent from natural language.

**Request:**
```json
{
  "text": "bench press 135 for 8"
}
```
Or with audio:
```json
{
  "audio_base64": "base64-encoded-audio",
  "audio_format": "m4a"
}
```
- `text` and `audio_base64` are mutually exclusive; send one
- `audio_format` optional: `"m4a"`, `"webm"`, etc. Defaults to webm if omitted

**Response:** Varies by intent. All responses include `intent` and usually `message`.

**Log intent:**
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

**Query intent:**
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

**Correction intent:**
```json
{
  "intent": "correction",
  "message": "Corrected."
}
```

**Remove intent:**
```json
{
  "intent": "remove",
  "message": "Removed."
}
```

**Restore intent:**
```json
{
  "intent": "restore",
  "message": "Brought back."
}
```

**Note intent:**
```json
{
  "intent": "note",
  "message": "Noted."
}
```

**Unknown intent:**
```json
{
  "intent": "unknown",
  "message": "I didn't understand. Try logging a workout, asking about your history, correcting a previous entry, or removing something."
}
```

**Example phrases:**
- Log: "bench 135 for 8", "squats 185x5", "RDL 135 for 6"
- Query: "what's my last bench", "how much did I deadlift"
- Correction: "change that to 140", "that was 6 reps not 8"
- Remove: "forget that", "remove the last bench"
- Restore: "bring that back", "oh sorry undo"
- Note: "remember for RDLs: warm up hamstrings"

**Throttling:** Per-user rate limits. 429 when over limit. See `docs/ai-throttling.md`.

---

### GET /sessions

**Auth:** Required

**Purpose:** List workout sessions (timeline). Most recent first.

**Query params:**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `limit` | int | 50 | Max sessions (1–100) |

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
| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `category` or `exercise` | string | Yes | e.g. `"bench press"`, `"deadlift"` |
| `variant` | string | No | Default `"standard"` |
| `limit` | int | No | Default 20, max 50 |
| `from` | string | No | YYYY-MM-DD |
| `to` | string | No | YYYY-MM-DD |

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

**Purpose:** List all exercise categories and variants for the user (global + user-level).

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

**Purpose:** Redirect to presigned URL for PR image. Returns 302 when ready.

**Path:** `id` = PR UUID

**Response codes:**
| Code | Meaning |
|------|---------|
| 404 | Image not ready yet → keep polling |
| 302 | Image ready → follow redirect to load the image |

**On 302:** Redirect goes to R2 presigned URL (valid 1 hour).

**Errors:** 404 if PR not found, not owned by user, or image not ready (DALL-E still generating).

**Polling strategy:**
- Endpoint: `GET /prs/{id}/image` (with auth)
- Interval: 3–5 seconds
- Timeout: ~60 seconds (DALL-E usually ~30 sec)
- On 302: Follow redirect and load the image

**Alternative:** Poll `GET /prs` and watch for the PR's `image_url` to become non-null, then call `GET /prs/{id}/image` once. Useful if already refreshing the PR list; otherwise polling the image endpoint is simpler.

V2: FCM notification when ready (foreground = update UI; background/closed = system notification).

---

### GET /health

**Auth:** None

**Purpose:** Health check. Pings database.

**Response 200:**
```json
{"status":"ok"}
```

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

**Codes:** `unauthorized`, `invalid_input`, `not_found`, `internal_error`, `method_not_allowed`

**error_token:** Unique per error. Display for bug reports; developer searches logs by token.

---

## Content-Type

- **Request:** `Content-Type: application/json` for POST body
- **Response:** `Content-Type: application/json`

---

## Summary: What the Android Client Can Do

| Action | Endpoint | Notes |
|--------|----------|-------|
| Verify auth | GET /me | After Google Sign-In |
| Load chat history | GET /chat/messages | Initial: limit=6. Scroll up: before=id |
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
| PR image | GET /prs/{id}/image | 302 redirect |
| Health check | GET /health | No auth |
