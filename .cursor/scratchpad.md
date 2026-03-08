# Gym App Scratchpad

## Current Status

- [x] Migrations created and run (000001_init, 000002_seed_categories)
- [x] Auth - Google Sign-In verification
- [ ] Session repo + service
- [ ] Log entry repo + service
- [ ] Exercise repo + basic resolution
- [ ] GET /sessions, GET /sessions/:id
- [ ] Query service, GET /exercises, GET /prs
- [ ] AI layer (POST /chat, Whisper, parse, etc.)

## Next

Session repo + service.

## Recent decisions (documented in docs/)

- Migration 000006: users.role. Run `make migrate-up` when DB is available.

- R2 for PR images, user photos. Path `pr/{user_id}/{pr_id}.png`. Backend proxy uploads only; presigned URLs for download.
- FCM for notifications. PR image ready: foreground = update UI (no notification); background/closed = system notification.

## Auth (completed)

- `internal/auth/verify.go` — VerifyGoogleToken using google.golang.org/api/idtoken
- `internal/auth/middleware.go` — RequireAuth middleware, get-or-create user, UserFromContext
- `internal/user/` — User struct, Repo (GetByGoogleID, Create)
- `GET /me` — protected endpoint returning current user (for testing auth)
- Env: GOOGLE_CLIENT_ID (OAuth2 client ID), DATABASE_URL
