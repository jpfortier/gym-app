package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/api/idtoken"

	"github.com/jpfortier/gym-app/internal/ai"
	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/chat"
	"github.com/jpfortier/gym-app/internal/correction"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/pr"
	"github.com/jpfortier/gym-app/internal/query"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/user"
)

func TestChat_logIntent(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "chat-" + uuid.New().String(), Email: "c@test.com", Name: "C"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	throttle := ai.NewThrottlerFromEnv()
	aiClient := ai.NewClient(throttle, nil)
	parser := ai.NewParser(aiClient)
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

	chatSvc := chat.NewService(aiClient, parser, sessionSvc, logentrySvc, logentryRepo, exerciseSvc, exerciseRepo, querySvc, correctionSvc, prSvc, prRepo, nil, nil)
	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: u.GoogleID}}
	mux := http.NewServeMux()
	mux.Handle("POST /chat", auth.RequireAuth(verifier, userRepo, "aud")(http.HandlerFunc(Chat(chatSvc))))

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
	t.Cleanup(func() { db.ExecContext(ctx, "DELETE FROM log_entries WHERE session_id IN (SELECT id FROM workout_sessions WHERE user_id = $1)", u.ID) })
	t.Cleanup(func() { db.ExecContext(ctx, "DELETE FROM workout_sessions WHERE user_id = $1", u.ID) })
	t.Cleanup(func() { db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	throttle := ai.NewThrottlerFromEnv()
	aiClient := ai.NewClient(throttle, nil)
	parser := ai.NewParser(aiClient)
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

	chatSvc := chat.NewService(aiClient, parser, sessionSvc, logentrySvc, logentryRepo, exerciseSvc, exerciseRepo, querySvc, correctionSvc, prSvc, prRepo, nil, nil)
	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: u.GoogleID}}
	mux := http.NewServeMux()
	mux.Handle("POST /chat", auth.RequireAuth(verifier, userRepo, "aud")(http.HandlerFunc(Chat(chatSvc))))

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
