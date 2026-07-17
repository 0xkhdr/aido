package handlers

import (
	"embed"
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

// New builds a Handler with parsed templates.
func New(store *db.Store) *Handler {
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/*.html"))
	return &Handler{store: store, tmpl: tmpl}
}

// Routes returns the app mux.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", h.index)
	mux.HandleFunc("POST /tasks", h.add)
	mux.HandleFunc("POST /tasks/{id}/toggle", h.toggle)
	mux.HandleFunc("DELETE /tasks/{id}", h.delete)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.store.ListTasks()
	if err != nil {
		httpErr(w, err)
		return
	}
	h.render(w, "index.html", tasks)
}

func (h *Handler) add(w http.ResponseWriter, r *http.Request) {
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		http.Error(w, "title required", http.StatusBadRequest)
		return
	}
	if _, err := h.store.AddTask(title); err != nil {
		httpErr(w, err)
		return
	}
	h.renderList(w)
}

func (h *Handler) toggle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.store.ToggleTask(id); err != nil {
		httpErr(w, err)
		return
	}
	h.renderList(w)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteTask(id); err != nil {
		httpErr(w, err)
		return
	}
	h.renderList(w)
}

// renderList returns the #task-list fragment (HTMX swap target).
func (h *Handler) renderList(w http.ResponseWriter) {
	tasks, err := h.store.ListTasks()
	if err != nil {
		httpErr(w, err)
		return
	}
	h.render(w, "list.html", tasks)
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
