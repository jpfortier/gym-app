# Context-Aware Chat

## Overview

The chat parser is currently stateless: each message is parsed in isolation. That breaks natural follow-ups like "and another one for 150" or "change that to 6 reps" because the parser has no idea what "another one" or "that" refers to.

This document describes the plan to add conversation context so the parser receives recent user/assistant turns and can resolve references correctly.

## Why Context Matters

| User says | Without context | With context |
|-----------|-----------------|--------------|
| "140 bench press" | ✅ Parses fine | ✅ Same |
| "and another one for 150" | ❌ Unknown exercise, unknown "another one" | ✅ Infers: add another set of bench at 150 |
| "change that to 6 reps" | ❌ Change what? | ✅ Change last set to 6 reps |
| "actually remove that" | ❌ Remove what? | ✅ Remove the last logged entry |

## Approach: Sliding Window

Pass the **last N messages** (user + assistant) to the parser with each request. The LLM sees the conversation flow and can resolve "that", "another one", etc.

- **Window size:** 4–6 messages (2–3 user + 2–3 assistant). Configurable.
- **Retention:** Keep forever. No trimming for now.
- **Separate from ai_usage:** `ai_usage` stays for cost/billing. `chat_messages` is for conversation content only.

## What We Store

### chat_messages table

| Column | Type | Purpose |
|--------|------|---------|
| `id` | uuid PK | |
| `user_id` | uuid FK → users | |
| `role` | text | `"user"` or `"assistant"` |
| `content` | text | User: raw text (or transcribed). Assistant: short summary of action (e.g. "Logged bench press 140×8.") |
| `created_at` | timestamptz | |

**Assistant content format:** Human-readable summary of what we did. Examples:
- "Logged bench press 140×8."
- "Removed the last bench set."
- "Brought back bench press 140×8."
- "Here's your bench history: ..."

The LLM needs to know what action was taken so it can resolve "that" and "another one".

## Flow (with context)

1. User sends "and another one for 150"
2. Load last N messages for user from `chat_messages`
3. Build parse prompt:
   ```
   [system: parse user intent...]

   User: bench press 135 for 8
   Assistant: Logged bench press 135×8.

   User: and another set at 140
   Assistant: Logged bench press 140×8.

   User: and another one for 150
   ```
4. LLM parses with context → intent: log, exercise: bench press, add set 150×?
5. Execute (create log entry)
6. Store user message + assistant response in `chat_messages`
7. Return response to client

## Implementation (done)

### 1. Migration: chat_messages table

```sql
CREATE TABLE chat_messages (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role text NOT NULL CHECK (role IN ('user', 'assistant')),
  content text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_chat_messages_user_created ON chat_messages(user_id, created_at DESC);
```

### 2. Chat messages repo

- `internal/chatmessages/` or `internal/chat/repo.go`
- `ListRecent(ctx, userID, limit int) ([]Message, error)` — last N messages, oldest first (for prompt ordering)
- `Append(ctx, userID, role, content) error` — insert after each turn

### 3. Chat service changes

- Inject chat messages repo into `chat.Service`
- Before `parser.Parse()`: load last N messages, build context
- Pass context to parser (new param or extend Parse to accept messages)
- After handling intent: append user message + assistant summary to `chat_messages`

### 4. Parser changes

- `Parse(ctx, userID, text string, recentMessages []ChatMessage) (*ParsedIntent, error)`
- Build messages array: `[system, ...recentMessages, user: text]`
- Update parse prompt to mention: "You may receive recent conversation. Use it to resolve 'that', 'another one', etc."

### 5. Assistant summary generation

- Each intent handler returns a response. We need a short summary for storage.
- Options: (a) derive from Response (e.g. "Logged." + entry details), (b) have handlers return a `summary` field, (c) build from intent + result in chat service.
- Simplest: chat service builds summary from Response (Intent, Message, Entries, etc.)

### 6. Tests

- Test with `OPENAI_TEST_MODE`: mock returns messages; chat service still stores/retrieves.
- Integration test: "bench 135" → "and 140" → verify second parse gets context and logs correctly.
- Test that Append happens after successful processing.

## Examples (for prompt design)

```
User: bench press 135 for 8
Assistant: Logged bench press 135×8.

User: and another set at 140
Assistant: Logged bench press 140×8.

User: make that 6 reps
→ Parser infers: correction, target last set, change reps to 6
```

```
User: squats 185 for 5
Assistant: Logged squat 185×5.

User: and bench 140 for 8
Assistant: Logged bench press 140×8.

User: remove that
→ Parser infers: remove, target last (bench 140×8)
```

## Open Questions (resolved)

- **Retention:** Keep forever. No trimming for now.
- **Window size:** Start with 6 messages. Tune if needed.
- **Session boundary:** Use last N messages regardless of date. Can add session-scoping later if needed.
