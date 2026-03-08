# Maintenance & Tech Debt

Items to improve for long-term maintainability.

## 1. dbForTest duplicated in 5 packages ✓

Fixed: Extracted to `internal/testutil.DBForTest`. All packages now use it via a thin wrapper.

## 2. Chat service constructor has 14 parameters ✓

Fixed: `chat.Config` struct. `chat.NewService(cfg Config)`.

## 3. Handler test setup is repetitive ✓

Fixed: `chatTestService` and `chatTestServer` helpers in handler package.

## 4. Note intent: prompt vs struct mismatch ✓

Fixed: Parse prompt now uses `note_content` to match the struct.

## 5. main.go is a single large wiring block ✓

Fixed: `cmd/api/server.go` with `Server` struct, `NewServer(ctx)`, `Run()`. main() is minimal.

## 6. R2 error handling ✓

Fixed: Log R2 init failure when err != nil.

## 7. Docs reference old env var names ✓

Fixed: `docs/README.md` and `docs/chat-context.md` now use `GYM_OPENAI_TEST_MODE`.
