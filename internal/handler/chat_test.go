package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/api/idtoken"

	"github.com/jpfortier/gym-app/internal/ai"
	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/chat"
	"github.com/jpfortier/gym-app/internal/chatmessages"
	"github.com/jpfortier/gym-app/internal/command"
	"github.com/jpfortier/gym-app/internal/correction"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/name"
	"github.com/jpfortier/gym-app/internal/notes"
	"github.com/jpfortier/gym-app/internal/pr"
	"github.com/jpfortier/gym-app/internal/query"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/storage"
	"github.com/jpfortier/gym-app/internal/user"
)

// chatTestService builds the chat service stack with mock AI. chatMessagesRepo can be nil.
func chatTestService(t *testing.T, db *sql.DB, chatMessagesRepo *chatmessages.Repo) *chat.Service {
	t.Helper()
	throttle := ai.NewThrottlerFromEnv()
	aiClient := ai.NewClient(throttle, nil)
	sessionRepo := session.NewRepo(db)
	logentryRepo := logentry.NewRepo(db)
	exerciseRepo := exercise.NewRepo(db)
	exerciseSvc := exercise.NewService(exerciseRepo, aiClient)
	prRepo := pr.NewRepo(db)
	sessionSvc := session.NewService(sessionRepo)
	logentrySvc := logentry.NewService(logentryRepo, sessionSvc)
	querySvc := query.NewService(exerciseRepo, logentryRepo, sessionRepo)
	correctionSvc := correction.NewService(logentryRepo, exerciseRepo)
	prSvc := pr.NewService(prRepo)
	notesRepo := notes.NewRepo(db)
	cmdExecutor := command.NewExecutor(
		sessionSvc, logentrySvc, logentryRepo, exerciseSvc, exerciseRepo,
		user.NewRepo(db), name.NewHandler(aiClient), notesRepo, prSvc,
	)
	return chat.NewService(chat.Config{
		Client: aiClient, UserRepo: user.NewRepo(db), NameHandler: name.NewHandler(aiClient),
		SessionSvc: sessionSvc, SessionRepo: sessionRepo,
		LogentrySvc: logentrySvc, LogentryRepo: logentryRepo,
		ExerciseSvc: exerciseSvc, ExerciseRepo: exerciseRepo, QuerySvc: querySvc, CorrectionSvc: correctionSvc,
		PrSvc: prSvc, PrRepo: prRepo, NotesRepo: notesRepo, ChatMessagesRepo: chatMessagesRepo, R2: nil,
		CommandExecutor: cmdExecutor,
	})
}

// chatTestServer sets up a POST /chat handler with mock AI. chatMessagesRepo can be nil.
func chatTestServer(t *testing.T, db *sql.DB, u *user.User, chatMessagesRepo *chatmessages.Repo) *http.ServeMux {
	t.Helper()
	chatSvc := chatTestService(t, db, chatMessagesRepo)
	userRepo := user.NewRepo(db)
	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: u.GoogleID}}
	mux := http.NewServeMux()
	mux.Handle("POST /chat", auth.RequireAuth(verifier, userRepo, "aud")(http.HandlerFunc(Chat(chatSvc))))
	return mux
}

// chatTestServiceWithR2 builds the chat service with real AI and R2 (when configured). For integration tests.
func chatTestServiceWithR2(t *testing.T, db *sql.DB, chatMessagesRepo *chatmessages.Repo) *chat.Service {
	t.Helper()
	throttle := ai.NewThrottlerFromEnv()
	aiClient := ai.NewClient(throttle, nil)
	sessionRepo := session.NewRepo(db)
	logentryRepo := logentry.NewRepo(db)
	exerciseRepo := exercise.NewRepo(db)
	exerciseSvc := exercise.NewService(exerciseRepo, aiClient)
	prRepo := pr.NewRepo(db)
	sessionSvc := session.NewService(sessionRepo)
	logentrySvc := logentry.NewService(logentryRepo, sessionSvc)
	querySvc := query.NewService(exerciseRepo, logentryRepo, sessionRepo)
	correctionSvc := correction.NewService(logentryRepo, exerciseRepo)
	prSvc := pr.NewService(prRepo)
	notesRepo := notes.NewRepo(db)
	cmdExecutor := command.NewExecutor(
		sessionSvc, logentrySvc, logentryRepo, exerciseSvc, exerciseRepo,
		user.NewRepo(db), name.NewHandler(aiClient), notesRepo, prSvc,
	)
	r2, _ := storage.NewR2()
	return chat.NewService(chat.Config{
		Client: aiClient, UserRepo: user.NewRepo(db), NameHandler: name.NewHandler(aiClient),
		SessionSvc: sessionSvc, SessionRepo: sessionRepo,
		LogentrySvc: logentrySvc, LogentryRepo: logentryRepo,
		ExerciseSvc: exerciseSvc, ExerciseRepo: exerciseRepo, QuerySvc: querySvc, CorrectionSvc: correctionSvc,
		PrSvc: prSvc, PrRepo: prRepo, NotesRepo: notesRepo, ChatMessagesRepo: chatMessagesRepo, R2: r2,
		CommandExecutor: cmdExecutor,
	})
}

// chatTestServerWithR2 sets up POST /chat with real AI and R2. For integration tests.
func chatTestServerWithR2(t *testing.T, db *sql.DB, u *user.User, chatMessagesRepo *chatmessages.Repo) *http.ServeMux {
	t.Helper()
	chatSvc := chatTestServiceWithR2(t, db, chatMessagesRepo)
	userRepo := user.NewRepo(db)
	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: u.GoogleID}}
	mux := http.NewServeMux()
	mux.Handle("POST /chat", auth.RequireAuth(verifier, userRepo, "aud")(http.HandlerFunc(Chat(chatSvc))))
	return mux
}

func TestChat_logIntent(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "chat-" + uuid.New().String(), Email: "c-" + uuid.New().String() + "@test.com", Name: "C"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	mux := chatTestServer(t, db, u, nil)

	body, _ := json.Marshal(map[string]string{"text": "bench press 135 for 8"})
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer x")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var out map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if msg, _ := out["message"].(string); msg == "" {
		t.Errorf("expected message, got %v", out["message"])
	}
}

func TestChat_removeIntent(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "chat-rm-" + uuid.New().String(), Email: "rm-" + uuid.New().String() + "@test.com", Name: "RM"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM log_entries WHERE session_id IN (SELECT id FROM workout_sessions WHERE user_id = $1)", u.ID) })
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM workout_sessions WHERE user_id = $1", u.ID) })
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	mux := chatTestServer(t, db, u, nil)

	body, _ := json.Marshal(map[string]string{"text": "bench press 135 for 8"})
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer x")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("log: got status %d: %s", rec.Code, rec.Body.String())
	}

	body, _ = json.Marshal(map[string]string{"text": "forget that bench"})
	req = httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer x")
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("remove: got status %d: %s", rec.Code, rec.Body.String())
	}
	var out map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if msg, _ := out["message"].(string); msg != "Scratched." {
		t.Errorf("got message %v, want Scratched.", out["message"])
	}

	body, _ = json.Marshal(map[string]string{"text": "oh sorry bring that bench back"})
	req = httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer x")
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("restore: got status %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if msg, _ := out["message"].(string); msg != "Back in." {
		t.Errorf("got message %v, want Back in.", out["message"])
	}
}

func TestChat_contextStoresMessages(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	chatMessagesRepo := chatmessages.NewRepo(db)
	u := &user.User{GoogleID: "chat-ctx-" + uuid.New().String(), Email: "ctx-" + uuid.New().String() + "@test.com", Name: "Ctx"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM chat_messages WHERE user_id = $1", u.ID) })
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM log_entries WHERE session_id IN (SELECT id FROM workout_sessions WHERE user_id = $1)", u.ID) })
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM workout_sessions WHERE user_id = $1", u.ID) })
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	mux := chatTestServer(t, db, u, chatMessagesRepo)

	body, _ := json.Marshal(map[string]string{"text": "bench press 135 for 8"})
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer x")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d: %s", rec.Code, rec.Body.String())
	}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM chat_messages WHERE user_id = $1", u.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("got %d chat_messages, want 2 (user + assistant)", count)
	}
}

func TestChat_correctionWithSquat(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "chat-corr-" + uuid.New().String(), Email: "corr-" + uuid.New().String() + "@test.com", Name: "C"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	sessionRepo := session.NewRepo(db)
	logentryRepo := logentry.NewRepo(db)
	exerciseRepo := exercise.NewRepo(db)
	variant, err := exerciseRepo.Resolve(ctx, u.ID, "squat", "standard")
	if err != nil || variant == nil {
		t.Fatal("need seeded squat:", err)
	}
	parsed, _ := time.Parse("2006-01-02", time.Now().Format("2006-01-02"))
	sess := &session.Session{UserID: u.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}
	w := 185.0
	entry := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variant.ID, RawSpeech: "squat"}
	if err := logentryRepo.Create(ctx, entry, []logentry.SetInput{{Weight: &w, Reps: 5, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}

	mux := chatTestServer(t, db, u, nil)
	body, _ := json.Marshal(map[string]string{"text": "change the last squat to 205"})
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer x")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d: %s", rec.Code, rec.Body.String())
	}
	var out map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if msg, _ := out["message"].(string); msg == "" {
		t.Errorf("expected message for correction, got %v", out["message"])
	}
}

func TestChat_setName(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "name-" + uuid.New().String(), Email: "name-" + uuid.New().String() + "@test.com", Name: ""}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM chat_messages WHERE user_id = $1", u.ID) })
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	mux := chatTestServer(t, db, u, nil)

	body, _ := json.Marshal(map[string]string{"text": "Brandon"})
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer x")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d: %s", rec.Code, rec.Body.String())
	}
	var out map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if msg, _ := out["message"].(string); msg == "" {
		t.Errorf("expected message for set_name, got %v", out["message"])
	}
	msg, _ := out["message"].(string)
	if msg == "" {
		t.Error("expected message")
	}
	// Mock twist: Brandon -> Brando
	if !strings.Contains(msg, "Brando") && !strings.Contains(msg, "Brandon") {
		t.Errorf("message should mention name: %q", msg)
	}
	// Verify user name was updated
	got, _ := userRepo.GetByGoogleID(ctx, u.GoogleID)
	if got == nil || got.Name == "" {
		t.Error("expected user name to be set")
	}
}

// TestChat_logAndQuerySamplesFromAudio exercises log/query samples from samples/audio/README.md.
func TestChat_logAndQuerySamplesFromAudio(t *testing.T) {
	cases := []struct {
		label       string
		text        string
		wantIntent  string // deprecated, used for description
		description string
	}{
		{"Close Grip Bench Press", "Close Grip Bench Press, 130.", "log", "single exercise, close grip variant"},
		{"query close grip bench", "What's my last close grip bench press?", "query", "query by variant"},
		{"RDL and shoulder press", "RDL 3 sets of 8 at 300 lbs and shoulder press 4 sets of 8 at 100 lbs.", "log", "multi-exercise log"},
		{"query deadlift", "What's my last deadlift?", "query", "query deadlift"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			db := dbForTest(t)
			defer db.Close()
			ctx := context.Background()

			userRepo := user.NewRepo(db)
			u := &user.User{GoogleID: "samples-" + uuid.New().String(), Email: "samples-" + uuid.New().String() + "@test.com", Name: "S"}
			if err := userRepo.Create(ctx, u); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_, _ = db.ExecContext(ctx, "DELETE FROM log_entry_sets WHERE log_entry_id IN (SELECT id FROM log_entries WHERE session_id IN (SELECT id FROM workout_sessions WHERE user_id = $1))", u.ID)
				_, _ = db.ExecContext(ctx, "DELETE FROM log_entries WHERE session_id IN (SELECT id FROM workout_sessions WHERE user_id = $1)", u.ID)
				_, _ = db.ExecContext(ctx, "DELETE FROM workout_sessions WHERE user_id = $1", u.ID)
				_, _ = db.ExecContext(ctx, "DELETE FROM chat_messages WHERE user_id = $1", u.ID)
				_, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID)
			})

			mux := chatTestServer(t, db, u, nil)

			body, _ := json.Marshal(map[string]string{"text": tc.text})
			req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer x")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("got status %d: %s", rec.Code, rec.Body.String())
			}
			var out map[string]interface{}
			if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
				t.Fatal(err)
			}
			msg, _ := out["message"].(string)
			if msg == "" && tc.wantIntent != "unknown" {
				t.Errorf("expected message for %q, got empty", tc.text)
			}
			t.Logf("text=%q → wantIntent=%s message=%q (%s)", tc.text, tc.wantIntent, msg, tc.description)
		})
	}
}

func ptrFloat(f float64) *float64 { return &f }
