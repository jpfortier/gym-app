package main

import (
	"context"
	"log"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	ctx := context.Background()
	srv, err := NewServer(ctx)
	if err != nil {
		log.Fatalf("server: %v", err)
	}
	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}
