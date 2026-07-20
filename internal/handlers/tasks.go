package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

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

// getPriority handles GET /tasks/:id/priority to read a task's priority.
// Returns JSON { priority: "high"|"medium"|"low"|"" }
// Returns 404 if task not found.
func (h *Handler) getPriority(w http.ResponseWriter, r *http.Request) {
	taskID, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}

	priority, err := h.store.GetTaskPriority(taskID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "task not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"priority": priority})
}

// bulkActions handles POST /api/projects/:projectId/bulk-actions.
// Supports actions: mark-done, delete, change-priority.
// Request body: { task_ids: [1,2,3], action: "mark-done"|"delete"|"change-priority", priority?: "high" }
// Returns { success: true, updated: N } for mark-done/change-priority, { success: true, deleted: N } for delete.
// Silently skips tasks that no longer exist.
func (h *Handler) bulkActions(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseID(w, r.PathValue("projectId"))
	if !ok {
		return
	}

	// Verify project exists
	_, ok = h.lookupProject(w, projectID)
	if !ok {
		return
	}

	var req struct {
		TaskIDs []int64 `json:"task_ids"`
		Action  string  `json:"action"`
		Priority string  `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch req.Action {
	case "mark-done":
		count, err := h.store.BulkMarkDone(req.TaskIDs)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{"error": "internal error"})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"success": true, "updated": count})
	case "delete":
		count, err := h.store.BulkDeleteTasks(req.TaskIDs)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{"error": "internal error"})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"success": true, "deleted": count})
	case "change-priority":
		count, err := h.store.BulkUpdateTaskPriority(req.TaskIDs, req.Priority)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{"error": "internal error"})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"success": true, "updated": count})
	default:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "unknown action"})
	}
}

// quickCreateTask handles POST /tasks/quick-create for fast inline task creation.
// Request body: { "title": "Task name", "project_id": 1 }
// Returns 201 with { id, title, done, priority, created_at } on success, 400 on validation error.
func (h *Handler) quickCreateTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title     string `json:"title"`
		ProjectID int64  `json:"project_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	// Validate and trim title
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "title required"})
		return
	}
	if len(req.Title) > 200 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "title exceeds 200 characters"})
		return
	}

	// Create task
	task, err := h.store.CreateTask(req.ProjectID, req.Title)
	switch {
	case errors.Is(err, db.ErrEmptyName):
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "title required"})
		return
	case errors.Is(err, db.ErrNoProject):
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "project not found"})
		return
	case err != nil:
		httpErr(w, err)
		return
	}

	// Set priority to medium
	if err := h.store.SetTaskPriority(task.ID, "medium"); err != nil {
		httpErr(w, err)
		return
	}
	task.Priority = "medium"

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

// searchTasks handles GET /api/projects/:projectId/search?q=keyword.
// Returns { tasks: [ {id, title, done, priority, tags} ] } with matching tasks.
func (h *Handler) searchTasks(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseID(w, r.PathValue("projectId"))
	if !ok {
		return
	}

	// Verify project exists
	_, ok = h.lookupProject(w, projectID)
	if !ok {
		return
	}

	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	if keyword == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"tasks": []db.Task{}})
		return
	}

	tasks, err := h.store.SearchTasksByKeyword(projectID, keyword)
	if err != nil {
		httpErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"tasks": tasks})
}

// addTag handles POST /tasks/:id/tags/add.
// Request body: { "tag_name": "urgent" }
// Returns { success: true, tag_id }
func (h *Handler) addTag(w http.ResponseWriter, r *http.Request) {
	taskID, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}

	var req struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	tagID, err := h.store.AddTag(taskID, req.TagName)
	if err != nil {
		if errors.Is(err, db.ErrEmptyName) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "tag name required"})
			return
		}
		httpErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"success": true, "tag_id": tagID})
}

// removeTag handles DELETE /tasks/:id/tags/:tag_id.
// Returns { success: true }
func (h *Handler) removeTag(w http.ResponseWriter, r *http.Request) {
	taskID, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	tagID, ok := parseID(w, r.PathValue("tag_id"))
	if !ok {
		return
	}

	err := h.store.RemoveTag(taskID, tagID)
	if err != nil {
		httpErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// setDueDate handles POST /tasks/{id}/due-date to set a task's due date.
// Request body: { "due_date": "2026-12-31" } (ISO 8601 format)
// Validates that date is today or in future; rejects past dates with 400.
// Returns 200 with updated task JSON, 400 for invalid date, 404 for missing task.
func (h *Handler) setDueDate(w http.ResponseWriter, r *http.Request) {
	taskID, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}

	var req struct {
		DueDate string `json:"due_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	// Parse and validate due date (must be today or future)
	dueDateParsed, err := time.Parse("2006-01-02", req.DueDate)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid date format; use YYYY-MM-DD"})
		return
	}

	today := time.Now().Truncate(24 * time.Hour)
	if dueDateParsed.Before(today) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "due date must be today or in the future"})
		return
	}

	// Update due date in database
	err = h.store.SetDueDate(taskID, &dueDateParsed)
	if err != nil {
		httpErr(w, err)
		return
	}

	// Fetch and return updated task
	task, err := h.store.GetTask(taskID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "task not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(task)
}

// listTasksByDueDateRange handles GET /api/projects/{projectId}/tasks?due-date-after=...&due-date-before=...
// Filters tasks within a date range and sorts by due_date ASC.
// Query params: due-date-after (inclusive), due-date-before (inclusive) in YYYY-MM-DD format.
// Returns { tasks: [...] } with matching tasks sorted by due date, 404 for missing project.
func (h *Handler) listTasksByDueDateRange(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseID(w, r.PathValue("projectId"))
	if !ok {
		return
	}

	// Verify project exists
	_, ok = h.lookupProject(w, projectID)
	if !ok {
		return
	}

	// Parse query parameters
	afterStr := strings.TrimSpace(r.URL.Query().Get("due-date-after"))
	beforeStr := strings.TrimSpace(r.URL.Query().Get("due-date-before"))

	if afterStr == "" || beforeStr == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "due-date-after and due-date-before required"})
		return
	}

	afterDate, err := time.Parse("2006-01-02", afterStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid due-date-after format; use YYYY-MM-DD"})
		return
	}

	beforeDate, err := time.Parse("2006-01-02", beforeStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid due-date-before format; use YYYY-MM-DD"})
		return
	}

	// Query database
	tasks, err := h.store.ListTasksByDueDateRange(projectID, afterDate, beforeDate)
	if err != nil {
		httpErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"tasks": tasks})
}
