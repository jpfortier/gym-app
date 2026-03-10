# Admin Panel Plan

Consolidated plan for the Gym app admin panel. (V2, alongside FCM and notifications.)

---

## Purpose

Web admin panel for backend administration. **Not for workout logging**—the Android app handles that. Admin is for viewing data, managing users, tracking AI costs, and administrative tasks.

---

## Requirements

- View data **globally** and **as a particular user** (other user view)
- Create/delete users
- AI token usage tracking (cost tracking, view globally and per user)
- Administrative tasks
- Bare bones UI—no full user profile or fluff

---

## Tech Stack

| Layer | Choice |
|-------|--------|
| **Frontend** | Alpine.js + Go templates |
| **Backend** | Same Go backend as API |
| **Rendering** | Server-rendered HTML |
| **Interactivity** | Alpine.js |
| **Scope** | Dashboard (higher-level views) + raw table CRUD |

---

## Auth & Roles

- **Same auth** as main app: Google Sign-In. No separate login.
- **Role check:** Middleware checks `users.role` for admin routes. Only users with `role = 'admin'` can access.
- **Roles:** `'user' | 'coach' | 'owner' | 'admin'`. No boolean flags. One user (you) has `'admin'`.
- **Set admin manually:** `UPDATE users SET role = 'admin' WHERE email = 'your@email.com';`

---

## API Endpoints

Admin routes use different middleware (require `role = 'admin'`). Examples:

| Method | Endpoint | Purpose |
|--------|----------|---------|
| `GET` | `/admin/users` | List users |
| `GET` | `/admin/usage` | AI token usage (global or per user) |
| `DELETE` | `/admin/users/:id` | Delete user |

Token usage is persisted to `ai_usage` per user (Chat, Transcribe, Embed, DALL-E). Expose via admin endpoint for dashboard.

---

## Database

- **users.role** — `'user' | 'coach' | 'owner' | 'admin'`. Default `'user'`. Migration 000006.
- **ai_usage** — Token tracking for admin dashboard. Fields: `user_id`, `model`, `prompt_tokens`, `completion_tokens`, `estimated_cost_cents`, `created_at`.

---

## Implementation Notes

- Build segment by segment. Each segment gets a test before moving on.
- Admin panel is V2; FCM and notifications are also V2.
