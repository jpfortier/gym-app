package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/jpfortier/gym-app/internal/admin"
	"github.com/jpfortier/gym-app/internal/ai"
	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/chat"
	"github.com/jpfortier/gym-app/internal/chatmessages"
	"github.com/jpfortier/gym-app/internal/command"
	"github.com/jpfortier/gym-app/internal/correction"
	"github.com/jpfortier/gym-app/internal/db"
	"github.com/jpfortier/gym-app/internal/env"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/handler"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/name"
	"github.com/jpfortier/gym-app/internal/notes"
	"github.com/jpfortier/gym-app/internal/pr"
	"github.com/jpfortier/gym-app/internal/query"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/storage"
	"github.com/jpfortier/gym-app/internal/systemlog"
	"github.com/jpfortier/gym-app/internal/usage"
	"github.com/jpfortier/gym-app/internal/user"
)

const welcomeMessage = "Yo! What should I call you?"

type userStoreWithWelcome struct {
	userRepo         *user.Repo
	chatMessagesRepo *chatmessages.Repo
}

func (s *userStoreWithWelcome) GetByGoogleID(ctx context.Context, googleID string) (*user.User, error) {
	return s.userRepo.GetByGoogleID(ctx, googleID)
}

func (s *userStoreWithWelcome) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	return s.userRepo.GetByEmail(ctx, email)
}

func (s *userStoreWithWelcome) Create(ctx context.Context, u *user.User) error {
	if err := s.userRepo.Create(ctx, u); err != nil {
		return err
	}
	_ = s.chatMessagesRepo.Append(ctx, u.ID, "assistant", welcomeMessage)
	return nil
}

func (s *userStoreWithWelcome) UpdateGoogleID(ctx context.Context, userID uuid.UUID, googleID string) error {
	return s.userRepo.UpdateGoogleID(ctx, userID, googleID)
}

// Server holds the HTTP server and dependencies.
type Server struct {
	mux    *http.ServeMux
	handler http.Handler
	db     *sql.DB
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
	r2, err := storage.NewR2()
	if err != nil {
		return nil, fmt.Errorf("R2 init: %w", err)
	}
	cmdExecutor := command.NewExecutor(
		sessionSvc, logentrySvc, logentryRepo, exerciseSvc, exerciseRepo,
		userRepo, name.NewHandler(aiClient), notesRepo, prSvc,
	)
	syslogRepo := systemlog.NewRepo(database)
	syslog := systemlog.NewRepoLogger(syslogRepo)
	chatSvc := chat.NewService(chat.Config{
		Client:           aiClient,
		UserRepo:         userRepo,
		NameHandler:      name.NewHandler(aiClient),
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
		CommandExecutor:  cmdExecutor,
		Systemlog:        syslog,
	})

	googleClientID := env.GoogleClientID()
	verifier := auth.GoogleVerifier{}
	userStore := &userStoreWithWelcome{userRepo: userRepo, chatMessagesRepo: chatMessagesRepo}

	userIDFromContext := func(ctx context.Context) *uuid.UUID {
		if u := auth.UserFromContext(ctx); u != nil {
			return &u.ID
		}
		return nil
	}
	logAction := systemlog.LogAction(syslog, userIDFromContext)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health(database))
	mux.HandleFunc("GET /dev/token", handler.DevToken)
	mux.Handle("GET /me", auth.RequireAuth(verifier, userStore, googleClientID, syslog)(logAction(http.HandlerFunc(handler.Me))))
	mux.Handle("GET /chat/messages", auth.RequireAuth(verifier, userStore, googleClientID, syslog)(logAction(http.HandlerFunc(handler.ChatMessages(chatMessagesRepo)))))
	mux.Handle("POST /chat", auth.RequireAuth(verifier, userStore, googleClientID, syslog)(logAction(http.HandlerFunc(handler.Chat(chatSvc)))))
	mux.Handle("GET /sessions", auth.RequireAuth(verifier, userStore, googleClientID, syslog)(logAction(http.HandlerFunc(handler.SessionsList(sessionRepo)))))
	mux.Handle("GET /sessions/{id}", auth.RequireAuth(verifier, userStore, googleClientID, syslog)(logAction(http.HandlerFunc(handler.SessionDetail(sessionRepo, logentryRepo, exerciseRepo)))))
	mux.Handle("GET /exercises", auth.RequireAuth(verifier, userStore, googleClientID, syslog)(logAction(http.HandlerFunc(handler.ExercisesList(exerciseRepo)))))
	mux.Handle("GET /query", auth.RequireAuth(verifier, userStore, googleClientID, syslog)(logAction(http.HandlerFunc(handler.QueryHistory(queryService, exerciseRepo)))))
	mux.Handle("GET /prs", auth.RequireAuth(verifier, userStore, googleClientID, syslog)(logAction(http.HandlerFunc(handler.PRsList(prRepo, exerciseRepo)))))
	if r2 != nil {
		mux.Handle("GET /prs/{id}/image", auth.RequireAuth(verifier, userStore, googleClientID, syslog)(logAction(http.HandlerFunc(handler.PRImage(prRepo, r2)))))
	}

	adminTpl, err := admin.LoadTemplates()
	if err != nil {
		return nil, fmt.Errorf("admin templates: %w", err)
	}
	adminHandler := &admin.Handler{
		UserRepo:         userRepo,
		SessionRepo:      sessionRepo,
		LogentryRepo:     logentryRepo,
		ExerciseRepo:     exerciseRepo,
		PrRepo:           prRepo,
		UsageRepo:        usageRepo,
		NotesRepo:        notesRepo,
		ChatMessagesRepo: chatMessagesRepo,
		SystemlogRepo:    syslogRepo,
		Templates:        adminTpl,
	}
	requireAdmin := auth.RequireAdmin(verifier, userStore, googleClientID, syslog)
	adminWithCookie := admin.InjectAuthCookie(requireAdmin)

	mux.HandleFunc("GET /admin/login", adminHandler.Login)
	mux.HandleFunc("POST /admin/login", adminHandler.Login)
	mux.Handle("POST /admin/select-user", adminWithCookie(logAction(http.HandlerFunc(adminHandler.SelectUser))))
	mux.Handle("GET /admin", adminWithCookie(logAction(http.HandlerFunc(adminHandler.Dashboard))))
	mux.Handle("GET /admin/users", adminWithCookie(logAction(http.HandlerFunc(adminHandler.Users))))
	mux.Handle("GET /admin/sessions", adminWithCookie(logAction(http.HandlerFunc(adminHandler.Sessions))))
	mux.Handle("GET /admin/sessions/{id}", adminWithCookie(logAction(http.HandlerFunc(adminHandler.SessionDetail))))
	mux.Handle("GET /admin/prs", adminWithCookie(logAction(http.HandlerFunc(adminHandler.PRs))))
	mux.Handle("GET /admin/usage", adminWithCookie(logAction(http.HandlerFunc(adminHandler.Usage))))
	mux.Handle("GET /admin/notes", adminWithCookie(logAction(http.HandlerFunc(adminHandler.Notes))))
	mux.Handle("GET /admin/logs", adminWithCookie(logAction(http.HandlerFunc(adminHandler.Logs))))

	handler := systemlog.RecoverPanic(syslog)(systemlog.AddLoggerToContext(syslog)(mux))
	return &Server{mux: mux, handler: handler, db: database}, nil
}

// Run starts the HTTP server. Blocks until error.
// Uses HTTPS when GYM_TLS_CERT_FILE and GYM_TLS_KEY_FILE are both set.
func (s *Server) Run() error {
	defer s.db.Close()
	port := env.Port()
	if port == "" {
		port = "8081"
	}
	addr := ":" + port
	certFile := env.TLSCertFile()
	keyFile := env.TLSKeyFile()
	h := s.handler
	if h == nil {
		h = s.mux
	}
	if certFile != "" && keyFile != "" {
		log.Printf("Listening on https://%s", addr)
		return http.ListenAndServeTLS(addr, certFile, keyFile, h)
	}
	log.Printf("Listening on http://%s", addr)
	return http.ListenAndServe(addr, h)
}
