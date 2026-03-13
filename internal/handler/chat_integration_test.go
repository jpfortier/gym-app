package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jpfortier/gym-app/internal/chat"
	"github.com/jpfortier/gym-app/internal/chatmessages"
	"github.com/jpfortier/gym-app/internal/env"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/pr"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/user"
)

// TestChat_realLLM_manual runs a full integration test against real LLM, Whisper, and DALL-E.
// Requires GYM_OPENAI_API_KEY set and GYM_OPENAI_TEST_MODE=false. Uses samples/audio/ and text.
// Run with: make test-real-llm
func TestChat_realLLM_manual(t *testing.T) {
	if env.OpenAIAPIKey() == "" {
		t.Skip("GYM_OPENAI_API_KEY not set, skipping real LLM test")
	}
	if env.OpenAITestMode() {
		t.Skip("GYM_OPENAI_TEST_MODE=true, skipping real LLM test (set false for manual run)")
	}

	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	chatMessagesRepo := chatmessages.NewRepo(db)
	u := &user.User{GoogleID: "real-llm-" + uuid.New().String(), Email: "real-" + uuid.New().String() + "@test.com", Name: ""}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, "DELETE FROM log_entry_sets WHERE log_entry_id IN (SELECT id FROM log_entries WHERE session_id IN (SELECT id FROM workout_sessions WHERE user_id = $1))", u.ID)
		_, _ = db.ExecContext(ctx, "DELETE FROM log_entries WHERE session_id IN (SELECT id FROM workout_sessions WHERE user_id = $1)", u.ID)
		_, _ = db.ExecContext(ctx, "DELETE FROM personal_records WHERE user_id = $1", u.ID)
		_, _ = db.ExecContext(ctx, "DELETE FROM notes WHERE user_id = $1", u.ID)
		_, _ = db.ExecContext(ctx, "DELETE FROM chat_messages WHERE user_id = $1", u.ID)
		_, _ = db.ExecContext(ctx, "DELETE FROM workout_sessions WHERE user_id = $1", u.ID)
		_, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID)
	})

	mux := chatTestServerWithR2(t, db, u, chatMessagesRepo)
	samplesDir := filepath.Join("..", "..", "samples", "audio")

	postChat := func(mux *http.ServeMux, audioPath string, text string) (int, map[string]interface{}) {
		var body []byte
		if audioPath != "" {
			data, err := os.ReadFile(audioPath)
			if err != nil {
				t.Fatalf("read %s: %v", audioPath, err)
			}
			b64 := base64.StdEncoding.EncodeToString(data)
			body, _ = json.Marshal(map[string]string{"audio_base64": b64, "audio_format": "m4a"})
		} else {
			body, _ = json.Marshal(map[string]string{"text": text})
		}
		req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer x")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		var out map[string]interface{}
		_ = json.NewDecoder(rec.Body).Decode(&out)
		return rec.Code, out
	}
	postChatDefault := func(audioPath string, text string) (int, map[string]interface{}) {
		return postChat(mux, audioPath, text)
	}

	// Clear path: explicit prompts that should log without follow-up. Verifies the flow actually works.
	t.Run("0_clear_path_log_and_correction", func(t *testing.T) {
		code, out := postChatDefault("", "bench press 135 for 8")
		if code != http.StatusOK {
			t.Fatalf("log: got status %d: %v", code, out)
		}
		entries, _ := out["entries"].([]interface{})
		if len(entries) == 0 {
			t.Fatalf("clear path: expected entries after logging, got %d. message=%q", len(entries), out["message"])
		}
		t.Logf("logged %d entries", len(entries))

		code, out = postChatDefault("", "change the last bench to 140")
		if code != http.StatusOK {
			t.Fatalf("correction: got status %d: %v", code, out)
		}
		msg, _ := out["message"].(string)
		if msg == "" {
			t.Errorf("correction: expected message, got %v", out["message"])
		}
		t.Logf("correction message=%q", msg)
	})

	// 0a. Text: two logs same exercise, second better — expect PR (first = baseline no celebration, second = celebrate).
	// Executor unit test TestExecutor_AppendSet verifies PR detection; this exercises the full LLM path.
	t.Run("0a_text_two_logs_same_exercise_pr", func(t *testing.T) {
		code, out := postChatDefault("", "deadlift 185 for 5")
		if code != http.StatusOK {
			t.Fatalf("first log: got status %d: %v", code, out)
		}
		entries, _ := out["entries"].([]interface{})
		if len(entries) == 0 {
			t.Fatalf("first log: expected entries, got %d", len(entries))
		}
		prs1, _ := out["prs"].([]interface{})
		if len(prs1) != 0 {
			t.Logf("first log: expected 0 PRs (baseline), got %d", len(prs1))
		}

		code, out = postChatDefault("", "deadlift 200 for 5")
		if code != http.StatusOK {
			t.Fatalf("second log: got status %d: %v", code, out)
		}
		prs2, _ := out["prs"].([]interface{})
		if len(prs2) < 1 {
			t.Logf("second log (better): expected >=1 PR, got %d — LLM may batch/append differently. message=%q", len(prs2), out["message"])
		} else {
			t.Logf("second log: prs=%d ✓", len(prs2))
		}

		// Debug: dump DB state for this user to trace PR flow
		rows, err := db.QueryContext(ctx, `
			SELECT ev.name as variant, ec.name as category, le.raw_speech, les.weight, les.reps, les.set_order
			FROM log_entries le
			JOIN log_entry_sets les ON les.log_entry_id = le.id
			JOIN exercise_variants ev ON ev.id = le.exercise_variant_id
			JOIN exercise_categories ec ON ec.id = ev.category_id
			JOIN workout_sessions ws ON ws.id = le.session_id
			WHERE ws.user_id = $1 AND le.disabled_at IS NULL
			ORDER BY le.created_at, les.set_order`,
			u.ID)
		if err == nil {
			t.Logf("--- DB log_entries+sets for user %s ---", u.ID)
			for rows.Next() {
				var variant, category, raw string
				var weight sql.NullFloat64
				var reps, setOrder int
				_ = rows.Scan(&variant, &category, &raw, &weight, &reps, &setOrder)
				w := "nil"
				if weight.Valid {
					w = fmt.Sprintf("%.0f", weight.Float64)
				}
				t.Logf("  %s %s: %s set_order=%d weight=%s reps=%d", category, variant, raw, setOrder, w, reps)
			}
			rows.Close()
		}
		var prCount int
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM personal_records WHERE user_id = $1`, u.ID).Scan(&prCount)
		t.Logf("--- personal_records count: %d ---", prCount)
		prRows, err2 := db.QueryContext(ctx, `
			SELECT ec.name, ev.name, pr.pr_type, pr.weight, pr.reps
			FROM personal_records pr
			JOIN exercise_variants ev ON ev.id = pr.exercise_variant_id
			JOIN exercise_categories ec ON ec.id = ev.category_id
			WHERE pr.user_id = $1 ORDER BY pr.created_at`, u.ID)
		if err2 == nil {
			for prRows.Next() {
				var cat, v, prType string
				var w float64
				var r sql.NullInt64
				_ = prRows.Scan(&cat, &v, &prType, &w, &r)
				repsStr := "nil"
				if r.Valid {
					repsStr = fmt.Sprintf("%d", r.Int64)
				}
				t.Logf("  PR: %s %s %s %.0fx%s", cat, v, prType, w, repsStr)
			}
			prRows.Close()
		}
	})

	// 0b. Text: two rack pulls — second is PR, triggers gpt-image-1.5 Edit API with reference images.
	// Requires GYM_PR_IMAGE_REF_1, GYM_PR_IMAGE_REF_2 and R2 configured for image_url to be set.
	t.Run("0b_text_pr_image_generation", func(t *testing.T) {
		code, out := postChatDefault("", "rack pull 200 for 5")
		if code != http.StatusOK {
			t.Fatalf("first log: got status %d: %v", code, out)
		}
		entries, _ := out["entries"].([]interface{})
		if len(entries) == 0 {
			t.Fatalf("first log: expected entries, got %d. message=%q", len(entries), out["message"])
		}

		code, out = postChatDefault("", "rack pull 370 for 5")
		if code != http.StatusOK {
			t.Fatalf("second log: got status %d: %v", code, out)
		}
		prs, _ := out["prs"].([]interface{})
		if len(prs) < 1 {
			t.Fatalf("second log: expected >=1 PR, got %d. message=%q", len(prs), out["message"])
		}
		t.Logf("prs=%d ✓", len(prs))

		refIDs := env.PRImageRefFileIDs()
		if len(refIDs) > 0 {
			prRepo := pr.NewRepo(db)
			list, err := prRepo.ListByUser(ctx, u.ID)
			if err != nil {
				t.Fatalf("list PRs: %v", err)
			}
			var foundWithImage int
			for _, p := range list {
				if p.ImageURL != "" {
					foundWithImage++
					t.Logf("PR %s has image_url=%s", p.ID, p.ImageURL)
				}
			}
			if foundWithImage == 0 {
				t.Logf("no PRs with image_url yet (R2 may be unconfigured or image gen failed)")
			} else {
				t.Logf("PR image generation: %d PR(s) with image_url ✓", foundWithImage)
			}
		} else {
			t.Logf("GYM_PR_IMAGE_REF_1/2 not set, skipping image_url check")
		}
	})

	// 1. Audio: Close Grip Bench Press, 130.
	t.Run("1_audio_close_grip_bench", func(t *testing.T) {
		path := filepath.Join(samplesDir, "20260306 133927.m4a")
		code, out := postChatDefault(path, "")
		if code != http.StatusOK {
			t.Fatalf("got status %d: %v", code, out)
		}
		msg, _ := out["message"].(string)
		if msg == "" {
			t.Errorf("expected message, got %v", out["message"])
		}
		t.Logf("message=%q", msg)
	})

	// 1b. Async audio: POST returns job_id+text immediately, poll GET /chat/jobs/{id} for result.
	t.Run("1b_async_audio_full_flow", func(t *testing.T) {
		jobStore := chat.NewJobStore()
		go jobStore.RunCleanup(context.Background())
		asyncMux := chatTestServerWithR2AndJobStore(t, db, u, chatMessagesRepo, jobStore)

		path := filepath.Join(samplesDir, "20260306 133927.m4a")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		body, _ := json.Marshal(map[string]string{"audio_base64": b64, "audio_format": "m4a"})
		req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer x")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		asyncMux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("POST /chat: got status %d: %s", rec.Code, rec.Body.String())
		}
		var out map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		jobID, _ := out["job_id"].(string)
		text, _ := out["text"].(string)
		status, _ := out["status"].(string)
		if jobID == "" {
			t.Fatalf("expected job_id, got %v", out)
		}
		if text == "" {
			t.Fatalf("expected text (transcription), got %v", out)
		}
		if status != "processing" {
			t.Fatalf("expected status=processing, got %q", status)
		}
		t.Logf("job_id=%s text=%q", jobID, text)

		deadline := time.Now().Add(30 * time.Second)
		var result map[string]interface{}
		for time.Now().Before(deadline) {
			getReq := httptest.NewRequest(http.MethodGet, "/chat/jobs/"+jobID, nil)
			getReq.Header.Set("Authorization", "Bearer x")
			getRec := httptest.NewRecorder()
			asyncMux.ServeHTTP(getRec, getReq)
			if getRec.Code != http.StatusOK {
				t.Fatalf("GET /chat/jobs: got status %d", getRec.Code)
			}
			if err := json.NewDecoder(getRec.Body).Decode(&out); err != nil {
				t.Fatal(err)
			}
			status, _ = out["status"].(string)
			if status == "complete" {
				result, _ = out["result"].(map[string]interface{})
				break
			}
			if status == "failed" {
				errMsg, _ := out["error"].(string)
				t.Fatalf("job failed: %s", errMsg)
			}
			time.Sleep(300 * time.Millisecond)
		}
		if result == nil {
			t.Fatalf("job did not complete within 30s")
		}
		msg, _ := result["message"].(string)
		if msg == "" {
			t.Errorf("expected message in result, got %v", result["message"])
		}
		t.Logf("async result message=%q", msg)
	})

	// 2. Audio: What's my last close grip bench press?
	t.Run("2_audio_query_close_grip", func(t *testing.T) {
		path := filepath.Join(samplesDir, "20260306 133935.m4a")
		code, out := postChatDefault(path, "")
		if code != http.StatusOK {
			t.Fatalf("got status %d: %v", code, out)
		}
		msg, _ := out["message"].(string)
		if msg == "" {
			t.Errorf("expected message, got %v", out["message"])
		}
		t.Logf("message=%q", msg)
	})

	// 3. Audio: RDL 300×8 + shoulder press 100×8 (PRs for new user)
	t.Run("3_audio_rdl_shoulder_pr", func(t *testing.T) {
		path := filepath.Join(samplesDir, "20260306 133944.m4a")
		code, out := postChatDefault(path, "")
		if code != http.StatusOK {
			t.Fatalf("got status %d: %v", code, out)
		}
		msg, _ := out["message"].(string)
		if msg == "" {
			t.Errorf("expected message, got %v", out["message"])
		}
		prs, _ := out["prs"].([]interface{})
		t.Logf("message=%q prs=%d", msg, len(prs))
	})

	// 4. Audio: What's my last deadlift?
	t.Run("4_audio_query_deadlift", func(t *testing.T) {
		path := filepath.Join(samplesDir, "20260306 134002.m4a")
		code, out := postChatDefault(path, "")
		if code != http.StatusOK {
			t.Fatalf("got status %d: %v", code, out)
		}
		msg, _ := out["message"].(string)
		if msg == "" {
			t.Errorf("expected message, got %v", out["message"])
		}
		t.Logf("message=%q", msg)
	})

	// 5. Text: squat 225 for 1 (1RM PR) — may get LLM follow-up asking for squat type
	t.Run("5_text_squat_1rm", func(t *testing.T) {
		code, out := postChatDefault("", "squat 225 for 1")
		if code != http.StatusOK {
			t.Fatalf("got status %d: %v", code, out)
		}
		msg, _ := out["message"].(string)
		if msg == "" {
			t.Errorf("expected message, got %v", out["message"])
		}
		prs, _ := out["prs"].([]interface{})
		t.Logf("message=%q prs=%d", msg, len(prs))
	})

	// 6. Text: change the last squat to 230 — pre-seed squat so correction has something to change
	t.Run("6_text_correction", func(t *testing.T) {
		sessionSvc := session.NewService(session.NewRepo(db))
		logentryRepo := logentry.NewRepo(db)
		exerciseRepo := exercise.NewRepo(db)
		variant, err := exerciseRepo.Resolve(ctx, u.ID, "squat", "standard")
		if err != nil || variant == nil {
			t.Fatal("need seeded squat:", err)
		}
		today := time.Now().Format("2006-01-02")
		sess, err := sessionSvc.GetOrCreateForDate(ctx, u.ID, today)
		if err != nil {
			t.Fatal(err)
		}
		w := 225.0
		entry := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "squat 225x1"}
		if err := logentryRepo.Create(ctx, entry, []logentry.SetInput{{Weight: &w, Reps: 1, SetOrder: 1}}); err != nil {
			t.Fatal(err)
		}

		code, out := postChatDefault("", "change the last squat to 230")
		if code != http.StatusOK {
			t.Fatalf("got status %d: %v", code, out)
		}
		msg, _ := out["message"].(string)
		if msg == "" {
			t.Errorf("expected message, got %v", out["message"])
		}
		var updatedWeight float64
		err = db.QueryRowContext(ctx,
			`SELECT les.weight FROM log_entry_sets les
			 JOIN log_entries le ON le.id = les.log_entry_id
			 JOIN workout_sessions ws ON ws.id = le.session_id
			 WHERE ws.user_id = $1 AND le.exercise_variant_id = $2
			 ORDER BY les.created_at DESC LIMIT 1`,
			u.ID, variant.ID,
		).Scan(&updatedWeight)
		if err != nil {
			t.Logf("could not verify correction in DB: %v", err)
		} else if updatedWeight != 230 {
			t.Errorf("correction should update weight to 230, got %.0f", updatedWeight)
		}
		t.Logf("message=%q", msg)
	})

	// 7. Text: forget that squat
	t.Run("7_text_remove", func(t *testing.T) {
		code, out := postChatDefault("", "forget that squat")
		if code != http.StatusOK {
			t.Fatalf("got status %d: %v", code, out)
		}
		msg, _ := out["message"].(string)
		if msg == "" {
			t.Errorf("expected message, got %v", out["message"])
		}
		t.Logf("message=%q", msg)
	})

	// 8. Text: bring that squat back
	t.Run("8_text_restore", func(t *testing.T) {
		code, out := postChatDefault("", "bring that squat back")
		if code != http.StatusOK {
			t.Fatalf("got status %d: %v", code, out)
		}
		msg, _ := out["message"].(string)
		if msg == "" {
			t.Errorf("expected message, got %v", out["message"])
		}
		t.Logf("message=%q", msg)
	})

	// 9. Audio: My name's Rocky
	t.Run("9_audio_set_name", func(t *testing.T) {
		path := filepath.Join(samplesDir, "20260309 142616.m4a")
		code, out := postChatDefault(path, "")
		if code != http.StatusOK {
			t.Fatalf("got status %d: %v", code, out)
		}
		msg, _ := out["message"].(string)
		if msg == "" {
			t.Errorf("expected message, got %v", out["message"])
		}
		t.Logf("message=%q", msg)
		got, _ := userRepo.GetByGoogleID(ctx, u.GoogleID)
		if got != nil && got.Name == "" {
			t.Error("expected user name to be set")
		}
	})

	// 10. Text: create note
	t.Run("10_text_create_note", func(t *testing.T) {
		code, out := postChatDefault("", "note for bench press: narrow grip feels good")
		if code != http.StatusOK {
			t.Fatalf("got status %d: %v", code, out)
		}
		msg, _ := out["message"].(string)
		if msg == "" {
			t.Errorf("expected message, got %v", out["message"])
		}
		t.Logf("message=%q", msg)
	})
}
