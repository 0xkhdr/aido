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
