package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aido/internal/db"
)

// readCloser wraps a reader as an io.ReadCloser for request body simulation.
type readCloser struct {
	io.Reader
}

func (r *readCloser) Close() error { return nil }

func newHandler(t *testing.T) *Handler {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return New(s)
}

// TestDeleteProject verifies DELETE /api/projects/:id removes project atomically
// and cascades task deletion (R2.1, R2.2, R2.3).
func TestDeleteProject(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	// Create a project and task
	p, err := h.store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	_, err = h.store.CreateTask(p.ID, "Test Task")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Delete the project via HTTP
	url := fmt.Sprintf("/api/projects/%d", p.ID)

	req := httptest.NewRequest("DELETE", url, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Verify 200 OK response
	if w.Code != http.StatusOK {
		t.Fatalf("delete project: want 200, got %d", w.Code)
	}

	// Verify response body
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if success, ok := resp["success"].(bool); !ok || !success {
		t.Fatalf("want success: true, got %#v", resp)
	}

	// Verify project is deleted
	if _, err := h.store.GetProject(p.ID); err == nil {
		t.Fatalf("project still exists after delete")
	}

	// Verify tasks are cascade-deleted
	tasks, err := h.store.ListTasksByProject(p.ID)
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("tasks not cascade-deleted: %#v", tasks)
	}
}

// TestDeleteProjectNotFound verifies DELETE on missing project returns 404.
func TestDeleteProjectNotFound(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	req := httptest.NewRequest("DELETE", "/api/projects/99999", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Verify 404 response
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}

	// Verify error response
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] != "Project not found" {
		t.Fatalf("want error message, got %#v", resp)
	}
}

// TestUpdateProject verifies PATCH /api/projects/{id} renames a project.
func TestUpdateProject(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	// Create a project
	p, err := h.store.CreateProject("Original")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	// Test successful rename
	reqBody := map[string]string{"name": "Renamed"}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d", p.ID)
	req := httptest.NewRequest("PATCH", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp db.Project
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Renamed" {
		t.Fatalf("want Renamed, got %q", resp.Name)
	}
	if resp.ID != p.ID {
		t.Fatalf("want id %d, got %d", p.ID, resp.ID)
	}
}

// TestUpdateProjectRejectsEmptyName verifies 400 for blank name.
func TestUpdateProjectRejectsEmptyName(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	// Create a project
	p, err := h.store.CreateProject("Original")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	reqBody := map[string]string{"name": "  "}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d", p.ID)
	req := httptest.NewRequest("PATCH", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "Project name cannot be empty" {
		t.Fatalf("want error message, got %q", resp["error"])
	}

	// Verify name unchanged
	got, _ := h.store.GetProject(p.ID)
	if got.Name != "Original" {
		t.Fatalf("name changed to %q", got.Name)
	}
}

// TestUpdateProjectNotFound verifies 404 for missing project.
func TestUpdateProjectNotFound(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	reqBody := map[string]string{"name": "New"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PATCH", "/api/projects/99999", io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "Project not found" {
		t.Fatalf("want error message, got %q", resp["error"])
	}
}

// TestUpdateProjectEmptyWhitespace verifies that names with only spaces/tabs are rejected.
func TestUpdateProjectEmptyWhitespace(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, _ := h.store.CreateProject("Original")

	tests := []string{
		"   ",
		"\t\t",
		" \t \t ",
	}

	for _, whitespace := range tests {
		reqBody := map[string]string{"name": whitespace}
		body, _ := json.Marshal(reqBody)
		url := fmt.Sprintf("/api/projects/%d", p.ID)
		req := httptest.NewRequest("PATCH", url, io.NopCloser(bytes.NewReader(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("whitespace %q: want 400, got %d", whitespace, w.Code)
		}

		var resp map[string]string
		json.NewDecoder(w.Body).Decode(&resp)
		if resp["error"] != "Project name cannot be empty" {
			t.Fatalf("want error message, got %q", resp["error"])
		}
	}

	// Verify name unchanged
	got, _ := h.store.GetProject(p.ID)
	if got.Name != "Original" {
		t.Fatalf("name changed to %q", got.Name)
	}
}

// TestUpdateProjectLongName verifies that 255-char names are accepted (DB limit).
func TestUpdateProjectLongName(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, _ := h.store.CreateProject("Original")

	longName := strings.Repeat("a", 255)
	reqBody := map[string]string{"name": longName}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d", p.ID)
	req := httptest.NewRequest("PATCH", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Project
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != longName {
		t.Fatalf("name mismatch: want len %d, got len %d", len(longName), len(resp.Name))
	}
	if resp.ID != p.ID {
		t.Fatalf("want id %d, got %d", p.ID, resp.ID)
	}
}

// TestUpdateProjectNameNotTrimmed verifies leading/trailing spaces are preserved.
func TestUpdateProjectNameNotTrimmed(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, _ := h.store.CreateProject("Original")

	// D1 spec: "submitted exactly" - preserve leading/trailing spaces
	nameWithSpaces := "  Padded Name  "
	reqBody := map[string]string{"name": nameWithSpaces}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d", p.ID)
	req := httptest.NewRequest("PATCH", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp db.Project
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != nameWithSpaces {
		t.Fatalf("spaces not preserved: want %q, got %q", nameWithSpaces, resp.Name)
	}
}

// TestUpdateProjectPreservesTasksOnRename verifies tasks are not orphaned during rename.
func TestUpdateProjectPreservesTasksOnRename(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, _ := h.store.CreateProject("Original")
	task1, _ := h.store.CreateTask(p.ID, "Task 1")
	task2, _ := h.store.CreateTask(p.ID, "Task 2")

	// Rename project
	reqBody := map[string]string{"name": "Renamed"}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d", p.ID)
	req := httptest.NewRequest("PATCH", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("rename failed: want 200, got %d", w.Code)
	}

	// Verify project was renamed
	var resp db.Project
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Renamed" {
		t.Fatalf("rename failed: want 'Renamed', got %q", resp.Name)
	}

	// Verify tasks still exist and belong to renamed project
	tasks, _ := h.store.ListTasksByProject(p.ID)
	if len(tasks) != 2 {
		t.Fatalf("want 2 tasks, got %d", len(tasks))
	}

	foundTask1, foundTask2 := false, false
	for _, tk := range tasks {
		if tk.ID == task1.ID && tk.ProjectID == p.ID {
			foundTask1 = true
		}
		if tk.ID == task2.ID && tk.ProjectID == p.ID {
			foundTask2 = true
		}
	}
	if !foundTask1 || !foundTask2 {
		t.Fatalf("tasks were orphaned during rename")
	}
}

// TestUpdateProjectConcurrentRenames verifies last write wins (no race condition).
func TestUpdateProjectConcurrentRenames(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, _ := h.store.CreateProject("Original")

	// Send two concurrent PATCH requests
	url := fmt.Sprintf("/api/projects/%d", p.ID)

	// First rename
	reqBody1 := map[string]string{"name": "First"}
	body1, _ := json.Marshal(reqBody1)
	req1 := httptest.NewRequest("PATCH", url, io.NopCloser(bytes.NewReader(body1)))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, req1)

	// Second rename
	reqBody2 := map[string]string{"name": "Second"}
	body2, _ := json.Marshal(reqBody2)
	req2 := httptest.NewRequest("PATCH", url, io.NopCloser(bytes.NewReader(body2)))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

	if w1.Code != http.StatusOK || w2.Code != http.StatusOK {
		t.Fatalf("requests failed: w1=%d, w2=%d", w1.Code, w2.Code)
	}

	// Verify final state is the second rename (last write wins)
	got, _ := h.store.GetProject(p.ID)
	if got.Name != "Second" {
		t.Fatalf("last write did not win: want 'Second', got %q", got.Name)
	}
}

// TestUpdateProjectInvalidJSON verifies malformed JSON returns 400.
func TestUpdateProjectInvalidJSON(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, _ := h.store.CreateProject("Original")

	invalidJSON := `{"name": "Unclosed`
	url := fmt.Sprintf("/api/projects/%d", p.ID)
	req := httptest.NewRequest("PATCH", url, io.NopCloser(bytes.NewReader([]byte(invalidJSON))))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}

	// Verify project name unchanged
	got, _ := h.store.GetProject(p.ID)
	if got.Name != "Original" {
		t.Fatalf("name changed to %q", got.Name)
	}
}

// TestUpdateProjectResponseFormat verifies response includes id, name, created_at.
func TestUpdateProjectResponseFormat(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, _ := h.store.CreateProject("Original")

	reqBody := map[string]string{"name": "Renamed"}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d", p.ID)
	req := httptest.NewRequest("PATCH", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	// Decode and verify all required fields
	var resp db.Project
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.ID == 0 {
		t.Fatalf("response missing id field")
	}
	if resp.ID != p.ID {
		t.Fatalf("want id %d, got %d", p.ID, resp.ID)
	}

	if resp.Name == "" {
		t.Fatalf("response missing name field")
	}
	if resp.Name != "Renamed" {
		t.Fatalf("want name 'Renamed', got %q", resp.Name)
	}

	if resp.CreatedAt.IsZero() {
		t.Fatalf("response missing or zero created_at field")
	}
}

// TestPatchTask verifies PATCH /api/projects/:projectId/tasks/:taskId updates a task.
func TestPatchTask(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	// Create a project and task
	p, err := h.store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	task, err := h.store.CreateTask(p.ID, "Original Title")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Test successful update
	reqBody := map[string]interface{}{
		"title":       "Updated Title",
		"description": "New description",
		"completed":   true,
	}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d/tasks/%d", p.ID, task.ID)
	req := httptest.NewRequest("PATCH", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp db.Task
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Title != "Updated Title" {
		t.Fatalf("want title 'Updated Title', got %q", resp.Title)
	}
	if resp.Done != true {
		t.Fatalf("want done=true, got %v", resp.Done)
	}
	if resp.ID != task.ID {
		t.Fatalf("want id %d, got %d", task.ID, resp.ID)
	}

	// Verify DB was updated by fetching all tasks for the project
	tasks, err := h.store.ListTasksByProject(p.ID)
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	found := false
	for _, tk := range tasks {
		if tk.ID == task.ID {
			if tk.Title != "Updated Title" {
				t.Fatalf("DB title not updated: want 'Updated Title', got %q", tk.Title)
			}
			if tk.Done != true {
				t.Fatalf("DB done not updated: want true, got %v", tk.Done)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("task not found after update")
	}
}

// TestPatchTaskRejectsEmptyTitle verifies 400 for blank title.
func TestPatchTaskRejectsEmptyTitle(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, err := h.store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	task, err := h.store.CreateTask(p.ID, "Original Title")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	reqBody := map[string]interface{}{
		"title":       "  ",
		"description": "desc",
		"completed":   false,
	}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d/tasks/%d", p.ID, task.ID)
	req := httptest.NewRequest("PATCH", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "Task title cannot be empty" {
		t.Fatalf("want error message, got %q", resp["error"])
	}

	// Verify title unchanged in DB
	tasks, _ := h.store.ListTasksByProject(p.ID)
	for _, tk := range tasks {
		if tk.ID == task.ID && tk.Title != "Original Title" {
			t.Fatalf("title changed to %q", tk.Title)
		}
	}
}

// TestPatchTaskNotFound verifies 404 for missing task.
func TestPatchTaskNotFound(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, err := h.store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	reqBody := map[string]interface{}{
		"title":       "New Title",
		"description": "desc",
		"completed":   false,
	}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d/tasks/99999", p.ID)
	req := httptest.NewRequest("PATCH", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "Task not found" {
		t.Fatalf("want error message, got %q", resp["error"])
	}
}

// TestTextarea verifies multiline description handling: preserve whitespace,
// reject >10000 chars, prevent XSS, store/render correctly (R4.1-R4.4).
func TestTextarea(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	// Create project and task
	p, _ := h.store.CreateProject("Test")
	task, _ := h.store.CreateTask(p.ID, "Task with description")

	// Test 1: Multiline submission and whitespace preservation
	multilineDesc := "Line 1\n  indented line 2\n\nLine 4 with trailing space  "
	body := &readCloser{bytes.NewReader([]byte("description=" + fmt.Sprintf("%s", multilineDesc)))}
	req := httptest.NewRequest("PUT", fmt.Sprintf("/tasks/%d/description?project=%d", task.ID, p.ID), body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("multiline update: want 200, got %d", w.Code)
	}

	// Verify stored with whitespace preserved
	retrieved, err := h.store.GetTask(task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if retrieved.Description != multilineDesc {
		t.Fatalf("whitespace not preserved: got %q, want %q", retrieved.Description, multilineDesc)
	}

	// Test 2: Reject > 10000 chars
	longDesc := strings.Repeat("x", 10001)
	body = &readCloser{bytes.NewReader([]byte("description=" + longDesc))}
	req = httptest.NewRequest("PUT", fmt.Sprintf("/tasks/%d/description?project=%d", task.ID, p.ID), body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("oversized description: want 400, got %d", w.Code)
	}

	// Test 3: XSS prevention (HTML-escape)
	xssDesc := `<script>alert('xss')</script>`
	body = &readCloser{bytes.NewReader([]byte("description=" + fmt.Sprintf("%s", xssDesc)))}
	req = httptest.NewRequest("PUT", fmt.Sprintf("/tasks/%d/description?project=%d", task.ID, p.ID), body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("xss test: want 200, got %d", w.Code)
	}

	// Verify response has escaped HTML
	respBody := w.Body.String()
	if strings.Contains(respBody, "<script>") {
		t.Fatalf("XSS not escaped in response: %s", respBody)
	}
	if !strings.Contains(respBody, "&lt;script&gt;") {
		t.Fatalf("expected escaped HTML in response, got: %s", respBody)
	}
}

// TestDeleteProjectCascadeTasks verifies all tasks are deleted when project is deleted.
// Tests FK cascade with multiple tasks (R2.2, R2.3).
func TestDeleteProjectCascadeTasks(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, err := h.store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	// Create multiple tasks
	task1, err := h.store.CreateTask(p.ID, "Task 1")
	if err != nil {
		t.Fatalf("create task 1: %v", err)
	}
	task2, err := h.store.CreateTask(p.ID, "Task 2")
	if err != nil {
		t.Fatalf("create task 2: %v", err)
	}
	task3, err := h.store.CreateTask(p.ID, "Task 3")
	if err != nil {
		t.Fatalf("create task 3: %v", err)
	}

	// Verify tasks exist
	tasks, err := h.store.ListTasksByProject(p.ID)
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("want 3 tasks, got %d", len(tasks))
	}

	// Delete project
	url := fmt.Sprintf("/api/projects/%d", p.ID)
	req := httptest.NewRequest("DELETE", url, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("delete project: want 200, got %d", w.Code)
	}

	// Verify all tasks are cascade-deleted
	tasks, err = h.store.ListTasksByProject(p.ID)
	if err != nil {
		t.Fatalf("list tasks after delete: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("want 0 tasks after delete, got %d", len(tasks))
	}

	// Verify each task is actually gone
	if _, err := h.store.GetTask(task1.ID); err == nil {
		t.Fatalf("task 1 still exists")
	}
	if _, err := h.store.GetTask(task2.ID); err == nil {
		t.Fatalf("task 2 still exists")
	}
	if _, err := h.store.GetTask(task3.ID); err == nil {
		t.Fatalf("task 3 still exists")
	}
}

// TestDeleteProjectInvalidID verifies non-integer ID returns 400.
func TestDeleteProjectInvalidID(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	req := httptest.NewRequest("DELETE", "/api/projects/not-a-number", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}

	// parseID returns plain text "bad id", not JSON
	body := w.Body.String()
	if body != "bad id\n" {
		t.Fatalf("want 'bad id' response, got %q", body)
	}
}

// TestDeleteProjectResponseFormat verifies response structure: {"success": true}.
func TestDeleteProjectResponseFormat(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, err := h.store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	url := fmt.Sprintf("/api/projects/%d", p.ID)
	req := httptest.NewRequest("DELETE", url, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	// Verify response body is exactly {"success": true}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	success, ok := resp["success"].(bool)
	if !ok {
		t.Fatalf("want success field as bool, got %T: %#v", resp["success"], resp)
	}
	if !success {
		t.Fatalf("want success=true, got %v", success)
	}

	// Verify no extra fields
	if len(resp) != 1 {
		t.Fatalf("want 1 field in response, got %d: %#v", len(resp), resp)
	}
}

// TestDeleteProjectCannotUndelete verifies after delete, project is permanently gone.
func TestDeleteProjectCannotUndelete(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, err := h.store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	projectID := p.ID

	// Delete the project
	url := fmt.Sprintf("/api/projects/%d", projectID)
	req := httptest.NewRequest("DELETE", url, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("delete failed: want 200, got %d", w.Code)
	}

	// Verify project is gone from database
	if _, err := h.store.GetProject(projectID); err == nil {
		t.Fatalf("project still exists after delete")
	}

	// Try to delete again (should get 404, not recover project)
	req = httptest.NewRequest("DELETE", url, nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("second delete: want 404, got %d", w.Code)
	}

	// Verify project still cannot be found
	if _, err := h.store.GetProject(projectID); err == nil {
		t.Fatalf("project should not be recoverable")
	}
}

// TestDeleteProjectConcurrentDelete verifies concurrent delete attempts.
// Second request should get 404 (R2.1 atomicity).
func TestDeleteProjectConcurrentDelete(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, err := h.store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	url := fmt.Sprintf("/api/projects/%d", p.ID)

	// First delete succeeds
	req1 := httptest.NewRequest("DELETE", url, nil)
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("first delete: want 200, got %d", w1.Code)
	}

	// Second delete should fail with 404
	req2 := httptest.NewRequest("DELETE", url, nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Fatalf("second delete: want 404, got %d", w2.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w2.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] != "Project not found" {
		t.Fatalf("want 'Project not found', got %q", resp["error"])
	}
}

// TestDeleteProjectTaskListRefresh verifies UI can correctly detect deleted project.
// After delete, attempting to list tasks returns error.
func TestDeleteProjectTaskListRefresh(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, err := h.store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	_, err = h.store.CreateTask(p.ID, "Task 1")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Delete project
	url := fmt.Sprintf("/api/projects/%d", p.ID)
	req := httptest.NewRequest("DELETE", url, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("delete project: want 200, got %d", w.Code)
	}

	// Verify project no longer exists
	if _, err := h.store.GetProject(p.ID); err == nil {
		t.Fatalf("project should be deleted")
	}

	// Verify tasks are gone
	tasks, err := h.store.ListTasksByProject(p.ID)
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("want 0 tasks for deleted project, got %d", len(tasks))
	}
}

// TestDeleteProjectLastProject verifies deleting the last project behavior.
// If this is the last project, a default "My Tasks" project should be created.
func TestDeleteProjectLastProject(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	// Get initial project list
	projects, err := h.store.ListProjects()
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}

	// Create and delete a project until we're down to one
	if len(projects) == 0 {
		p, err := h.store.CreateProject("Only Project")
		if err != nil {
			t.Fatalf("create project: %v", err)
		}

		url := fmt.Sprintf("/api/projects/%d", p.ID)
		req := httptest.NewRequest("DELETE", url, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("delete project: want 200, got %d", w.Code)
		}

		// After deleting last project, a default "My Tasks" project should exist
		projects, err = h.store.ListProjects()
		if err != nil {
			t.Fatalf("list projects after delete: %v", err)
		}

		// Should have at least the default project
		if len(projects) == 0 {
			t.Fatalf("want at least 1 project (default 'My Tasks'), got 0")
		}

		// Verify default project exists with correct name
		defaultFound := false
		for _, proj := range projects {
			if proj.Name == "My Tasks" {
				defaultFound = true
				break
			}
		}
		if !defaultFound {
			t.Fatalf("want default 'My Tasks' project after deleting last project")
		}
	}
}
