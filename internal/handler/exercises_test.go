package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/api/idtoken"

	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/testutil"
	"github.com/jpfortier/gym-app/internal/user"
)

func TestExercisesList_returnsCategoriesAndVariants(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "exercises")
	userRepo := user.NewRepo(db)
	exerciseRepo := exercise.NewRepo(db)
	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: u.GoogleID}}
	mux := http.NewServeMux()
	mux.Handle("GET /exercises", auth.RequireAuth(verifier, userRepo, "aud", nil)(http.HandlerFunc(ExercisesList(exerciseRepo))))

	req := httptest.NewRequest(http.MethodGet, "/exercises", nil)
	req.Header.Set("Authorization", "Bearer x")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var out []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Skip("no seeded categories; run migrations")
	}
	first := out[0]
	if _, ok := first["category_name"]; !ok {
		t.Errorf("expected category_name in %v", first)
	}
	if _, ok := first["variant_name"]; !ok {
		t.Errorf("expected variant_name in %v", first)
	}
}

