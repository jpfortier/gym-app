package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jpfortier/gym-app/internal/chatmessages"
	"github.com/jpfortier/gym-app/internal/env"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
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

	postChat := func(audioPath string, text string) (int, map[string]interface{}) {
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

	// Clear path: explicit prompts that should log without follow-up. Verifies the flow actually works.
	t.Run("0_clear_path_log_and_correction", func(t *testing.T) {
		code, out := postChat("", "bench press 135 for 8")
		if code != http.StatusOK {
			t.Fatalf("log: got status %d: %v", code, out)
		}
		entries, _ := out["entries"].([]interface{})
		if len(entries) == 0 {
			t.Fatalf("clear path: expected entries after logging, got %d. message=%q", len(entries), out["message"])
		}
		t.Logf("logged %d entries", len(entries))

		code, out = postChat("", "change the last bench to 140")
		if code != http.StatusOK {
			t.Fatalf("correction: got status %d: %v", code, out)
		}
		msg, _ := out["message"].(string)
		if msg == "" {
			t.Errorf("correction: expected message, got %v", out["message"])
		}
		t.Logf("correction message=%q", msg)
	})

	// 1. Audio: Close Grip Bench Press, 130.
	t.Run("1_audio_close_grip_bench", func(t *testing.T) {
		path := filepath.Join(samplesDir, "20260306 133927.m4a")
		code, out := postChat(path, "")
		if code != http.StatusOK {
			t.Fatalf("got status %d: %v", code, out)
		}
		msg, _ := out["message"].(string)
		if msg == "" {
			t.Errorf("expected message, got %v", out["message"])
		}
		t.Logf("message=%q", msg)
	})

	// 2. Audio: What's my last close grip bench press?
	t.Run("2_audio_query_close_grip", func(t *testing.T) {
		path := filepath.Join(samplesDir, "20260306 133935.m4a")
		code, out := postChat(path, "")
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
		code, out := postChat(path, "")
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
		code, out := postChat(path, "")
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
		code, out := postChat("", "squat 225 for 1")
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

		code, out := postChat("", "change the last squat to 230")
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
		code, out := postChat("", "forget that squat")
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
		code, out := postChat("", "bring that squat back")
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
		code, out := postChat(path, "")
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
		code, out := postChat("", "note for bench press: narrow grip feels good")
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
