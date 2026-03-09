# Workout Logger – Changes from Architecture Proposal

This document compares the **LLM + Rule Engine Architecture** proposal with the current implementation and outlines the changes required to align the codebase.

---

## 1. Pipeline

**Proposed:**
```
User message → Build context → LLM interprets → Build assumption ledger → Validate → Decide execution policy → Confirm or execute → Return result → Log audit event
```

**Current:**
```
User message → Load recent chat messages → LLM parses intent → Execute immediately → Return result → Append to chat_messages
```

**Changes:**
- Add explicit **context building** step before LLM call (structured workout context, not just chat history)
- Add **assumption ledger** construction from LLM output
- Add **validation** layer between interpretation and execution
- Add **execution policy** (confirm vs auto-execute)
- Add **audit logging** for every request
- Optional confirmation flow before destructive/ambiguous actions

---

## 2. Domain Model

### WorkoutSession

| Field | Proposed | Current |
|-------|----------|---------|
| id | ✓ | ✓ |
| date | ✓ | ✓ |
| notes | ✓ | ✗ |
| location | ✓ | ✗ |
| tags | ✓ | ✗ |

**Changes:** Add `notes`, `location`, `tags` to `workout_sessions` (migration).

### ExerciseEntry (log_entries)

| Field | Proposed | Current |
|-------|----------|---------|
| id | ✓ | ✓ |
| session_id | ✓ | ✓ |
| canonical_exercise_name | ✓ | Uses `exercise_variant_id` (resolved) |
| variant | ✓ | ✓ (via variant) |
| order | ✓ | Implicit via `created_at` |
| notes | ✓ | ✓ |

**Changes:** Consider adding `order` column for explicit exercise ordering within a session. Current model is adequate; naming differs (ExerciseEntry vs LogEntry).

### SetEntry (log_entry_sets)

| Field | Proposed | Current |
|-------|----------|---------|
| id | ✓ | ✓ |
| exercise_entry_id | ✓ | `log_entry_id` ✓ |
| reps | ✓ | ✓ (NOT NULL) |
| weight | ✓ | ✓ (nullable for bodyweight) |
| unit | ✓ | ✗ |
| rir/rpe | ✓ | ✗ |
| tempo | ✓ | ✗ |

**Changes:**
- Add `unit` column (lb/kg) — currently assumed lb
- Add `rir` or `rpe` (optional)
- Add `tempo` (optional)
- Allow **partially specified sets**: `reps` should be nullable when only weight is logged (e.g. "bench 140" without reps)

---

## 3. LLM Responsibilities

### Proposed LLM Must:
- Classify intent ✓ (current: log, query, correction, remove, restore, note)
- Normalize exercise names ✓ (current: via ResolveOrCreate)
- Extract reps/weight/set structures ✓ (current: ParsedExercise, ParsedSet)
- Resolve conversational references ✓ (current: via recentMessages)
- Identify ambiguities ✗
- List assumptions/defaults applied ✗
- Produce structured commands ✗ (current: intent + ad-hoc fields)
- Optionally produce preview/confirmation text ✗

### Proposed LLM Must NOT:
- Execute logic ✓
- Choose confirmation policy ✓
- Access database ✓
- Expand search scope beyond provided context ✓

**Changes:**
- Extend LLM output to include `assumptions`, `ambiguities`, `ui_text.preview`, `ui_text.confirmation`
- Replace intent-based parsing with **structured command DSL** output
- LLM outputs commands (e.g. `APPEND_SET`, `UPDATE_SET`) instead of intent strings

---

## 4. Workout Context Sent to LLM

**Proposed:** Compact context object with:
- `today`
- `REFERENCE_OBJECTS`: last_created_set, last_exercise, last_session
- `active_session`: id, date, exercises with sets (ids)
- `recent_sessions`: last 7 days summary
- `recent_actions`: e.g. APPEND_SET, set_id
- `exercise_aliases`: bench → barbell bench press, etc.
- `user_defaults`: weight_unit

**Current:** Only `recentMessages` (user/assistant chat turns). No structured workout state.

**Changes:**
- Build **workout context** before each LLM call:
  - Active session (today’s session with exercises and sets)
  - Recent sessions (last 7 days, compact)
  - Reference objects (last set, last exercise, last session)
  - User exercise aliases from `user_exercise_aliases`
  - User defaults (weight unit)
- Pass this context to the LLM in a structured format (e.g. YAML or JSON in system/user message)

---

## 5. Workout Command DSL

**Proposed commands:**
- `ENSURE_SESSION`
- `CREATE_EXERCISE_ENTRY`
- `APPEND_SET`
- `UPDATE_SET`
- `DELETE_SET`
- `QUERY_HISTORY`

**Current:** Intent-based. No DSL.
- Log → creates session + entry + all sets (no APPEND_SET to existing exercise)
- Correction → updates first set of most recent entry
- Remove → disables entire entry (no DELETE_SET for single set)

**Changes:**
- Define command types and JSON schema for LLM output
- Implement `APPEND_SET`: add a set to existing exercise in active session
- Implement `UPDATE_SET` with `targetRef` (e.g. "last_created_set")
- Implement `DELETE_SET` (delete single set) in addition to entry-level remove
- Support incremental logging: "bench" → "140 for 8" → "145 for 6" as APPEND_SET flow

---

## 6. Workout Query DSL

**Proposed:** `op`, `exercise`, `date_ref`, `scope`, `metric`, `limit`

**Metrics:** max_weight, latest_weight, max_reps, count_sets, count_sessions, total_volume, estimated_1rm

**Scopes:** most_recent, recent, best, aggregate, session_detail, trend

**Current:** Only `History(category, variant, fromDate, toDate, limit)`. No metrics, no scopes.

**Changes:**
- Extend query service to support:
  - `scope`: most_recent, recent, best, aggregate, session_detail, trend
  - `metric`: max_weight, latest_weight, max_reps, count_sets, count_sessions, total_volume, estimated_1rm
- Add query execution for "What's my best bench?" → max_weight
- Add query execution for "Show my last three squats" → scope=recent, limit=3
- Implement `estimated_1rm` (e.g. Epley formula)

---

## 7. Assumption Ledger

**Proposed:** Record explicit vs inferred facts.
```json
{
  "explicitFacts": { "exercise": "bench", "weight": 140 },
  "inferredFacts": { "date": "today", "unit": "lb" },
  "ambiguities": []
}
```

**Current:** None.

**Changes:**
- LLM outputs assumptions as part of response
- Build assumption ledger from LLM output
- Use for debugging and confirmation policy (e.g. require confirm when many inferred facts)

---

## 8. Validation

**Proposed:** Deterministic validation with:
- Exercise exists or is creatable
- Target references resolve
- Command structure valid
- Units valid
- Date references resolvable
- Edit/delete targets unique

Output: `resolvedTargets`, `issues`

**Current:** Basic checks (exercise resolve, entry existence). No structured validation output.

**Changes:**
- Add validation layer between LLM output and execution
- Validate command structure, target refs, units, dates
- Return `resolvedTargets` and `issues` for confirmation/error handling

---

## 9. Confirmation Policy

**Proposed:** Code-owned rules.
- **Auto-execute:** append/create, target unique, date today, non-destructive
- **Require confirmation:** delete, multiple targets, editing older sessions, ambiguous references

**Current:** No confirmation. Always executes immediately.

**Changes:**
- Implement confirmation policy in code
- When confirmation required: return `needs_confirmation: true` with `preview` text and pending command
- Add API for "confirm" action (e.g. POST with confirmation token or command replay)
- For ambiguous targets: "Do you want to update today's back squat or yesterday's?"

---

## 10. Scope Expansion Policy

**Proposed:** Deterministic fallback for edits:
1. Active session
2. Today
3. Yesterday
4. Most recent matching exercise in last 7 days

**Current:** Correction uses most recent entry (any date). Remove uses today or most recent. No explicit fallback order.

**Changes:**
- Implement scope expansion as deterministic code
- Use fallback order when target not found in primary scope
- LLM does not expand scope; code does

---

## 11. Error Repair

**Proposed:** On `TARGET_NOT_FOUND`, optionally suggest repair (e.g. yesterday’s squat). For edits/deletes, confirmation if scope changes.

**Current:** Returns error or "Couldn't find that exercise."

**Changes:**
- On validation failure, attempt scope expansion
- If repair finds target, may require confirmation before executing

---

## 12. LLM Response Structure

**Proposed:**
```json
{
  "intent": "UPDATE_SET",
  "commands": [{ "type": "UPDATE_SET", "targetRef": "last_created_set", "changes": { "weight": 205 } }],
  "assumptions": [{ "kind": "target_inferred", "value": "last_created_set" }],
  "ambiguities": [],
  "ui_text": { "preview": "Update your most recent set to 205 lb.", "confirmation": null }
}
```

**Current:** `ParsedIntent` with intent, exercises, changes, etc. No assumptions, ambiguities, or ui_text.

**Changes:**
- Redesign `ParsedIntent` / LLM output schema to match
- Add `commands` array (DSL)
- Add `assumptions`, `ambiguities`, `ui_text`

---

## 13. Success Messages

**Proposed:** Generated by code from execution results. Examples:
- "Logged bench press — 140 lb for today."
- "Updated your last set to 205 lb."

**Current:** Generic ("Logged.", "Corrected.", "Removed.") or PR-enhanced ("Logged. 2 new PR(s)!").

**Changes:**
- Generate success messages from actual execution state
- Use specific wording: "Logged bench press — 140 lb for today.", "Updated your last set to 205 lb."

---

## 14. Query Flow

**Proposed:** Queries do not require confirmation. Flow: interpret → validate → execute → return formatted results.

**Current:** Same. Queries execute immediately.

**Changes:** Ensure query path stays confirmation-free. Add richer query result formatting per metric/scope.

---

## 15. Audit Logging

**Proposed:** Every request logs:
- User message
- Context snapshot
- Interpretation
- Assumption ledger
- Validation result
- Execution decision
- Execution result
- Timestamps

**Current:** No audit log. Chat messages stored for context only.

**Changes:**
- Add `audit_log` or similar table
- Log full request lifecycle for each chat/command
- Include context, interpretation, validation, decision, result

---

## 16. LLM Call Count

**Proposed:** One LLM call per request (interpretation + preview). Optional second for semantic repair.

**Current:** One call per request. No repair flow.

**Changes:** Keep single-call default. Add optional repair path when validation fails with recoverable error.

---

## 17. Common User Interaction Patterns

**Proposed patterns:**
- One-shot: "bench 140"
- Incremental: "bench" → "140 for 8" → "145 for 6"
- Correction: "actually 205"
- Recent edit: "change the last squat to 205"
- History query: "what was my last deadlift"

**Current support:**
- One-shot ✓
- Incremental: Partial. "and another one for 150" requires new log entry (creates new exercise block). No APPEND_SET to same exercise.
- Correction ✓ (but updates first set only, not "last" set)
- Recent edit: Partial. Correction finds most recent entry; no "last set" targeting.
- History query ✓

**Changes:**
- Implement APPEND_SET for incremental logging to same exercise
- Support "last set" / "last_created_set" in correction (not just first set of entry)
- Add UPDATE_SET with targetRef for "change the last squat to 205"

---

## 18. Summary: Priority Order

| Priority | Area | Effort | Impact |
|----------|------|--------|--------|
| 1 | Workout context to LLM | Medium | High – enables better interpretation |
| 2 | Command DSL (APPEND_SET, UPDATE_SET with targetRef) | Medium | High – incremental logging, set-level edits |
| 3 | Partially specified sets (reps nullable) | Low | Medium |
| 4 | Query DSL (metrics, scopes) | Medium | Medium |
| 5 | Validation layer | Medium | High – safety, confirmation |
| 6 | Confirmation policy + API | Medium | Medium |
| 7 | Assumption ledger + LLM output | Low | Medium |
| 8 | Audit logging | Low | Medium |
| 9 | Session notes/location/tags | Low | Low |
| 10 | Set unit, rir, tempo | Low | Low |

---

## 19. What Already Aligns

- **Auth** (Google Sign-In)
- **Session + Log entry + Set** model (core structure)
- **Exercise resolution** (exact, alias, embedding, create)
- **User exercise aliases** (migration 000009)
- **Chat context** (recent messages for reference resolution)
- **Intent handling** (log, query, correction, remove, restore, note)
- **PR detection** and image generation
- **Bodyweight** support (nullable weight)
- **Single LLM call** per request
- **Query execution** without confirmation
