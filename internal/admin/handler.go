package admin

import (
	"context"
	"html/template"
	"net/http"

	"github.com/google/uuid"

	"github.com/jpfortier/gym-app/internal/auth"
	"github.com/jpfortier/gym-app/internal/env"
	"github.com/jpfortier/gym-app/internal/chatmessages"
	"github.com/jpfortier/gym-app/internal/exercise"
	"github.com/jpfortier/gym-app/internal/logentry"
	"github.com/jpfortier/gym-app/internal/notes"
	"github.com/jpfortier/gym-app/internal/pr"
	"github.com/jpfortier/gym-app/internal/session"
	"github.com/jpfortier/gym-app/internal/usage"
	"github.com/jpfortier/gym-app/internal/user"
)

func resolveExerciseNames(ctx context.Context, repo *exercise.Repo, variantID uuid.UUID) (catName, varName string) {
	v, err := repo.GetVariantByID(ctx, variantID)
	if err != nil || v == nil {
		return "", variantID.String()
	}
	varName = v.Name
	if cat, err := repo.GetCategoryByID(ctx, v.CategoryID); err == nil && cat != nil {
		catName = cat.Name
	}
	return catName, varName
}

// Handler holds dependencies for admin handlers.
type Handler struct {
	UserRepo         *user.Repo
	SessionRepo      *session.Repo
	LogentryRepo     *logentry.Repo
	ExerciseRepo     *exercise.Repo
	PrRepo           *pr.Repo
	UsageRepo        *usage.Repo
	NotesRepo        *notes.Repo
	ChatMessagesRepo *chatmessages.Repo
	Templates        *template.Template
}

// LayoutData is passed to every template.
type LayoutData struct {
	CurrentUser  *user.User
	SelectedUser *user.User
	Users        []*user.User
	Title        string
	NavActive    string
	Page         string // template name for content block
	RequiresUser bool
	FlashMessage string
	Data         map[string]interface{}
}

func (h *Handler) layoutData(r *http.Request, title, navActive, page string, requiresUser bool) (*LayoutData, error) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		return &LayoutData{Title: title, NavActive: navActive, Page: page, RequiresUser: requiresUser}, nil
	}
	users, err := h.UserRepo.List(r.Context(), 200)
	if err != nil {
		return nil, err
	}
	selectedID := ReadSelectedUser(r)
	var selectedUser *user.User
	if selectedID != nil {
		for _, uu := range users {
			if uu.ID == *selectedID {
				selectedUser = uu
				break
			}
		}
	}
	return &LayoutData{
		CurrentUser:  u,
		SelectedUser: selectedUser,
		Users:        users,
		Title:        title,
		NavActive:    navActive,
		Page:         page,
		RequiresUser: requiresUser,
	}, nil
}

// SelectUser handles POST /admin/select-user.
func (h *Handler) SelectUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin", http.StatusFound)
		return
	}
	userID := r.FormValue("user_id")
	if userID != "" {
		if _, err := uuid.Parse(userID); err != nil {
			http.Redirect(w, r, "/admin", http.StatusFound)
			return
		}
	}
	SetSelectedUserCookie(w, userID)
	redirect := r.FormValue("redirect")
	if redirect == "" {
		redirect = "/admin"
	}
	http.Redirect(w, r, redirect, http.StatusFound)
}

// Dashboard handles GET /admin.
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	ld, err := h.layoutData(r, "Admin Dashboard", "dashboard", "dashboard", true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ld.RequiresUser && ld.SelectedUser == nil {
		ld.Data = map[string]interface{}{}
		if err := h.Templates.ExecuteTemplate(w, "layout", ld); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if ld.SelectedUser != nil {
		sessions, _ := h.SessionRepo.ListByUser(r.Context(), ld.SelectedUser.ID, 10)
		prs, _ := h.PrRepo.ListByUser(r.Context(), ld.SelectedUser.ID)
		ld.Data = map[string]interface{}{
			"Sessions": sessions,
			"PRs":      prs,
		}
	} else {
		ld.Data = map[string]interface{}{}
	}
	if err := h.Templates.ExecuteTemplate(w, "layout", ld); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Users handles GET /admin/users.
func (h *Handler) Users(w http.ResponseWriter, r *http.Request) {
	ld, err := h.layoutData(r, "Users", "users", "users", false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ld.Data = map[string]interface{}{"Users": ld.Users}
	if err := h.Templates.ExecuteTemplate(w, "layout", ld); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Sessions handles GET /admin/sessions.
func (h *Handler) Sessions(w http.ResponseWriter, r *http.Request) {
	ld, err := h.layoutData(r, "Sessions", "sessions", "sessions", true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ld.SelectedUser == nil {
		ld.Data = map[string]interface{}{}
		if err := h.Templates.ExecuteTemplate(w, "layout", ld); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	sessions, err := h.SessionRepo.ListByUser(r.Context(), ld.SelectedUser.ID, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ld.Data = map[string]interface{}{"Sessions": sessions}
	if err := h.Templates.ExecuteTemplate(w, "layout", ld); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// SessionDetail handles GET /admin/sessions/{id}.
func (h *Handler) SessionDetail(w http.ResponseWriter, r *http.Request) {
	ld, err := h.layoutData(r, "Session Detail", "sessions", "session_detail", true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ld.SelectedUser == nil {
		http.Redirect(w, r, "/admin/sessions", http.StatusFound)
		return
	}
	idStr := r.PathValue("id")
	if idStr == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
		return
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}
	sess, err := h.SessionRepo.GetByID(r.Context(), id)
	if err != nil || sess == nil || sess.UserID != ld.SelectedUser.ID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	entries, err := h.LogentryRepo.ListBySession(r.Context(), sess.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	entriesWithNames := make([]map[string]interface{}, len(entries))
	for i, e := range entries {
		catName, varName := resolveExerciseNames(r.Context(), h.ExerciseRepo, e.ExerciseVariantID)
		entriesWithNames[i] = map[string]interface{}{
			"Entry":        e,
			"CategoryName": catName,
			"VariantName":  varName,
		}
	}
	ld.Data = map[string]interface{}{
		"Session": sess,
		"Entries": entriesWithNames,
	}
	if err := h.Templates.ExecuteTemplate(w, "layout", ld); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// PRs handles GET /admin/prs.
func (h *Handler) PRs(w http.ResponseWriter, r *http.Request) {
	ld, err := h.layoutData(r, "Personal Records", "prs", "prs", true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ld.SelectedUser == nil {
		ld.Data = map[string]interface{}{}
		if err := h.Templates.ExecuteTemplate(w, "layout", ld); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	prs, err := h.PrRepo.ListByUser(r.Context(), ld.SelectedUser.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	prsWithNames := make([]map[string]interface{}, len(prs))
	for i, p := range prs {
		catName, varName := resolveExerciseNames(r.Context(), h.ExerciseRepo, p.ExerciseVariantID)
		prsWithNames[i] = map[string]interface{}{
			"PR":          p,
			"CategoryName": catName,
			"VariantName":  varName,
		}
	}
	ld.Data = map[string]interface{}{"PRs": prsWithNames}
	if err := h.Templates.ExecuteTemplate(w, "layout", ld); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Usage handles GET /admin/usage.
func (h *Handler) Usage(w http.ResponseWriter, r *http.Request) {
	ld, err := h.layoutData(r, "AI Usage", "usage", "usage", true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ld.SelectedUser == nil {
		ld.Data = map[string]interface{}{}
		if err := h.Templates.ExecuteTemplate(w, "layout", ld); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	recs, err := h.UsageRepo.List(r.Context(), &ld.SelectedUser.ID, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ld.Data = map[string]interface{}{"Usage": recs}
	if err := h.Templates.ExecuteTemplate(w, "layout", ld); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Notes handles GET /admin/notes.
func (h *Handler) Notes(w http.ResponseWriter, r *http.Request) {
	ld, err := h.layoutData(r, "Notes", "notes", "notes", true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ld.SelectedUser == nil {
		ld.Data = map[string]interface{}{}
		if err := h.Templates.ExecuteTemplate(w, "layout", ld); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	notesList, err := h.NotesRepo.ListByUser(r.Context(), ld.SelectedUser.ID, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ld.Data = map[string]interface{}{"Notes": notesList}
	if err := h.Templates.ExecuteTemplate(w, "layout", ld); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Login handles GET /admin/login and POST /admin/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		data := map[string]string{"BuildDate": env.BuildDate()}
		for _, name := range []string{"templates/login.html", "login.html"} {
			if h.Templates != nil && h.Templates.Lookup(name) != nil {
				_ = h.Templates.ExecuteTemplate(w, name, data)
				break
			}
		}
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}
		token := r.FormValue("token")
		if token == "" {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}
		SetAuthCookie(w, token)
		http.Redirect(w, r, "/admin", http.StatusFound)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
