# Database Design

## Overview

Postgres on Fly. Migrations via golang-migrate.

**Embeddings:** Store vectors for exercise categories and variants (pgvector or separate table). Cosine similarity for semantic matching.

## Tables (Planned)

### users
- `id` uuid PK
- `google_id` text unique — from Google Sign-In
- `email` text
- `name` text
- `photo_url` text (optional)
- `role` text — 'user' | 'coach' | 'owner' | 'admin'. Default 'user'. Extensible for future roles.
- `created_at` timestamptz

### exercise_categories
Top-level exercises. **Global list** (seeded) + **user-level** (user's own).
- `id` uuid PK
- `user_id` uuid FK → users (nullable) — null = global, set = user's custom
- `name` text — e.g. "bench press", "deadlift"
- `show_weight` boolean — include weight in UI (default true)
- `show_reps` boolean — include reps in UI (default true)
- `created_at` timestamptz
- Unique: (user_id, name) — global: user_id null; user: per-user unique names

**Global list:** Seeded with common exercises (Bench Press, Deadlift, Squat, Shoulder Press, Row, etc.) as beginner entry. Users see these by default.

**User list:** User can add their own categories (e.g. "hula hoop overhead"). Resolved after global.

### exercise_variants
One per category. **Variants are flexible**—AI creates new ones as needed.
- `id` uuid PK
- `category_id` uuid FK → exercise_categories
- `user_id` uuid FK → users (nullable) — null = global variant, set = user's variant
- `name` text — e.g. "close grip", "RDL", "standard", "Dorian"
- `created_at` timestamptz
- Unique: (category_id, user_id, name)

**Resolution order:** For a user, check global categories + user's categories. For variants: global variants for that category, then user's variants. AI can create user-level variants when it doesn't find a match.

### Global seed list (starter categories)
To be seeded in migration. Examples: Bench Press, Deadlift, Squat, Overhead Press, Row, Pull-up, Push-up, Lunge, Calf Raise, Curl, Tricep Extension, etc.

### Embeddings (exercise resolution)
- `embedding` vector(1536) on exercise_categories and exercise_variants (pgvector)
- Or: separate `exercise_embeddings` table (entity_id, entity_type, embedding)
- Generate via OpenAI embeddings API when creating/updating exercises

### user_exercise_aliases
Learned mappings from user input to resolved variant. When we resolve via embedding or create a new variant, we store the alias so future lookups skip the LLM.
- `id` uuid PK
- `user_id` uuid FK → users (ON DELETE CASCADE)
- `alias_key` text — normalized input, e.g. `"rdl standard"`
- `variant_id` uuid FK → exercise_variants (ON DELETE CASCADE)
- `created_at` timestamptz
- Unique: (user_id, alias_key)

**Resolution order:** Exact match → alias lookup → embedding match → create new. Store alias when resolving via embedding or create.

### workout_sessions
One per user per day (or explicit session).
- `id` uuid PK
- `user_id` uuid FK → users
- `date` date — workout date
- `created_at` timestamptz

### log_entries
**Block** — one row per exercise in a session (e.g. "bench press" for today). Soft delete via `disabled_at`.
- `id` uuid PK
- `session_id` uuid FK → workout_sessions
- `exercise_variant_id` uuid FK → exercise_variants
- `raw_speech` text — exact text the user said for this exercise block (e.g. "back squat 105", "100 pushups"). Per-exercise segment, not the full paragraph. Enables reprocessing if parsing improves.
- `notes` text
- `disabled_at` timestamptz — soft delete (set when user says "remove", "forget that", etc.)
- `created_at` timestamptz

### log_entry_sets
**Sets** — one row per set within a block. Supports ramp/pyramid (140×8, 150×4, 160×2, 170×1).
- `id` uuid PK
- `log_entry_id` uuid FK → log_entries
- `weight` decimal — pounds (nullable for bodyweight exercises)
- `reps` int
- `set_order` int — 1, 2, 3...
- `set_type` text — warm-up, working, drop, etc. (optional)
- `created_at` timestamptz

*Extensible (optional):* effort/RPE, rest_time, side, tempo.

**set_type:** Free-form text. Examples from real routines: "For Weight", "Speed Pull / For Weight", "CAT 6S / For Weight", "Heavy Pull / For Weight", "PUMP / For Weight".

### Sample routines → stored JSON

Reference: `docs/notes.md` "Sample Routines (real)".

**1. Close Grip Bench Press — 4 × 8 — For Weight**
```json
{
  "exercise_name": "Bench Press",
  "variant_name": "close grip",
  "notes": null,
  "sets": [
    { "set_order": 1, "weight": 135, "reps": 8, "set_type": "For Weight" },
    { "set_order": 2, "weight": 135, "reps": 8, "set_type": "For Weight" },
    { "set_order": 3, "weight": 135, "reps": 8, "set_type": "For Weight" },
    { "set_order": 4, "weight": 135, "reps": 8, "set_type": "For Weight" }
  ]
}
```

**2. Deadlift — 6 × 3 — Speed Pull / For Weight**
```json
{
  "exercise_name": "Deadlift",
  "variant_name": "standard",
  "notes": null,
  "sets": [
    { "set_order": 1, "weight": 225, "reps": 3, "set_type": "Speed Pull / For Weight" },
    { "set_order": 2, "weight": 225, "reps": 3, "set_type": "Speed Pull / For Weight" },
    { "set_order": 3, "weight": 275, "reps": 3, "set_type": "Speed Pull / For Weight" },
    { "set_order": 4, "weight": 275, "reps": 3, "set_type": "Speed Pull / For Weight" },
    { "set_order": 5, "weight": 315, "reps": 3, "set_type": "Speed Pull / For Weight" },
    { "set_order": 6, "weight": 315, "reps": 3, "set_type": "Speed Pull / For Weight" }
  ]
}
```

**3. Back Squat — 5 × 6 — CAT 6S / For Weight**
```json
{
  "exercise_name": "Squat",
  "variant_name": "back squat",
  "notes": null,
  "sets": [
    { "set_order": 1, "weight": 185, "reps": 6, "set_type": "CAT 6S / For Weight" },
    { "set_order": 2, "weight": 185, "reps": 6, "set_type": "CAT 6S / For Weight" },
    { "set_order": 3, "weight": 185, "reps": 6, "set_type": "CAT 6S / For Weight" },
    { "set_order": 4, "weight": 185, "reps": 6, "set_type": "CAT 6S / For Weight" },
    { "set_order": 5, "weight": 185, "reps": 6, "set_type": "CAT 6S / For Weight" }
  ]
}
```

**4. Close Grip Bench Press — 5 × 6 — CAT 6S / For Weight**
```json
{
  "exercise_name": "Bench Press",
  "variant_name": "close grip",
  "notes": null,
  "sets": [
    { "set_order": 1, "weight": 115, "reps": 6, "set_type": "CAT 6S / For Weight" },
    { "set_order": 2, "weight": 115, "reps": 6, "set_type": "CAT 6S / For Weight" },
    { "set_order": 3, "weight": 115, "reps": 6, "set_type": "CAT 6S / For Weight" },
    { "set_order": 4, "weight": 115, "reps": 6, "set_type": "CAT 6S / For Weight" },
    { "set_order": 5, "weight": 115, "reps": 6, "set_type": "CAT 6S / For Weight" }
  ]
}
```

**5. Barbell RDL — 4 × 6 — Heavy Pull / For Weight**
```json
{
  "exercise_name": "Deadlift",
  "variant_name": "barbell RDL",
  "notes": null,
  "sets": [
    { "set_order": 1, "weight": 185, "reps": 6, "set_type": "Heavy Pull / For Weight" },
    { "set_order": 2, "weight": 185, "reps": 6, "set_type": "Heavy Pull / For Weight" },
    { "set_order": 3, "weight": 185, "reps": 6, "set_type": "Heavy Pull / For Weight" },
    { "set_order": 4, "weight": 185, "reps": 6, "set_type": "Heavy Pull / For Weight" }
  ]
}
```

**6. Heel Elevated Back Squats — 8, 6, 4, MAX — PUMP / For Weight**

Pyramid: different reps per set. MAX = 1 rep.
```json
{
  "exercise_name": "Squat",
  "variant_name": "heel elevated back squat",
  "notes": null,
  "sets": [
    { "set_order": 1, "weight": 135, "reps": 8, "set_type": "PUMP / For Weight" },
    { "set_order": 2, "weight": 155, "reps": 6, "set_type": "PUMP / For Weight" },
    { "set_order": 3, "weight": 175, "reps": 4, "set_type": "PUMP / For Weight" },
    { "set_order": 4, "weight": 195, "reps": 1, "set_type": "PUMP / For Weight" }
  ]
}
```

### notes
User notes. Global or scoped to category/variant.
- `id` uuid PK
- `user_id` uuid FK → users
- `exercise_category_id` uuid FK (nullable) — null = global
- `exercise_variant_id` uuid FK (nullable) — null = applies to whole category
- `content` text
- `created_at` timestamptz

### personal_records
- `id` uuid PK
- `user_id` uuid FK → users
- `exercise_variant_id` uuid FK → exercise_variants
- `pr_type` text — "natural_set" | "one_rep_max" | ...
- `weight` decimal
- `reps` int (for natural set)
- `log_entry_set_id` uuid FK → log_entry_sets (source set)
- `image_url` text — DALL-E generated
- `created_at` timestamptz

### ai_usage
Token tracking for admin dashboard.
- `id` uuid PK
- `user_id` uuid FK → users
- `model` text — gpt-4o, whisper, dall-e-3
- `prompt_tokens` int
- `completion_tokens` int
- `estimated_cost_cents` decimal
- `created_at` timestamptz

## Indexes (To Add)

- `user_exercise_aliases(user_id, alias_key)` — for alias lookup (created in migration 000009)
- `log_entries(session_id)`
- `log_entries(exercise_variant_id, created_at)` — for history queries
- `log_entry_sets(log_entry_id)`
- `workout_sessions(user_id, date)`
- `personal_records(user_id, exercise_variant_id, pr_type)`
- `ai_usage(user_id, created_at)`

## Storage

- **Provider:** Cloudflare R2 (bucket `gym-app`). S3-compatible API.
- **PR images:** `pr/{user_id}/{pr_id}.png`. DALL-E generates; background job uploads. `personal_records.image_url` stores the key.
- **User profile photos:** R2. `users.photo_url` stores the key. Upload via backend proxy.
- **Access:** Private bucket. Backend generates presigned URLs for downloads. Sharing = presigned URL with longer expiry.
- **Future (not v1):** Workout list photos, routine screenshots (e.g. from other apps).
- **APK hosting:** S3 or R2 for distribution. In-app update flow checks for new version, downloads, prompts install.
