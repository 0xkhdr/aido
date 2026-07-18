package handlers

import (
	"embed"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"aido/internal/db"
)

//go:embed all:templates
var templatesFS embed.FS

// Handler bundles dependencies for HTTP handlers.
type Handler struct {
	store *db.Store
	tmpl  *template.Template
}

// pageData is the view model shared by the full page and every fragment.
type pageData struct {
	Projects []db.Project
	Active   db.Project
	Tasks    []db.Task
}

// New builds a Handler with parsed templates.
func New(store *db.Store) *Handler {
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/*.html"))
	return &Handler{store: store, tmpl: tmpl}
}

// Routes returns the app mux.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", h.home)
	mux.HandleFunc("POST /projects", h.createProject)
	mux.HandleFunc("GET /projects/{id}", h.selectProject)
	mux.HandleFunc("POST /projects/{id}/tasks", h.createTask)
	mux.HandleFunc("POST /tasks/{id}/toggle", h.toggleTask)
	mux.HandleFunc("DELETE /tasks/{id}", h.deleteTask)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

// home renders the full two-pane page with the default project active (R2.3).
func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	projects, err := h.store.ListProjects()
	if err != nil {
		httpErr(w, err)
		return
	}
	if len(projects) == 0 {
		// Defensive: migration guarantees a default, but never render empty.
		if _, err := h.store.EnsureDefaultProject(); err != nil {
			httpErr(w, err)
			return
		}
		if projects, err = h.store.ListProjects(); err != nil {
			httpErr(w, err)
			return
		}
	}

	active := projects[0]
	tasks, err := h.store.ListTasksByProject(active.ID)
	if err != nil {
		httpErr(w, err)
		return
	}
	h.render(w, "index.html", pageData{Projects: projects, Active: active, Tasks: tasks})
}

// createProject creates a project from the sidebar form and swaps #sidebar
// (R1.2, R1.3). A blank name is rejected with no row written.
func (h *Handler) createProject(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	created, err := h.store.CreateProject(name)
	switch {
	case errors.Is(err, db.ErrEmptyName):
		http.Error(w, "project name required", http.StatusBadRequest)
		return
	case err != nil:
		httpErr(w, err)
		return
	}

	projects, err := h.store.ListProjects()
	if err != nil {
		httpErr(w, err)
		return
	}
	// The freshly created project becomes the active/highlighted one.
	h.render(w, "sidebar.html", pageData{Projects: projects, Active: created})
}

// selectProject makes a project active and swaps #main with its tasks (R2.2).
// An unknown or foreign id is rejected and leaves the active project unchanged.
func (h *Handler) selectProject(w http.ResponseWriter, r *http.Request) {
	active, ok := h.projectFromPath(w, r)
	if !ok {
		return
	}
	tasks, err := h.store.ListTasksByProject(active.ID)
	if err != nil {
		httpErr(w, err)
		return
	}
	h.render(w, "main.html", pageData{Active: active, Tasks: tasks})
}

// createTask creates one task in the active project and swaps #task-list
// (R3.2, R3.3, R1.4). Blank text creates nothing; an unknown project is 404.
func (h *Handler) createTask(w http.ResponseWriter, r *http.Request) {
	active, ok := h.projectFromPath(w, r)
	if !ok {
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	_, err := h.store.CreateTask(active.ID, title)
	switch {
	case errors.Is(err, db.ErrEmptyName):
		http.Error(w, "task text required", http.StatusBadRequest)
		return
	case errors.Is(err, db.ErrNoProject):
		http.Error(w, "project not found", http.StatusNotFound)
		return
	case err != nil:
		httpErr(w, err)
		return
	}
	h.renderList(w, active)
}

// toggleTask flips done and re-renders the active project's list (R4.2).
func (h *Handler) toggleTask(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	active, ok := h.projectFromQuery(w, r)
	if !ok {
		return
	}
	if err := h.store.ToggleTask(id); err != nil {
		httpErr(w, err)
		return
	}
	h.renderList(w, active)
}

// deleteTask removes a task and re-renders the active project's list (R4.3).
func (h *Handler) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	active, ok := h.projectFromQuery(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteTask(id); err != nil {
		httpErr(w, err)
		return
	}
	h.renderList(w, active)
}

// projectFromPath resolves the {id} path value to an existing project, writing
// the appropriate error response and returning ok=false when it cannot.
func (h *Handler) projectFromPath(w http.ResponseWriter, r *http.Request) (db.Project, bool) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return db.Project{}, false
	}
	return h.lookupProject(w, id)
}

// projectFromQuery resolves the ?project=<id> query value to an existing
// project (used by task mutations whose route carries only the task id).
func (h *Handler) projectFromQuery(w http.ResponseWriter, r *http.Request) (db.Project, bool) {
	id, ok := parseID(w, r.URL.Query().Get("project"))
	if !ok {
		return db.Project{}, false
	}
	return h.lookupProject(w, id)
}

func (h *Handler) lookupProject(w http.ResponseWriter, id int64) (db.Project, bool) {
	p, err := h.store.GetProject(id)
	switch {
	case errors.Is(err, db.ErrNoProject):
		http.Error(w, "project not found", http.StatusNotFound)
		return db.Project{}, false
	case err != nil:
		httpErr(w, err)
		return db.Project{}, false
	}
	return p, true
}

func parseID(w http.ResponseWriter, raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

// renderList returns the #task-list fragment for one project (HTMX swap).
func (h *Handler) renderList(w http.ResponseWriter, active db.Project) {
	tasks, err := h.store.ListTasksByProject(active.ID)
	if err != nil {
		httpErr(w, err)
		return
	}
	h.render(w, "list.html", pageData{Active: active, Tasks: tasks})
}

func (h *Handler) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
	}
}

func httpErr(w http.ResponseWriter, err error) {
	log.Printf("error: %v", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}
