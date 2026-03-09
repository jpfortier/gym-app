// Transcribe sample audio files via Whisper and write README.md.
// Run from gym root: go run ./cmd/transcribe
// Requires GYM_OPENAI_API_KEY in .env. Uses GYM_OPENAI_TEST_MODE to skip real API.
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jpfortier/gym-app/internal/ai"
	"github.com/jpfortier/gym-app/internal/env"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if env.OpenAIAPIKey() == "" {
		return fmt.Errorf("GYM_OPENAI_API_KEY not set")
	}
	if env.OpenAITestMode() {
		return fmt.Errorf("GYM_OPENAI_TEST_MODE=true; set to false for real transcription")
	}

	samplesDir := filepath.Join("samples", "audio")
	entries, err := os.ReadDir(samplesDir)
	if err != nil {
		return fmt.Errorf("read %s: %w", samplesDir, err)
	}

	var m4aFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".m4a") {
			m4aFiles = append(m4aFiles, name)
		}
	}
	sort.Strings(m4aFiles)

	client := ai.NewClient(ai.NewThrottler(60, 1000, 10), nil)
	ctx := context.Background()
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	var sb strings.Builder
	sb.WriteString("# Audio Samples\n\n")
	sb.WriteString("Sample m4a files for testing the chat/voice pipeline. Transcripts via Whisper.\n\n")
	sb.WriteString("| File | Transcript |\n")
	sb.WriteString("|------|------------|\n")

	for _, name := range m4aFiles {
		path := filepath.Join(samplesDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		text, err := client.Transcribe(ctx, userID, b64, "m4a")
		if err != nil {
			return fmt.Errorf("transcribe %s: %w", name, err)
		}
		text = strings.TrimSpace(text)
		textEsc := strings.ReplaceAll(text, "|", "\\|")
		textEsc = strings.ReplaceAll(textEsc, "\n", " ")
		sb.WriteString(fmt.Sprintf("| %s | %s |\n", name, textEsc))
		fmt.Printf("%s: %s\n", name, text)
	}

	readmePath := filepath.Join(samplesDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("write %s: %w", readmePath, err)
	}
	fmt.Printf("\nWrote %s\n", readmePath)
	return nil
}
