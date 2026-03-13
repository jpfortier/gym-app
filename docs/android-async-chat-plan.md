# Android App — Async Chat Implementation Plan

Plan for implementing the async audio flow in the Android app. When the user sends voice, the backend returns transcribed text immediately; the LLM result arrives via polling.

---

## Current vs New Flow

| Input | Current | New |
|------|---------|-----|
| **Text** | POST /chat → wait → full response | Same (unchanged) |
| **Audio** | POST /chat → wait (Whisper + LLM) → full response | POST /chat → immediate `job_id` + `text` → poll GET /chat/jobs/{id} → full response |

---

## 1. Data Models

### ChatResponse (existing, for text and final result)
```kotlin
data class ChatResponse(
    val message: String?,
    val entries: List<LogEntry>?,
    val history: HistoryResult?,
    val prs: List<PRResult>?
)
```

### JobResponse (new, for async audio)
```kotlin
data class JobResponse(
    val job_id: String,
    val text: String,
    val status: String,  // "processing" | "complete" | "failed"
    val result: ChatResponse?,
    val error: String?
)
```

### Response discriminator
POST /chat can return either:
- **ChatResponse** (text input, or legacy sync) — has `message`, `entries`, etc.
- **JobResponse** (audio input) — has `job_id`, `text`, `status`

Use presence of `job_id` to distinguish. If `job_id` is present, treat as JobResponse; otherwise ChatResponse.

---

## 2. API Layer

### ChatApi / ChatRepository
```kotlin
// POST /chat
suspend fun postChat(text: String, audioBase64: String?, audioFormat: String?): Either<ChatResponse, JobResponse>
```

- If response has `job_id` → return `Right(JobResponse)` (async path)
- Otherwise → return `Left(ChatResponse)` (sync path)

### Polling
```kotlin
suspend fun getChatJob(jobId: String): JobResponse
```

- Call `GET /chat/jobs/{jobId}`
- Poll every 300–500 ms until `status` is `"complete"` or `"failed"`
- Max 30–60 s timeout
- Cancel polling when user navigates away or cancels

---

## 3. Use Case / ViewModel

### SendMessage (or ChatUseCase)
```kotlin
// Pseudocode
suspend fun sendMessage(text: String?, audio: ByteArray?, format: String?) {
    if (text != null) {
        // Sync path
        val result = api.postChat(text, null, null)
        emit(ChatResult.Success(result.left))
    } else if (audio != null) {
        val b64 = Base64.encodeToString(audio, Base64.NO_WRAP)
        val response = api.postChat("", b64, format ?: "m4a")
        when (response) {
            is Left -> emit(ChatResult.Success(response.value))
            is Right -> {
                val job = response.value
                emit(ChatResult.Transcribed(job.text))  // Show text immediately
                val final = pollUntilComplete(job.job_id)
                emit(ChatResult.Success(final.result))
            }
        }
    }
}
```

### State / sealed class
```kotlin
sealed class ChatResult {
    data class Transcribed(val text: String) : ChatResult()  // Show "You said: ..."
    data class Success(val response: ChatResponse) : ChatResult()
    data class Error(val message: String) : ChatResult()
}
```

---

## 4. UI

### Chat screen
1. **User sends voice** → Show recording indicator, then send.
2. **On Transcribed** → Immediately show user message bubble with `text` ("You said: bench press 135 for 8").
3. **Show loading** → Typing indicator or "Thinking..." for assistant.
4. **On Success** → Replace loading with assistant message (Markdown), entries, PRs, etc.
5. **On Error** → Show error toast/snackbar.

### Message list
- Append user message as soon as `Transcribed` is received.
- Append assistant placeholder (loading) while polling.
- Replace placeholder with full assistant message when `Success` arrives.

---

## 5. Polling Implementation

### Option A: Coroutine loop
```kotlin
suspend fun pollUntilComplete(jobId: String): JobResponse {
    while (true) {
        val job = api.getChatJob(jobId)
        when (job.status) {
            "complete" -> return job
            "failed" -> throw error(job.error)
        }
        delay(300)
    }
}
```

### Option B: Flow
```kotlin
fun pollJob(jobId: String): Flow<JobResponse> = flow {
    while (true) {
        val job = api.getChatJob(jobId)
        emit(job)
        if (job.status in listOf("complete", "failed")) break
        delay(300)
    }
}
```

### Cancellation
- Use `CoroutineScope` tied to ViewModel.
- Cancel when `onCleared()` or user navigates away.
- Avoid polling on a destroyed screen.

---

## 6. Error Handling

| Case | Handling |
|------|----------|
| 401 | Refresh token, retry once |
| 429 | Show "Rate limited. Try again in a minute." |
| Job failed | Show `job.error` to user |
| Poll timeout | Show "Request took too long. Try again." |
| Network error | Show "Connection error. Check network." |

---

## 7. Task Checklist

| # | Task | Success Criteria |
|---|------|------------------|
| 1 | Add `JobResponse` model | Parses `job_id`, `text`, `status`, `result`, `error` |
| 2 | Update POST /chat response handling | Detect `job_id` vs `message`; return correct type |
| 3 | Add `GET /chat/jobs/{id}` to API | Returns `JobResponse` |
| 4 | Implement polling | Poll until complete/failed; timeout 30s |
| 5 | Add `Transcribed` state | ViewModel emits when text received |
| 6 | Update UI for async flow | Show user message immediately; loading for assistant |
| 7 | Handle cancellation | Stop polling when leaving screen |
| 8 | Error handling | Show job failure, timeout, network errors |

---

## 8. Optional Enhancements

- **Retry** — On 5xx or network error during poll, retry 2–3 times before giving up.
- **Progress** — If backend adds `progress` field later, show "Transcribing..." vs "Processing...".
- **Offline** — Queue audio; send when back online (future).

---

## 9. Testing

- **Unit:** Mock API returns `JobResponse`; verify polling loop and state transitions.
- **Integration:** Use real backend; send audio, verify transcribed text appears, then full response.
- **UI:** Manual test: record voice → see text appear → see assistant reply.
