package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"aido/internal/db"
)

// deleteProject removes a project and all its tasks atomically (R2.1).
// On success, returns 200 with { "success": true }.
// If project not found, returns 404 with { "error": "Project not found" }.
func (h *Handler) deleteProject(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}

	_, err := h.store.DeleteProject(id)
	switch {
	case errors.Is(err, db.ErrNoProject):
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Project not found"})
		return
	case err != nil:
		httpErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// updateProject handles PATCH /api/projects/{id} to rename a project.
// Returns 200 with updated project JSON, 400 for empty name, 404 for missing project.
func (h *Handler) updateProject(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
		return
	}

	p, err := h.store.RenameProject(id, req.Name)
	switch {
	case errors.Is(err, db.ErrEmptyName):
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Project name cannot be empty"})
		return
	case errors.Is(err, db.ErrNoProject):
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Project not found"})
		return
	case err != nil:
		httpErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(p)
}
