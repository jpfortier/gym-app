package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/db"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/handler"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/session"
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
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health(database))
	mux.Handle("GET /me", auth.RequireAuth(auth.GoogleVerifier{}, userRepo, googleClientID)(http.HandlerFunc(handler.Me)))
	mux.Handle("GET /sessions", auth.RequireAuth(auth.GoogleVerifier{}, userRepo, googleClientID)(http.HandlerFunc(handler.SessionsList(sessionRepo))))
	mux.Handle("GET /sessions/{id}", auth.RequireAuth(auth.GoogleVerifier{}, userRepo, googleClientID)(http.HandlerFunc(handler.SessionDetail(sessionRepo, logentryRepo, exerciseRepo))))

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
