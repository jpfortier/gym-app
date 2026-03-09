package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/api/idtoken"

	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/pr"
	"github.com/jpfortier/gym-app/internal/user"
)

func TestPRsList_returnsUserPRs(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	u := &user.User{GoogleID: "prs-" + uuid.New().String(), Email: "pr@test.com", Name: "PR"}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID) })

	var variantID uuid.UUID
	if err := db.QueryRowContext(ctx, `SELECT id FROM exercise_variants WHERE user_id IS NULL LIMIT 1`).Scan(&variantID); err != nil {
		t.Fatal(err)
	}

	prRepo := pr.NewRepo(db)
	w := 225.0
	prRec := &pr.PersonalRecord{UserID: u.ID, ExerciseVariantID: variantID, PRType: "weight", Weight: w}
	if err := prRepo.Create(ctx, prRec); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM personal_records WHERE id = $1", prRec.ID) })

	exerciseRepo := exercise.NewRepo(db)
	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: u.GoogleID}}
	mux := http.NewServeMux()
	mux.Handle("GET /prs", auth.RequireAuth(verifier, userRepo, "aud")(http.HandlerFunc(PRsList(prRepo, exerciseRepo))))

	req := httptest.NewRequest(http.MethodGet, "/prs", nil)
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
	if len(out) != 1 {
		t.Errorf("got %d prs, want 1", len(out))
	}
	if out[0]["weight"] != 225.0 {
		t.Errorf("got weight %v, want 225", out[0]["weight"])
	}
}
