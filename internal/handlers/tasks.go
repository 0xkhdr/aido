package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"aido/internal/db"
)

// updateTask handles PATCH /api/projects/:projectId/tasks/:taskId to edit a task.
// Request body: { "title": "...", "description": "...", "completed": false }
// Returns 200 with updated task JSON, 400 for empty title, 404 for missing task.
func (h *Handler) updateTask(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseID(w, r.PathValue("projectId"))
	if !ok {
		return
	}
	taskID, ok := parseID(w, r.PathValue("taskId"))
	if !ok {
		return
	}

	// Verify project exists
	active, ok := h.lookupProject(w, projectID)
	if !ok {
		return
	}

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Completed   bool   `json:"completed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
		return
	}

	// Update task atomically
	task, err := h.store.UpdateTask(taskID, req.Title, req.Completed)
	switch {
	case errors.Is(err, db.ErrEmptyName):
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Task title cannot be empty"})
		return
	case errors.Is(err, db.ErrNoProject):
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Task not found"})
		return
	case err != nil:
		httpErr(w, err)
		return
	}

	// Verify task belongs to the requested project
	if task.ProjectID != active.ID {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Task not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(task)
}
