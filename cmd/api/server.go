package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"github.com/jpfortier/gym-app/internal/ai"
	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/chat"
	"github.com/jpfortier/gym-app/internal/chatmessages"
	"github.com/jpfortier/gym-app/internal/correction"
	"github.com/jpfortier/gym-app/internal/db"
	"github.com/jpfortier/gym-app/internal/env"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/handler"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/notes"
	"github.com/jpfortier/gym-app/internal/pr"
	"github.com/jpfortier/gym-app/internal/query"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/storage"
	"github.com/jpfortier/gym-app/internal/usage"
	"github.com/jpfortier/gym-app/internal/user"
)

// Server holds the HTTP server and dependencies.
type Server struct {
	mux *http.ServeMux
	db  *sql.DB
}

// NewServer wires dependencies and returns a Server. Call Run() to start.
func NewServer(ctx context.Context) (*Server, error) {
	database, err := db.New(ctx)
	if err != nil {
		return nil, err
	}

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
	notesRepo := notes.NewRepo(database)
	usageRepo := usage.NewRepo(database)
	chatMessagesRepo := chatmessages.NewRepo(database)

	throttle := ai.NewThrottlerFromEnv()
	aiClient := ai.NewClient(throttle, usageRepo)
	exerciseSvc := exercise.NewService(exerciseRepo, aiClient)
	parser := ai.NewParser(aiClient)
	r2, err := storage.NewR2()
	if err != nil {
		log.Printf("R2 init failed (PR images disabled): %v", err)
		r2 = nil
	}
	chatSvc := chat.NewService(chat.Config{
		Client:           aiClient,
		Parser:           parser,
		SessionSvc:       sessionSvc,
		SessionRepo:      sessionRepo,
		LogentrySvc:      logentrySvc,
		LogentryRepo:     logentryRepo,
		ExerciseSvc:      exerciseSvc,
		ExerciseRepo:     exerciseRepo,
		QuerySvc:         queryService,
		CorrectionSvc:    correctionSvc,
		PrSvc:            prSvc,
		PrRepo:           prRepo,
		NotesRepo:        notesRepo,
		ChatMessagesRepo: chatMessagesRepo,
		R2:               r2,
	})

	googleClientID := env.GoogleClientID()
	verifier := auth.GoogleVerifier{}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health(database))
	mux.Handle("GET /me", auth.RequireAuth(verifier, userRepo, googleClientID)(http.HandlerFunc(handler.Me)))
	mux.Handle("POST /chat", auth.RequireAuth(verifier, userRepo, googleClientID)(http.HandlerFunc(handler.Chat(chatSvc))))
	mux.Handle("GET /sessions", auth.RequireAuth(verifier, userRepo, googleClientID)(http.HandlerFunc(handler.SessionsList(sessionRepo))))
	mux.Handle("GET /sessions/{id}", auth.RequireAuth(verifier, userRepo, googleClientID)(http.HandlerFunc(handler.SessionDetail(sessionRepo, logentryRepo, exerciseRepo))))
	mux.Handle("GET /exercises", auth.RequireAuth(verifier, userRepo, googleClientID)(http.HandlerFunc(handler.ExercisesList(exerciseRepo))))
	mux.Handle("GET /query", auth.RequireAuth(verifier, userRepo, googleClientID)(http.HandlerFunc(handler.QueryHistory(queryService, exerciseRepo))))
	mux.Handle("GET /prs", auth.RequireAuth(verifier, userRepo, googleClientID)(http.HandlerFunc(handler.PRsList(prRepo, exerciseRepo))))
	if r2 != nil {
		mux.Handle("GET /prs/{id}/image", auth.RequireAuth(verifier, userRepo, googleClientID)(http.HandlerFunc(handler.PRImage(prRepo, r2))))
	}

	return &Server{mux: mux, db: database}, nil
}

// Run starts the HTTP server. Blocks until error.
func (s *Server) Run() error {
	defer s.db.Close()
	port := env.Port()
	if port == "" {
		port = "8081"
	}
	addr := ":" + port
	log.Printf("Listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}
