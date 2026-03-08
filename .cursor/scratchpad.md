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

V2: FCM, notifications, admin panel.

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

**V2:** FCM, notifications, admin panel

## Development workflow (Executor)

Segment by segment. For each segment: implement → test → verify. Then integration test with previous segment. Shared setup (login once), cleanup after each test.

## Recent decisions (documented in docs/)

- Migration 000006: users.role. Run `make migrate-up` when DB is available.

- R2 for PR images, user photos. Path `pr/{user_id}/{pr_id}.png`. Backend proxy uploads only; presigned URLs for download.
- FCM for notifications. PR image ready: foreground = update UI (no notification); background/closed = system notification.

## Executor progress

Segments 1–6 done: Session repo+service, Log entry repo+service, Exercise repo+resolution. All tests pass. `make test` sources .env for DATABASE_URL.

## Auth (completed)

- `internal/auth/verify.go` — VerifyGoogleToken using google.golang.org/api/idtoken
- `internal/auth/middleware.go` — RequireAuth middleware, get-or-create user, UserFromContext
- `internal/user/` — User struct, Repo (GetByGoogleID, Create)
- `GET /me` — protected endpoint returning current user (for testing auth)
- Env: GOOGLE_CLIENT_ID (OAuth2 client ID), DATABASE_URL
