package admin

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/api/idtoken"

	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/chatmessages"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/notes"
	"github.com/jpfortier/gym-app/internal/pr"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/testutil"
	"github.com/jpfortier/gym-app/internal/usage"
	"github.com/jpfortier/gym-app/internal/user"
)

func dbForTest(t *testing.T) *sql.DB { return testutil.DBForTest(t) }

type mockVerifier struct {
	payload *idtoken.Payload
}

func (m *mockVerifier) Verify(_ context.Context, _, _ string) (*idtoken.Payload, error) {
	return m.payload, nil
}

// setupAdminTest creates admin user, handler, mux. Returns cleanup func. Caller may add more users.
func setupAdminTest(t *testing.T) (db *sql.DB, adminUser *user.User, h *Handler, mux *http.ServeMux, cleanup func()) {
	t.Helper()
	db = dbForTest(t)
	ctx := context.Background()

	userRepo := user.NewRepo(db)
	adminUser = &user.User{GoogleID: "admin-" + uuid.New().String(), Email: "admin-" + uuid.New().String() + "@test.com", Name: "Admin"}
	if err := userRepo.Create(ctx, adminUser); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE users SET role = 'admin' WHERE id = $1", adminUser.ID); err != nil {
		t.Fatal(err)
	}

	tpl, err := LoadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	h = &Handler{
		UserRepo:          userRepo,
		SessionRepo:       session.NewRepo(db),
		LogentryRepo:      logentry.NewRepo(db),
		ExerciseRepo:     exercise.NewRepo(db),
		PrRepo:            pr.NewRepo(db),
		UsageRepo:         usage.NewRepo(db),
		NotesRepo:         notes.NewRepo(db),
		ChatMessagesRepo:  chatmessages.NewRepo(db),
		Templates:         tpl,
	}

	verifier := &mockVerifier{payload: &idtoken.Payload{Subject: adminUser.GoogleID}}
	requireAdmin := auth.RequireAdmin(verifier, userRepo, "aud")
	adminWithCookie := InjectAuthCookie(requireAdmin)

	mux = http.NewServeMux()
	mux.Handle("GET /admin", adminWithCookie(http.HandlerFunc(h.Dashboard)))
	mux.Handle("GET /admin/users", adminWithCookie(http.HandlerFunc(h.Users)))
	mux.Handle("GET /admin/sessions", adminWithCookie(http.HandlerFunc(h.Sessions)))
	mux.Handle("GET /admin/sessions/{id}", adminWithCookie(http.HandlerFunc(h.SessionDetail)))
	mux.Handle("GET /admin/prs", adminWithCookie(http.HandlerFunc(h.PRs)))
	mux.Handle("GET /admin/usage", adminWithCookie(http.HandlerFunc(h.Usage)))
	mux.Handle("GET /admin/notes", adminWithCookie(http.HandlerFunc(h.Notes)))
	mux.Handle("POST /admin/select-user", adminWithCookie(http.HandlerFunc(h.SelectUser)))

	cleanup = func() {
		_, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", adminUser.ID)
		db.Close()
	}
	return db, adminUser, h, mux, cleanup
}

func authReq(r *http.Request, token string) *http.Request {
	r.Header.Set("Authorization", "Bearer "+token)
	return r
}

func withCookie(r *http.Request, name, value string) *http.Request {
	r.AddCookie(&http.Cookie{Name: name, Value: value})
	return r
}

func TestAdmin_Login_GET_returnsForm(t *testing.T) {
	tpl, err := LoadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{Templates: tpl}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/login", h.Login)

	req := httptest.NewRequest(http.MethodGet, "/admin/login", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Admin Login") {
		t.Errorf("expected login page, got: %s", body[:min(200, len(body))])
	}
	if !strings.Contains(body, `name="token"`) {
		t.Error("expected token input field")
	}
}

func TestAdmin_Login_POST_emptyToken_redirectsToLogin(t *testing.T) {
	tpl, err := LoadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{Templates: tpl}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/login", h.Login)

	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader("token="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("got status %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin/login" {
		t.Errorf("got Location %q, want /admin/login", loc)
	}
}

func TestAdmin_Login_POST_withToken_setsCookieAndRedirects(t *testing.T) {
	tpl, err := LoadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{Templates: tpl}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/login", h.Login)

	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader("token=dev:test@example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("got status %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin" {
		t.Errorf("got Location %q, want /admin", loc)
	}
	cookies := rec.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == authTokenCookie && c.Value != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected auth cookie to be set")
	}
}

func TestAdmin_Dashboard_noSelectedUser_showsPickUser(t *testing.T) {
	_, adminUser, _, mux, cleanup := setupAdminTest(t)
	_ = adminUser
	defer cleanup()

	req := authReq(httptest.NewRequest(http.MethodGet, "/admin", nil), "x")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Pick a user") && !strings.Contains(body, "pick") {
		t.Error("expected pick user message")
	}
	if !strings.Contains(body, adminUser.Email) {
		t.Error("expected admin email in response")
	}
}

func TestAdmin_Dashboard_withSelectedUser_showsData(t *testing.T) {
	db, _, _, mux, cleanup := setupAdminTest(t)
	defer cleanup()
	ctx := context.Background()

	targetUser := &user.User{GoogleID: "target-" + uuid.New().String(), Email: "target-" + uuid.New().String() + "@test.com", Name: "Target"}
	if err := user.NewRepo(db).Create(ctx, targetUser); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", targetUser.ID) }()

	sessionRepo := session.NewRepo(db)
	parsed, _ := time.Parse("2006-01-02", "2025-03-20")
	sess := &session.Session{UserID: targetUser.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}

	req := authReq(httptest.NewRequest(http.MethodGet, "/admin", nil), "x")
	req = withCookie(req, selectedUserCookie, targetUser.ID.String())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "2025-03-20") {
		t.Error("expected session date in response")
	}
}

func TestAdmin_SelectUser_setsCookieAndRedirects(t *testing.T) {
	db, _, _, mux, cleanup := setupAdminTest(t)
	defer cleanup()
	ctx := context.Background()

	targetUser := &user.User{GoogleID: "target-sel-" + uuid.New().String(), Email: "targetsel-" + uuid.New().String() + "@test.com", Name: "Target"}
	if err := user.NewRepo(db).Create(ctx, targetUser); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", targetUser.ID) }()

	body := "user_id=" + targetUser.ID.String() + "&redirect=/admin/sessions"
	req := authReq(httptest.NewRequest(http.MethodPost, "/admin/select-user", strings.NewReader(body)), "x")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("got status %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin/sessions" {
		t.Errorf("got Location %q, want /admin/sessions", loc)
	}
	cookies := rec.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == selectedUserCookie && c.Value == targetUser.ID.String() {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected gym_admin_selected_user cookie to be set")
	}
}

func TestAdmin_SelectUser_emptyUserID_clearsCookie(t *testing.T) {
	_, _, _, mux, cleanup := setupAdminTest(t)
	defer cleanup()

	req := authReq(httptest.NewRequest(http.MethodPost, "/admin/select-user", strings.NewReader("user_id=&redirect=/admin")), "x")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("got status %d, want 302", rec.Code)
	}
	cookies := rec.Result().Cookies()
	for _, c := range cookies {
		if c.Name == selectedUserCookie && c.MaxAge == -1 {
			return
		}
	}
	t.Error("expected selected user cookie to be cleared (MaxAge -1)")
}

func TestAdmin_Users_returnsUserList(t *testing.T) {
	db, adminUser, _, mux, cleanup := setupAdminTest(t)
	_ = adminUser
	defer cleanup()
	ctx := context.Background()

	otherUser := &user.User{GoogleID: "other-" + uuid.New().String(), Email: "other-" + uuid.New().String() + "@test.com", Name: "Other"}
	if err := user.NewRepo(db).Create(ctx, otherUser); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", otherUser.ID) }()

	req := authReq(httptest.NewRequest(http.MethodGet, "/admin/users", nil), "x")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, adminUser.Email) || !strings.Contains(body, otherUser.Email) {
		t.Errorf("expected both users in list, got: %s", body[:min(200, len(body))])
	}
}

func TestAdmin_Sessions_noSelectedUser_showsPickUser(t *testing.T) {
	_, _, _, mux, cleanup := setupAdminTest(t)
	defer cleanup()

	req := authReq(httptest.NewRequest(http.MethodGet, "/admin/sessions", nil), "x")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "pick") {
		t.Error("expected pick user message")
	}
}

func TestAdmin_Sessions_withSelectedUser_showsSessions(t *testing.T) {
	db, _, _, mux, cleanup := setupAdminTest(t)
	defer cleanup()
	ctx := context.Background()

	targetUser := &user.User{GoogleID: "target-sess-" + uuid.New().String(), Email: "targetsess-" + uuid.New().String() + "@test.com", Name: "Target"}
	if err := user.NewRepo(db).Create(ctx, targetUser); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", targetUser.ID) }()

	sessionRepo := session.NewRepo(db)
	parsed, _ := time.Parse("2006-01-02", "2025-03-21")
	sess := &session.Session{UserID: targetUser.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}

	req := authReq(httptest.NewRequest(http.MethodGet, "/admin/sessions", nil), "x")
	req = withCookie(req, selectedUserCookie, targetUser.ID.String())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "2025-03-21") {
		t.Error("expected session date in response")
	}
}

func TestAdmin_SessionDetail_noSelectedUser_redirectsToSessions(t *testing.T) {
	_, _, _, mux, cleanup := setupAdminTest(t)
	defer cleanup()

	req := authReq(httptest.NewRequest(http.MethodGet, "/admin/sessions/"+uuid.New().String(), nil), "x")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("got status %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin/sessions" {
		t.Errorf("got Location %q, want /admin/sessions", loc)
	}
}

func TestAdmin_SessionDetail_withSelectedUser_showsDetail(t *testing.T) {
	db, _, _, mux, cleanup := setupAdminTest(t)
	defer cleanup()
	ctx := context.Background()

	targetUser := &user.User{GoogleID: "target-detail-" + uuid.New().String(), Email: "targetdetail-" + uuid.New().String() + "@test.com", Name: "Target"}
	if err := user.NewRepo(db).Create(ctx, targetUser); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", targetUser.ID) }()

	var variantID uuid.UUID
	if err := db.QueryRowContext(ctx, `SELECT id FROM exercise_variants WHERE user_id IS NULL LIMIT 1`).Scan(&variantID); err != nil {
		t.Fatal(err)
	}

	sessionRepo := session.NewRepo(db)
	parsed, _ := time.Parse("2006-01-02", "2025-03-22")
	sess := &session.Session{UserID: targetUser.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}

	logentryRepo := logentry.NewRepo(db)
	w := 135.0
	entry := &logentry.LogEntry{SessionID: sess.ID, ExerciseVariantID: variantID, RawSpeech: "bench 135x8"}
	if err := logentryRepo.Create(ctx, entry, []logentry.SetInput{{Weight: &w, Reps: 8, SetOrder: 1}}); err != nil {
		t.Fatal(err)
	}

	req := authReq(httptest.NewRequest(http.MethodGet, "/admin/sessions/"+sess.ID.String(), nil), "x")
	req = withCookie(req, selectedUserCookie, targetUser.ID.String())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "2025-03-22") {
		t.Error("expected session date")
	}
	if !strings.Contains(body, "bench 135x8") {
		t.Error("expected raw speech")
	}
}

func TestAdmin_SessionDetail_otherUserSession_returns404(t *testing.T) {
	db, adminUser, _, mux, cleanup := setupAdminTest(t)
	_ = adminUser
	defer cleanup()
	ctx := context.Background()

	otherUser := &user.User{GoogleID: "other-sess-" + uuid.New().String(), Email: "othersess-" + uuid.New().String() + "@test.com", Name: "Other"}
	if err := user.NewRepo(db).Create(ctx, otherUser); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", otherUser.ID) }()

	sessionRepo := session.NewRepo(db)
	parsed, _ := time.Parse("2006-01-02", "2025-03-23")
	sess := &session.Session{UserID: otherUser.ID, Date: parsed}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}

	req := authReq(httptest.NewRequest(http.MethodGet, "/admin/sessions/"+sess.ID.String(), nil), "x")
	req = withCookie(req, selectedUserCookie, adminUser.ID.String())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want 404", rec.Code)
	}
}

func TestAdmin_PRs_noSelectedUser_showsPickUser(t *testing.T) {
	_, _, _, mux, cleanup := setupAdminTest(t)
	defer cleanup()

	req := authReq(httptest.NewRequest(http.MethodGet, "/admin/prs", nil), "x")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "pick") {
		t.Error("expected pick user message")
	}
}

func TestAdmin_PRs_withSelectedUser_showsPRs(t *testing.T) {
	db, _, _, mux, cleanup := setupAdminTest(t)
	defer cleanup()
	ctx := context.Background()

	targetUser := &user.User{GoogleID: "target-pr-" + uuid.New().String(), Email: "targetpr-" + uuid.New().String() + "@test.com", Name: "Target"}
	if err := user.NewRepo(db).Create(ctx, targetUser); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", targetUser.ID) }()

	var variantID uuid.UUID
	if err := db.QueryRowContext(ctx, `SELECT id FROM exercise_variants WHERE user_id IS NULL LIMIT 1`).Scan(&variantID); err != nil {
		t.Fatal(err)
	}

	prRepo := pr.NewRepo(db)
	reps := 8
	prRec := &pr.PersonalRecord{UserID: targetUser.ID, ExerciseVariantID: variantID, PRType: "weight", Weight: 185, Reps: &reps}
	if err := prRepo.Create(ctx, prRec); err != nil {
		t.Fatal(err)
	}

	req := authReq(httptest.NewRequest(http.MethodGet, "/admin/prs", nil), "x")
	req = withCookie(req, selectedUserCookie, targetUser.ID.String())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "185") {
		t.Error("expected PR weight in response")
	}
}

func TestAdmin_Usage_noSelectedUser_showsPickUser(t *testing.T) {
	_, _, _, mux, cleanup := setupAdminTest(t)
	defer cleanup()

	req := authReq(httptest.NewRequest(http.MethodGet, "/admin/usage", nil), "x")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "pick") {
		t.Error("expected pick user message")
	}
}

func TestAdmin_Usage_withSelectedUser_showsUsage(t *testing.T) {
	db, _, _, mux, cleanup := setupAdminTest(t)
	defer cleanup()
	ctx := context.Background()

	targetUser := &user.User{GoogleID: "target-usage-" + uuid.New().String(), Email: "targetusage-" + uuid.New().String() + "@test.com", Name: "Target"}
	if err := user.NewRepo(db).Create(ctx, targetUser); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", targetUser.ID) }()

	usageRepo := usage.NewRepo(db)
	usageRepo.Record(ctx, &targetUser.ID, "gpt-4o", 100, 50, 0.5)

	req := authReq(httptest.NewRequest(http.MethodGet, "/admin/usage", nil), "x")
	req = withCookie(req, selectedUserCookie, targetUser.ID.String())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "gpt-4o") {
		t.Error("expected model name in response")
	}
}

func TestAdmin_Notes_noSelectedUser_showsPickUser(t *testing.T) {
	_, _, _, mux, cleanup := setupAdminTest(t)
	defer cleanup()

	req := authReq(httptest.NewRequest(http.MethodGet, "/admin/notes", nil), "x")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "pick") {
		t.Error("expected pick user message")
	}
}

func TestAdmin_Notes_withSelectedUser_showsNotes(t *testing.T) {
	db, _, _, mux, cleanup := setupAdminTest(t)
	defer cleanup()
	ctx := context.Background()

	targetUser := &user.User{GoogleID: "target-notes-" + uuid.New().String(), Email: "targetnotes-" + uuid.New().String() + "@test.com", Name: "Target"}
	if err := user.NewRepo(db).Create(ctx, targetUser); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", targetUser.ID) }()

	notesRepo := notes.NewRepo(db)
	if _, err := notesRepo.Create(ctx, targetUser.ID, nil, nil, "warm up hamstrings"); err != nil {
		t.Fatal(err)
	}

	req := authReq(httptest.NewRequest(http.MethodGet, "/admin/notes", nil), "x")
	req = withCookie(req, selectedUserCookie, targetUser.ID.String())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "warm up hamstrings") {
		t.Error("expected note content in response")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
