package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/jpfortier/gym-app/internal/ai"
	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/chat"
	"github.com/jpfortier/gym-app/internal/correction"
	"github.com/jpfortier/gym-app/internal/db"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/handler"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/pr"
	"github.com/jpfortier/gym-app/internal/query"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/storage"
	"github.com/jpfortier/gym-app/internal/user"
)

func main() {
	ctx := context.Background()
	database, err := db.New(ctx)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer database.Close()

	userRepo := user.NewRepo(database)
	sessionRepo := session.NewRepo(database)
	logentryRepo := logentry.NewRepo(database)
	exerciseRepo := exercise.NewRepo(database)
	prRepo := pr.NewRepo(database)
	sessionSvc := session.NewService(sessionRepo)
	logentrySvc := logentry.NewService(logentryRepo, sessionSvc)
	queryService := query.NewService(exerciseRepo, logentryRepo, sessionRepo)
	correctionSvc := correction.NewService(logentryRepo, exerciseRepo)
	prSvc := pr.NewService(prRepo)

	throttle := ai.NewThrottlerFromEnv()
	aiClient := ai.NewClient(throttle)
	exerciseSvc := exercise.NewService(exerciseRepo, aiClient)
	parser := ai.NewParser(aiClient)
	r2, _ := storage.NewR2()
	chatSvc := chat.NewService(aiClient, parser, sessionSvc, logentrySvc, logentryRepo, exerciseSvc, exerciseRepo, queryService, correctionSvc, prSvc, prRepo, r2)

	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health(database))
	mux.Handle("GET /me", auth.RequireAuth(auth.GoogleVerifier{}, userRepo, googleClientID)(http.HandlerFunc(handler.Me)))
	mux.Handle("POST /chat", auth.RequireAuth(auth.GoogleVerifier{}, userRepo, googleClientID)(http.HandlerFunc(handler.Chat(chatSvc))))
	mux.Handle("GET /sessions", auth.RequireAuth(auth.GoogleVerifier{}, userRepo, googleClientID)(http.HandlerFunc(handler.SessionsList(sessionRepo))))
	mux.Handle("GET /sessions/{id}", auth.RequireAuth(auth.GoogleVerifier{}, userRepo, googleClientID)(http.HandlerFunc(handler.SessionDetail(sessionRepo, logentryRepo, exerciseRepo))))
	mux.Handle("GET /exercises", auth.RequireAuth(auth.GoogleVerifier{}, userRepo, googleClientID)(http.HandlerFunc(handler.ExercisesList(exerciseRepo))))
	mux.Handle("GET /query", auth.RequireAuth(auth.GoogleVerifier{}, userRepo, googleClientID)(http.HandlerFunc(handler.QueryHistory(queryService, exerciseRepo))))
	mux.Handle("GET /prs", auth.RequireAuth(auth.GoogleVerifier{}, userRepo, googleClientID)(http.HandlerFunc(handler.PRsList(prRepo, exerciseRepo))))
	if r2 != nil {
		mux.Handle("GET /prs/{id}/image", auth.RequireAuth(auth.GoogleVerifier{}, userRepo, googleClientID)(http.HandlerFunc(handler.PRImage(prRepo, r2))))
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	addr := ":" + port
	log.Printf("Listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
