// Presign PR image URL for viewing. Run: go run ./scripts/presign-pr-image
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/jpfortier/gym-app/internal/storage"
)

func main() {
	_ = godotenv.Load(".env")
	userID := uuid.MustParse("019cdec4-5872-7b52-9a33-94963ad0cfdd")
	prID := uuid.MustParse("019cdec4-ea63-7c0d-9307-6a32d1f83299")
	r2, err := storage.NewR2()
	if err != nil || r2 == nil {
		fmt.Fprintln(os.Stderr, "R2 not configured:", err)
		os.Exit(1)
	}
	url, err := r2.PresignPRImage(context.Background(), userID, prID, 3600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(url)
}
