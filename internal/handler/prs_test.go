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
	"github.com/jpfortier/gym-app/internal/testutil"
	"github.com/jpfortier/gym-app/internal/user"
)

func TestPRsList_returnsUserPRs(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "prs")
	userRepo := user.NewRepo(db)

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
	mux.Handle("GET /prs", auth.RequireAuth(verifier, userRepo, "aud", nil)(http.HandlerFunc(PRsList(prRepo, exerciseRepo))))

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

func TestPRImage_returns404WhenImageNotReady(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "primg")
	userRepo := user.NewRepo(db)

	var variantID uuid.UUID
	if err := db.QueryRowContext(ctx, `SELECT id FROM exercise_variants WHERE user_id IS NULL LIMIT 1`).Scan(&variantID); err != nil {
		t.Fatal(err)
	}

	prRepo := pr.NewRepo(db)
	prRec := &pr.PersonalRecord{UserID: u.ID, ExerciseVariantID: variantID, PRType: "weight", Weight: 135}
	if err := prRepo.Create(ctx, prRec); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM personal_records WHERE id = $1", prRec.ID) })

	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: u.GoogleID}}
	mux := http.NewServeMux()
	mux.Handle("GET /prs/{id}/image", auth.RequireAuth(verifier, userRepo, "aud", nil)(http.HandlerFunc(PRImage(prRepo, nil))))

	req := httptest.NewRequest(http.MethodGet, "/prs/"+prRec.ID.String()+"/image", nil)
	req.Header.Set("Authorization", "Bearer x")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want 404 (image not ready): %s", rec.Code, rec.Body.String())
	}
}

func TestPRImage_returns404WhenWrongUser(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u1 := testutil.CreateTestUser(t, db, ctx, "primg-u1")
	u2 := testutil.CreateTestUser(t, db, ctx, "primg-u2")
	userRepo := user.NewRepo(db)

	var variantID uuid.UUID
	if err := db.QueryRowContext(ctx, `SELECT id FROM exercise_variants WHERE user_id IS NULL LIMIT 1`).Scan(&variantID); err != nil {
		t.Fatal(err)
	}

	prRepo := pr.NewRepo(db)
	prRec := &pr.PersonalRecord{UserID: u1.ID, ExerciseVariantID: variantID, PRType: "weight", Weight: 135}
	if err := prRepo.Create(ctx, prRec); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM personal_records WHERE id = $1", prRec.ID) })
	if err := prRepo.UpdateImageURL(ctx, prRec.ID, "pr/u1/"+prRec.ID.String()+".png"); err != nil {
		t.Fatal(err)
	}

	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: u2.GoogleID}}
	mux := http.NewServeMux()
	mux.Handle("GET /prs/{id}/image", auth.RequireAuth(verifier, userRepo, "aud", nil)(http.HandlerFunc(PRImage(prRepo, nil))))

	req := httptest.NewRequest(http.MethodGet, "/prs/"+prRec.ID.String()+"/image", nil)
	req.Header.Set("Authorization", "Bearer x")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want 404 (wrong user): %s", rec.Code, rec.Body.String())
	}
}

func TestPRImage_returns404WhenNotFound(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "primg-nf")
	userRepo := user.NewRepo(db)
	prRepo := pr.NewRepo(db)
	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: u.GoogleID}}
	mux := http.NewServeMux()
	mux.Handle("GET /prs/{id}/image", auth.RequireAuth(verifier, userRepo, "aud", nil)(http.HandlerFunc(PRImage(prRepo, nil))))

	fakeID := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/prs/"+fakeID+"/image", nil)
	req.Header.Set("Authorization", "Bearer x")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want 404 (not found): %s", rec.Code, rec.Body.String())
	}
}

func TestPRImage_returns401WhenUnauthorized(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()

	userRepo := user.NewRepo(db)
	prRepo := pr.NewRepo(db)
	mux := http.NewServeMux()
	mux.Handle("GET /prs/{id}/image", auth.RequireAuth(&mockVerifier{}, userRepo, "aud", nil)(http.HandlerFunc(PRImage(prRepo, nil))))

	req := httptest.NewRequest(http.MethodGet, "/prs/"+uuid.New().String()+"/image", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want 401 (missing auth): %s", rec.Code, rec.Body.String())
	}
}

func TestPRImage_returns400WhenInvalidID(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "primg-inv")
	userRepo := user.NewRepo(db)
	prRepo := pr.NewRepo(db)
	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: u.GoogleID}}
	mux := http.NewServeMux()
	mux.Handle("GET /prs/{id}/image", auth.RequireAuth(verifier, userRepo, "aud", nil)(http.HandlerFunc(PRImage(prRepo, nil))))

	req := httptest.NewRequest(http.MethodGet, "/prs/not-a-uuid/image", nil)
	req.Header.Set("Authorization", "Bearer x")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestPRImage_returns503WhenR2NotConfigured(t *testing.T) {
	db := dbForTest(t)
	defer db.Close()
	ctx := context.Background()

	u := testutil.CreateTestUser(t, db, ctx, "primg-r2")
	userRepo := user.NewRepo(db)

	var variantID uuid.UUID
	if err := db.QueryRowContext(ctx, `SELECT id FROM exercise_variants WHERE user_id IS NULL LIMIT 1`).Scan(&variantID); err != nil {
		t.Fatal(err)
	}

	prRepo := pr.NewRepo(db)
	prRec := &pr.PersonalRecord{UserID: u.ID, ExerciseVariantID: variantID, PRType: "weight", Weight: 135}
	if err := prRepo.Create(ctx, prRec); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM personal_records WHERE id = $1", prRec.ID) })
	if err := prRepo.UpdateImageURL(ctx, prRec.ID, "pr/u/"+prRec.ID.String()+".png"); err != nil {
		t.Fatal(err)
	}

	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: u.GoogleID}}
	mux := http.NewServeMux()
	mux.Handle("GET /prs/{id}/image", auth.RequireAuth(verifier, userRepo, "aud", nil)(http.HandlerFunc(PRImage(prRepo, nil))))

	req := httptest.NewRequest(http.MethodGet, "/prs/"+prRec.ID.String()+"/image", nil)
	req.Header.Set("Authorization", "Bearer x")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("got status %d, want 503 (R2 not configured): %s", rec.Code, rec.Body.String())
	}
}
