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

	"github.com/google/uuid"
	"google.golang.org/api/idtoken"

	"github.com/jpfortier/gym-app/internal/ai"
	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/chat"
	"github.com/jpfortier/gym-app/internal/chatmessages"
	"github.com/jpfortier/gym-app/internal/correction"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/name"
	"github.com/jpfortier/gym-app/internal/pr"
	"github.com/jpfortier/gym-app/internal/query"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/user"
)

// chatTestService builds the chat service stack with mock AI. chatMessagesRepo can be nil. parser can be nil for default.
func chatTestService(t *testing.T, db *sql.DB, chatMessagesRepo *chatmessages.Repo, parser ai.Parser) *chat.Service {
	t.Helper()
	throttle := ai.NewThrottlerFromEnv()
	aiClient := ai.NewClient(throttle, nil)
	if parser == nil {
		parser = ai.NewParser(aiClient)
	}
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
	return chat.NewService(chat.Config{
		Client: aiClient, Parser: parser, UserRepo: user.NewRepo(db), NameHandler: name.NewHandler(aiClient),
		SessionSvc: sessionSvc, SessionRepo: sessionRepo,
		LogentrySvc: logentrySvc, LogentryRepo: logentryRepo,
		ExerciseSvc: exerciseSvc, ExerciseRepo: exerciseRepo, QuerySvc: querySvc, CorrectionSvc: correctionSvc,
		PrSvc: prSvc, PrRepo: prRepo, NotesRepo: nil, ChatMessagesRepo: chatMessagesRepo, R2: nil,
	})
}

// chatTestServer sets up a POST /chat handler with mock AI. chatMessagesRepo can be nil.
func chatTestServer(t *testing.T, db *sql.DB, u *user.User, chatMessagesRepo *chatmessages.Repo) *http.ServeMux {
	t.Helper()
	chatSvc := chatTestService(t, db, chatMessagesRepo, nil)
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
	u := &user.User{GoogleID: "chat-" + uuid.New().String(), Email: "c@test.com", Name: "C"}
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
	if out["intent"] != "log" {
		t.Errorf("got intent %v, want log", out["intent"])
	}
}

func TestChat_removeIntent(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "chat-rm-" + uuid.New().String(), Email: "rm@test.com", Name: "RM"}
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

	body, _ = json.Marshal(map[string]string{"text": "forget that"})
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
	if out["intent"] != "remove" {
		t.Errorf("got intent %v, want remove", out["intent"])
	}
	if out["message"] != "Removed." {
		t.Errorf("got message %v, want Removed.", out["message"])
	}

	body, _ = json.Marshal(map[string]string{"text": "oh sorry bring that back"})
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
	if out["intent"] != "restore" {
		t.Errorf("got intent %v, want restore", out["intent"])
	}
	if out["message"] != "Brought back." {
		t.Errorf("got message %v, want Brought back.", out["message"])
	}
}

func TestChat_contextStoresMessages(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	chatMessagesRepo := chatmessages.NewRepo(db)
	u := &user.User{GoogleID: "chat-ctx-" + uuid.New().String(), Email: "ctx@test.com", Name: "Ctx"}
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

type mockParser struct {
	intent *ai.ParsedIntent
}

func (m *mockParser) Parse(ctx context.Context, userID uuid.UUID, text string, recentMessages []ai.ChatMessage, workoutContext string, userName string) (*ai.ParsedIntent, error) {
	return m.intent, nil
}

func TestChat_needsConfirmationWhenAmbiguous(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "chat-ambig-" + uuid.New().String(), Email: "ambig@test.com", Name: "A"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	parser := &mockParser{
		intent: &ai.ParsedIntent{
			Intent:      "correction",
			Category:    "squat",
			Variant:     "standard",
			TargetRef:   "last",
			Changes:     &ai.ParsedCorrection{Weight: ptrFloat(205)},
			Ambiguities: []string{"multiple_targets"},
			UIText:      &ai.ParsedUIText{Preview: "Update your last squat to 205 lb."},
		},
	}
	chatSvc := chatTestService(t, db, nil, parser)
	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: u.GoogleID}}
	mux := http.NewServeMux()
	mux.Handle("POST /chat", auth.RequireAuth(verifier, userRepo, "aud")(http.HandlerFunc(Chat(chatSvc))))

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
	if out["needs_confirmation"] != true {
		t.Errorf("got needs_confirmation %v, want true", out["needs_confirmation"])
	}
	if out["intent"] != "correction" {
		t.Errorf("got intent %v, want correction", out["intent"])
	}
}

func TestChat_setName(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "name-" + uuid.New().String(), Email: "name@test.com", Name: ""}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM chat_messages WHERE user_id = $1", u.ID) })
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	mux := chatTestServer(t, db, u, nil)

	body, _ := json.Marshal(map[string]string{"text": "Peter"})
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
	if out["intent"] != "set_name" {
		t.Errorf("got intent %v, want set_name", out["intent"])
	}
	msg, _ := out["message"].(string)
	if msg == "" {
		t.Error("expected message")
	}
	// Mock twist: Peter -> Pete
	if !strings.Contains(msg, "Pete") && !strings.Contains(msg, "Peter") {
		t.Errorf("message should mention name: %q", msg)
	}
	// Verify user name was updated
	got, _ := userRepo.GetByGoogleID(ctx, u.GoogleID)
	if got == nil || got.Name == "" {
		t.Error("expected user name to be set")
	}
}

func ptrFloat(f float64) *float64 { return &f }
