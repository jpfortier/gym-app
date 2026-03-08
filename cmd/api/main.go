package main

import (
	"context"
	"log"
)

func main() {
	ctx := context.Background()
	srv, err := NewServer(ctx)
	if err != nil {
		log.Fatalf("server: %v", err)
	}
	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}
