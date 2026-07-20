package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aido/internal/db"
)

// TestGetTaskPriority verifies GET /tasks/{id}/priority returns task priority.
func TestGetTaskPriority(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	h := New(store)

	// Create project and task
	proj, err := store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	task, err := store.CreateTask(proj.ID, "Test Task")
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Set priority in DB
	if err := store.SetTaskPriority(task.ID, "high"); err != nil {
		t.Fatalf("SetTaskPriority failed: %v", err)
	}

	// Make request through router
	req := httptest.NewRequest("GET", fmt.Sprintf("/tasks/%d/priority", task.ID), nil)
	w := httptest.NewRecorder()

	h.Routes().ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}

	if resp["priority"] != "high" {
		t.Errorf("expected priority 'high', got %q", resp["priority"])
	}
}

// TestUpdateTaskPriority verifies POST /tasks/{id}/priority updates priority.
func TestUpdateTaskPriority(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	h := New(store)

	// Create project and task
	proj, err := store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	task, err := store.CreateTask(proj.ID, "Test Task")
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Make request to update priority through router
	body := []byte(`{"priority":"high"}`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/tasks/%d/priority", task.ID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Routes().ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	// Verify DB persists change
	stored, err := store.GetTaskPriority(task.ID)
	if err != nil {
		t.Fatalf("GetTaskPriority failed: %v", err)
	}

	if stored != "high" {
		t.Errorf("expected DB priority 'high', got %q", stored)
	}
}

// TestInvalidPriority verifies POST /tasks/{id}/priority rejects invalid priority.
func TestInvalidPriority(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	h := New(store)

	// Create project and task
	proj, err := store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	task, err := store.CreateTask(proj.ID, "Test Task")
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Make request with invalid priority through router
	body := []byte(`{"priority":"urgent"}`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/tasks/%d/priority", task.ID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Routes().ServeHTTP(w, req)

	// Verify 400 response
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err == nil {
		if resp["error"] == "" {
			t.Errorf("expected error message in response")
		}
	}
}

// TestBulkUpdatePriority verifies bulk action change-priority works with multiple tasks.
func TestBulkUpdatePriority(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	h := New(store)

	// Create project and tasks
	proj, err := store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	task1, err := store.CreateTask(proj.ID, "Task 1")
	if err != nil {
		t.Fatalf("CreateTask 1 failed: %v", err)
	}

	task2, err := store.CreateTask(proj.ID, "Task 2")
	if err != nil {
		t.Fatalf("CreateTask 2 failed: %v", err)
	}

	// Make bulk action request through the router
	body := []byte(fmt.Sprintf(`{"task_ids":[%d,%d],"action":"change-priority","priority":"low"}`, task1.ID, task2.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/projects/%d/bulk-actions", proj.ID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Routes().ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}

	if !resp["success"].(bool) {
		t.Errorf("expected success=true")
	}

	updated := int(resp["updated"].(float64))
	if updated != 2 {
		t.Errorf("expected 2 tasks updated, got %d", updated)
	}

	// Verify DB changes
	p1, err := store.GetTaskPriority(task1.ID)
	if err != nil {
		t.Fatalf("GetTaskPriority task1 failed: %v", err)
	}

	p2, err := store.GetTaskPriority(task2.ID)
	if err != nil {
		t.Fatalf("GetTaskPriority task2 failed: %v", err)
	}

	if p1 != "low" {
		t.Errorf("task1: expected priority 'low', got %q", p1)
	}

	if p2 != "low" {
		t.Errorf("task2: expected priority 'low', got %q", p2)
	}
}

// TestQuickCreateTask verifies POST /tasks/quick-create creates a task with default priority.
// Request: { title: "Buy milk", project_id: <id> }
// Response: 201 Created with { id, title, done, priority: "medium", created_at }
// Database: task exists with title, done=0, priority='medium'.
func TestQuickCreateTask(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	h := New(store)
	mux := h.Routes()

	// Create a project first
	p, err := store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	// Quick-create a task
	reqBody := map[string]interface{}{
		"title":      "Buy milk",
		"project_id": p.ID,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/tasks/quick-create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Verify 201 Created
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response structure
	var resp db.Task
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify required fields
	if resp.ID == 0 {
		t.Fatalf("response missing id field")
	}
	if resp.Title != "Buy milk" {
		t.Fatalf("want title 'Buy milk', got %q", resp.Title)
	}
	if resp.Done != false {
		t.Fatalf("want done=false, got %v", resp.Done)
	}
	if resp.Priority != "medium" {
		t.Fatalf("want priority 'medium', got %q", resp.Priority)
	}
	if resp.CreatedAt.IsZero() {
		t.Fatalf("response missing or zero created_at field")
	}

	// Verify database: task exists with correct fields
	task, err := store.GetTask(resp.ID)
	if err != nil {
		t.Fatalf("get task from db: %v", err)
	}
	if task.Title != "Buy milk" {
		t.Fatalf("DB: want title 'Buy milk', got %q", task.Title)
	}
	if task.Done != false {
		t.Fatalf("DB: want done=false, got %v", task.Done)
	}
	if task.Priority != "medium" {
		t.Fatalf("DB: want priority 'medium', got %q", task.Priority)
	}
}

// TestQuickCreateEmptyTitle verifies POST /tasks/quick-create rejects empty title.
// Request: { title: "", project_id: <id> }
// Response: 400 Bad Request with error: "title required"
func TestQuickCreateEmptyTitle(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	h := New(store)
	mux := h.Routes()

	// Create a project first
	p, err := store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	// Quick-create with empty title
	reqBody := map[string]interface{}{
		"title":      "",
		"project_id": p.ID,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/tasks/quick-create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Verify 400 Bad Request
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}

	// Verify error message
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] != "title required" {
		t.Fatalf("want error 'title required', got %q", resp["error"])
	}
}

// TestQuickCreateLongTitle verifies POST /tasks/quick-create rejects titles >200 chars.
// Request: { title: (201 chars), project_id: <id> }
// Response: 400 Bad Request with error about length
func TestQuickCreateLongTitle(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	h := New(store)
	mux := h.Routes()

	// Create a project first
	p, err := store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	// Quick-create with title exceeding 200 chars
	longTitle := "a"
	for i := 0; i < 201; i++ {
		longTitle += "a"
	}
	reqBody := map[string]interface{}{
		"title":      longTitle,
		"project_id": p.ID,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/tasks/quick-create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Verify 400 Bad Request
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}

	// Verify error message contains reference to limit
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] == "" {
		t.Fatalf("want error message, got empty")
	}
}

// TestQuickCreateTrimWhitespace verifies POST /tasks/quick-create trims leading/trailing spaces.
// Request: { title: "  Trimmed  ", project_id: <id> }
// Response: 201 Created with title: "Trimmed" (no leading/trailing spaces)
// Database: title stored as "Trimmed"
func TestQuickCreateTrimWhitespace(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	h := New(store)
	mux := h.Routes()

	// Create a project first
	p, err := store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	// Quick-create with whitespace-padded title
	reqBody := map[string]interface{}{
		"title":      "  Trimmed  ",
		"project_id": p.ID,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/tasks/quick-create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Verify 201 Created
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d", w.Code)
	}

	// Verify response has trimmed title
	var resp db.Task
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Title != "Trimmed" {
		t.Fatalf("want title 'Trimmed' (trimmed), got %q", resp.Title)
	}

	// Verify database has trimmed title
	task, err := store.GetTask(resp.ID)
	if err != nil {
		t.Fatalf("get task from db: %v", err)
	}
	if task.Title != "Trimmed" {
		t.Fatalf("DB: want title 'Trimmed', got %q", task.Title)
	}
}

// TestBulkMarkDone verifies POST /api/projects/{id}/bulk-actions with action=mark-done
// marks specified tasks as done and returns correct response count.
func TestBulkMarkDone(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	// Create project and 3 tasks
	p, err := h.store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

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

	// Verify tasks are not done initially
	t1, _ := h.store.GetTask(task1.ID)
	t2, _ := h.store.GetTask(task2.ID)
	t3, _ := h.store.GetTask(task3.ID)
	if t1.Done || t2.Done || t3.Done {
		t.Fatalf("tasks should start as not done")
	}

	// Bulk mark done
	reqBody := map[string]interface{}{
		"action":   "mark-done",
		"task_ids": []int64{task1.ID, task2.ID, task3.ID},
	}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d/bulk-actions", p.ID)
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Verify 200 OK
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response format
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if success, ok := resp["success"].(bool); !ok || !success {
		t.Fatalf("want success: true, got %#v", resp)
	}

	updated, ok := resp["updated"].(float64)
	if !ok || int(updated) != 3 {
		t.Fatalf("want updated: 3, got %#v", resp)
	}

	// Verify all 3 tasks are marked done in DB
	t1, _ = h.store.GetTask(task1.ID)
	t2, _ = h.store.GetTask(task2.ID)
	t3, _ = h.store.GetTask(task3.ID)

	if !t1.Done || !t2.Done || !t3.Done {
		t.Fatalf("tasks not marked done: t1=%v, t2=%v, t3=%v", t1.Done, t2.Done, t3.Done)
	}
}

// TestBulkMarkDoneEmpty verifies bulk-actions with empty task_ids returns 0.
func TestBulkMarkDoneEmpty(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, err := h.store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	reqBody := map[string]interface{}{
		"action":   "mark-done",
		"task_ids": []int64{},
	}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d/bulk-actions", p.ID)
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	updated, ok := resp["updated"].(float64)
	if !ok || int(updated) != 0 {
		t.Fatalf("want updated: 0 for empty list, got %#v", resp)
	}
}

// TestBulkDelete verifies POST /api/projects/{id}/bulk-actions with action=delete
// deletes specified tasks and returns correct count.
func TestBulkDelete(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	// Create project and 3 tasks
	p, err := h.store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

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
	tasks, _ := h.store.ListTasksByProject(p.ID)
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	// Bulk delete
	reqBody := map[string]interface{}{
		"action":   "delete",
		"task_ids": []int64{task1.ID, task2.ID, task3.ID},
	}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d/bulk-actions", p.ID)
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Verify 200 OK
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response format
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if success, ok := resp["success"].(bool); !ok || !success {
		t.Fatalf("want success: true, got %#v", resp)
	}

	deleted, ok := resp["deleted"].(float64)
	if !ok || int(deleted) != 3 {
		t.Fatalf("want deleted: 3, got %#v", resp)
	}

	// Verify all tasks are deleted from DB
	tasks, _ = h.store.ListTasksByProject(p.ID)
	if len(tasks) != 0 {
		t.Fatalf("tasks not deleted: got %d tasks, want 0", len(tasks))
	}

	// Verify each task individually is gone
	if _, err := h.store.GetTask(task1.ID); err == nil {
		t.Fatalf("task 1 still exists after delete")
	}
	if _, err := h.store.GetTask(task2.ID); err == nil {
		t.Fatalf("task 2 still exists after delete")
	}
	if _, err := h.store.GetTask(task3.ID); err == nil {
		t.Fatalf("task 3 still exists after delete")
	}
}

// TestBulkDeleteEmpty verifies delete with empty task_ids returns 0 deleted.
func TestBulkDeleteEmpty(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, err := h.store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	reqBody := map[string]interface{}{
		"action":   "delete",
		"task_ids": []int64{},
	}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d/bulk-actions", p.ID)
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	deleted, ok := resp["deleted"].(float64)
	if !ok || int(deleted) != 0 {
		t.Fatalf("want deleted: 0 for empty list, got %#v", resp)
	}
}

// TestBulkChangePriority verifies POST /api/projects/{id}/bulk-actions with action=change-priority
// updates priority on specified tasks and returns correct count.
func TestBulkChangePriority(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	// Create project and 3 tasks
	p, err := h.store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

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

	// Verify initial priority is empty
	t1, _ := h.store.GetTask(task1.ID)
	if t1.Priority != "" {
		t.Fatalf("task 1 should have empty priority initially, got %q", t1.Priority)
	}

	// Bulk change priority to high
	reqBody := map[string]interface{}{
		"action":   "change-priority",
		"task_ids": []int64{task1.ID, task2.ID, task3.ID},
		"priority": "high",
	}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d/bulk-actions", p.ID)
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Verify 200 OK
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response format
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if success, ok := resp["success"].(bool); !ok || !success {
		t.Fatalf("want success: true, got %#v", resp)
	}

	updated, ok := resp["updated"].(float64)
	if !ok || int(updated) != 3 {
		t.Fatalf("want updated: 3, got %#v", resp)
	}

	// Verify all 3 tasks have priority=high in DB
	t1, _ = h.store.GetTask(task1.ID)
	t2, _ := h.store.GetTask(task2.ID)
	t3, _ := h.store.GetTask(task3.ID)

	if t1.Priority != "high" {
		t.Fatalf("task 1 priority: want 'high', got %q", t1.Priority)
	}
	if t2.Priority != "high" {
		t.Fatalf("task 2 priority: want 'high', got %q", t2.Priority)
	}
	if t3.Priority != "high" {
		t.Fatalf("task 3 priority: want 'high', got %q", t3.Priority)
	}
}

// TestBulkChangePriorityToMedium verifies changing priority to medium.
func TestBulkChangePriorityToMedium(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, _ := h.store.CreateProject("Test Project")
	task1, _ := h.store.CreateTask(p.ID, "Task 1")

	// Set initial priority to high
	h.store.SetTaskPriority(task1.ID, "high")

	// Change to medium
	reqBody := map[string]interface{}{
		"action":   "change-priority",
		"task_ids": []int64{task1.ID},
		"priority": "medium",
	}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d/bulk-actions", p.ID)
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	t1, _ := h.store.GetTask(task1.ID)
	if t1.Priority != "medium" {
		t.Fatalf("priority should be medium, got %q", t1.Priority)
	}
}

// TestBulkChangePriorityToLow verifies changing priority to low.
func TestBulkChangePriorityToLow(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, _ := h.store.CreateProject("Test Project")
	task1, _ := h.store.CreateTask(p.ID, "Task 1")

	// Change to low
	reqBody := map[string]interface{}{
		"action":   "change-priority",
		"task_ids": []int64{task1.ID},
		"priority": "low",
	}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d/bulk-actions", p.ID)
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	t1, _ := h.store.GetTask(task1.ID)
	if t1.Priority != "low" {
		t.Fatalf("priority should be low, got %q", t1.Priority)
	}
}

// TestBulkDeleteSkipsNonexistent verifies that deleting with non-existent IDs
// still succeeds and returns the actual count of deleted tasks, not the input count.
func TestBulkDeleteSkipsNonexistent(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, err := h.store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	task1, err := h.store.CreateTask(p.ID, "Task 1")
	if err != nil {
		t.Fatalf("create task 1: %v", err)
	}
	task2, err := h.store.CreateTask(p.ID, "Task 2")
	if err != nil {
		t.Fatalf("create task 2: %v", err)
	}

	// Try to delete 2 existing tasks + 1 non-existent ID
	reqBody := map[string]interface{}{
		"action":   "delete",
		"task_ids": []int64{task1.ID, task2.ID, 99999},
	}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d/bulk-actions", p.ID)
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Verify 200 OK (not an error)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	// Verify response shows only 2 deleted (not 3)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	deleted, ok := resp["deleted"].(float64)
	if !ok || int(deleted) != 2 {
		t.Fatalf("want deleted: 2, got %#v", resp)
	}

	// Verify only the 2 real tasks are gone
	tasks, _ := h.store.ListTasksByProject(p.ID)
	if len(tasks) != 0 {
		t.Fatalf("want 0 tasks left, got %d", len(tasks))
	}
}

// TestBulkMarkDoneSkipsNonexistent verifies mark-done skips non-existent IDs.
func TestBulkMarkDoneSkipsNonexistent(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, _ := h.store.CreateProject("Test Project")
	task1, _ := h.store.CreateTask(p.ID, "Task 1")
	task2, _ := h.store.CreateTask(p.ID, "Task 2")

	// Mark done with 2 existing + 1 non-existent
	reqBody := map[string]interface{}{
		"action":   "mark-done",
		"task_ids": []int64{task1.ID, task2.ID, 99999},
	}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d/bulk-actions", p.ID)
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	updated, ok := resp["updated"].(float64)
	if !ok || int(updated) != 2 {
		t.Fatalf("want updated: 2, got %#v", resp)
	}

	// Verify both real tasks are marked done
	t1, _ := h.store.GetTask(task1.ID)
	t2, _ := h.store.GetTask(task2.ID)
	if !t1.Done || !t2.Done {
		t.Fatalf("tasks not marked done")
	}
}

// TestBulkChangePrioritySkipsNonexistent verifies change-priority skips non-existent IDs.
func TestBulkChangePrioritySkipsNonexistent(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, _ := h.store.CreateProject("Test Project")
	task1, _ := h.store.CreateTask(p.ID, "Task 1")
	task2, _ := h.store.CreateTask(p.ID, "Task 2")

	// Change priority with 2 existing + 1 non-existent
	reqBody := map[string]interface{}{
		"action":   "change-priority",
		"task_ids": []int64{task1.ID, task2.ID, 99999},
		"priority": "high",
	}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d/bulk-actions", p.ID)
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	updated, ok := resp["updated"].(float64)
	if !ok || int(updated) != 2 {
		t.Fatalf("want updated: 2, got %#v", resp)
	}

	// Verify both real tasks have high priority
	t1, _ := h.store.GetTask(task1.ID)
	t2, _ := h.store.GetTask(task2.ID)
	if t1.Priority != "high" || t2.Priority != "high" {
		t.Fatalf("priorities not updated: t1=%q, t2=%q", t1.Priority, t2.Priority)
	}
}

// TestBulkActionsInvalidProject verifies 404 for non-existent project.
func TestBulkActionsInvalidProject(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	reqBody := map[string]interface{}{
		"action":   "mark-done",
		"task_ids": []int64{1, 2, 3},
	}
	body, _ := json.Marshal(reqBody)
	url := "/api/projects/99999/bulk-actions"
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

// TestBulkActionsUnknownAction verifies 400 for unknown action.
func TestBulkActionsUnknownAction(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, _ := h.store.CreateProject("Test Project")

	reqBody := map[string]interface{}{
		"action":   "unknown-action",
		"task_ids": []int64{1, 2},
	}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d/bulk-actions", p.ID)
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "unknown action" {
		t.Fatalf("want 'unknown action', got %q", resp["error"])
	}
}

// TestBulkActionsInvalidJSON verifies 400 for malformed JSON.
func TestBulkActionsInvalidJSON(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, _ := h.store.CreateProject("Test Project")

	invalidJSON := `{"action": "mark-done", "task_ids": [1, 2`
	url := fmt.Sprintf("/api/projects/%d/bulk-actions", p.ID)
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader([]byte(invalidJSON))))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "invalid request" {
		t.Fatalf("want 'invalid request', got %q", resp["error"])
	}
}

// TestBulkActionsResponseContentType verifies application/json content type.
func TestBulkActionsResponseContentType(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, _ := h.store.CreateProject("Test Project")
	task1, _ := h.store.CreateTask(p.ID, "Task 1")

	reqBody := map[string]interface{}{
		"action":   "mark-done",
		"task_ids": []int64{task1.ID},
	}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d/bulk-actions", p.ID)
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Fatalf("want Content-Type: application/json, got %q", contentType)
	}
}

// TestBulkMarkDonePartial verifies marking only a subset of tasks done.
func TestBulkMarkDonePartial(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, _ := h.store.CreateProject("Test Project")
	task1, _ := h.store.CreateTask(p.ID, "Task 1")
	task2, _ := h.store.CreateTask(p.ID, "Task 2")
	task3, _ := h.store.CreateTask(p.ID, "Task 3")

	// Mark only task 1 and 2 done
	reqBody := map[string]interface{}{
		"action":   "mark-done",
		"task_ids": []int64{task1.ID, task2.ID},
	}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d/bulk-actions", p.ID)
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	// Verify correct count
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	updated := int(resp["updated"].(float64))
	if updated != 2 {
		t.Fatalf("want updated: 2, got %d", updated)
	}

	// Verify states
	t1, _ := h.store.GetTask(task1.ID)
	t2, _ := h.store.GetTask(task2.ID)
	t3, _ := h.store.GetTask(task3.ID)

	if !t1.Done || !t2.Done || t3.Done {
		t.Fatalf("wrong tasks marked done")
	}
}

// TestBulkDeletePartial verifies deleting only a subset of tasks.
func TestBulkDeletePartial(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, _ := h.store.CreateProject("Test Project")
	task1, _ := h.store.CreateTask(p.ID, "Task 1")
	task2, _ := h.store.CreateTask(p.ID, "Task 2")
	task3, _ := h.store.CreateTask(p.ID, "Task 3")

	// Delete only task 1 and 2
	reqBody := map[string]interface{}{
		"action":   "delete",
		"task_ids": []int64{task1.ID, task2.ID},
	}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d/bulk-actions", p.ID)
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	// Verify correct count
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	deleted := int(resp["deleted"].(float64))
	if deleted != 2 {
		t.Fatalf("want deleted: 2, got %d", deleted)
	}

	// Verify only task 3 remains
	tasks, _ := h.store.ListTasksByProject(p.ID)
	if len(tasks) != 1 {
		t.Fatalf("want 1 task remaining, got %d", len(tasks))
	}
	if tasks[0].ID != task3.ID {
		t.Fatalf("wrong task remaining")
	}
}

// TestBulkChangePriorityPartial verifies changing priority on only a subset.
func TestBulkChangePriorityPartial(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	p, _ := h.store.CreateProject("Test Project")
	task1, _ := h.store.CreateTask(p.ID, "Task 1")
	task2, _ := h.store.CreateTask(p.ID, "Task 2")
	task3, _ := h.store.CreateTask(p.ID, "Task 3")

	// Change priority for task 1 and 2 only
	reqBody := map[string]interface{}{
		"action":   "change-priority",
		"task_ids": []int64{task1.ID, task2.ID},
		"priority": "high",
	}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/api/projects/%d/bulk-actions", p.ID)
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	// Verify correct count
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	updated := int(resp["updated"].(float64))
	if updated != 2 {
		t.Fatalf("want updated: 2, got %d", updated)
	}

	// Verify priorities
	t1, _ := h.store.GetTask(task1.ID)
	t2, _ := h.store.GetTask(task2.ID)
	t3, _ := h.store.GetTask(task3.ID)

	if t1.Priority != "high" || t2.Priority != "high" || t3.Priority != "" {
		t.Fatalf("wrong priorities: t1=%q, t2=%q, t3=%q", t1.Priority, t2.Priority, t3.Priority)
	}
}

// setupTestDB creates an in-memory SQLite test database.
func setupTestDB(t *testing.T) (*db.Store, func()) {
	dir, err := os.MkdirTemp("", "aido-test-")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}

	dbPath := filepath.Join(dir, "test.db")
	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}

	if err := store.Migrate(); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	cleanup := func() {
		_ = store.Close()
		_ = os.RemoveAll(dir)
	}

	return store, cleanup
}

// TestSetDueDateFuture verifies POST /tasks/{id}/due-date with future date updates task.
// Should return 200 OK with updated task JSON.
func TestSetDueDateFuture(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	h := New(store)
	mux := h.Routes()

	// Create project and task
	proj, err := store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}
	task, err := store.CreateTask(proj.ID, "Test Task")
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Set future due date
	futureDate := "2027-01-01"
	reqBody := map[string]string{"due_date": futureDate}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", fmt.Sprintf("/tasks/%d/due-date", task.ID), io.NopCloser(bytes.NewReader(body)))
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

	if resp.ID != task.ID {
		t.Fatalf("want task id %d, got %d", task.ID, resp.ID)
	}

	if resp.DueDate == nil {
		t.Fatalf("want DueDate set, got nil")
	}

	expectedDate, _ := time.Parse("2006-01-02", futureDate)
	if resp.DueDate.Format("2006-01-02") != expectedDate.Format("2006-01-02") {
		t.Fatalf("want DueDate %s, got %s", futureDate, resp.DueDate.Format("2006-01-02"))
	}

	// Verify database was updated
	retrieved, err := store.GetTask(task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if retrieved.DueDate == nil {
		t.Fatalf("DB: want DueDate set, got nil")
	}
	if retrieved.DueDate.Format("2006-01-02") != futureDate {
		t.Fatalf("DB: want DueDate %s, got %s", futureDate, retrieved.DueDate.Format("2006-01-02"))
	}
}

// TestSetDueDateToday verifies POST /tasks/{id}/due-date with today's date is allowed.
// Should return 200 OK (today is allowed, not rejected as past).
func TestSetDueDateToday(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	h := New(store)
	mux := h.Routes()

	// Create project and task
	proj, err := store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}
	task, err := store.CreateTask(proj.ID, "Test Task")
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Set today's due date
	today := time.Now().Format("2006-01-02")
	reqBody := map[string]string{"due_date": today}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", fmt.Sprintf("/tasks/%d/due-date", task.ID), io.NopCloser(bytes.NewReader(body)))
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

	if resp.DueDate == nil {
		t.Fatalf("want DueDate set, got nil")
	}

	if resp.DueDate.Format("2006-01-02") != today {
		t.Fatalf("want DueDate %s, got %s", today, resp.DueDate.Format("2006-01-02"))
	}
}

// TestSetDueDatePast verifies POST /tasks/{id}/due-date with past date is rejected.
// Should return 400 Bad Request with error message.
func TestSetDueDatePast(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	h := New(store)
	mux := h.Routes()

	// Create project and task
	proj, err := store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}
	task, err := store.CreateTask(proj.ID, "Test Task")
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Try to set past due date
	pastDate := "2020-01-01"
	reqBody := map[string]string{"due_date": pastDate}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", fmt.Sprintf("/tasks/%d/due-date", task.ID), io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["error"] == "" {
		t.Fatalf("want error message, got empty")
	}

	// Verify task due_date unchanged
	retrieved, err := store.GetTask(task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if retrieved.DueDate != nil {
		t.Fatalf("want DueDate nil, got %v", retrieved.DueDate)
	}
}

// TestListByDueDateRange verifies GET /api/projects/{id}/tasks?due-date-after=...&due-date-before=...
// filters tasks within a date range and sorts by due_date ASC.
func TestListByDueDateRange(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	h := New(store)
	mux := h.Routes()

	// Create project
	proj, err := store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	// Create 3 tasks with different due dates
	task1, err := store.CreateTask(proj.ID, "Task 1")
	if err != nil {
		t.Fatalf("create task 1: %v", err)
	}
	date1, _ := time.Parse("2006-01-02", "2026-07-15")
	if err := store.SetDueDate(task1.ID, &date1); err != nil {
		t.Fatalf("set due date task 1: %v", err)
	}

	task2, err := store.CreateTask(proj.ID, "Task 2")
	if err != nil {
		t.Fatalf("create task 2: %v", err)
	}
	date2, _ := time.Parse("2006-01-02", "2026-07-25")
	if err := store.SetDueDate(task2.ID, &date2); err != nil {
		t.Fatalf("set due date task 2: %v", err)
	}

	task3, err := store.CreateTask(proj.ID, "Task 3")
	if err != nil {
		t.Fatalf("create task 3: %v", err)
	}
	date3, _ := time.Parse("2006-01-02", "2026-08-05")
	if err := store.SetDueDate(task3.ID, &date3); err != nil {
		t.Fatalf("set due date task 3: %v", err)
	}

	// Query tasks in range: 2026-07-20 to 2026-08-01 (should return only task2)
	url := fmt.Sprintf("/api/projects/%d/tasks?due-date-after=2026-07-20&due-date-before=2026-08-01", proj.ID)
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string][]db.Task
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	tasks := resp["tasks"]
	if len(tasks) != 1 {
		t.Fatalf("want 1 task in range, got %d: %#v", len(tasks), tasks)
	}

	if tasks[0].ID != task2.ID {
		t.Fatalf("want task id %d, got %d", task2.ID, tasks[0].ID)
	}

	if tasks[0].DueDate.Format("2006-01-02") != "2026-07-25" {
		t.Fatalf("want due date 2026-07-25, got %s", tasks[0].DueDate.Format("2006-01-02"))
	}
}

// TestListByDueDateRangeMultipleSorted verifies results are sorted ASC by due_date.
func TestListByDueDateRangeMultipleSorted(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	h := New(store)
	mux := h.Routes()

	// Create project
	proj, err := store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	// Create tasks with due dates (unordered)
	dates := []string{"2026-07-25", "2026-07-20", "2026-07-30", "2026-07-22"}
	for i, dateStr := range dates {
		task, err := store.CreateTask(proj.ID, fmt.Sprintf("Task %d", i+1))
		if err != nil {
			t.Fatalf("create task: %v", err)
		}
		dt, _ := time.Parse("2006-01-02", dateStr)
		if err := store.SetDueDate(task.ID, &dt); err != nil {
			t.Fatalf("set due date: %v", err)
		}
	}

	// Query all tasks
	url := fmt.Sprintf("/api/projects/%d/tasks?due-date-after=2026-07-20&due-date-before=2026-07-31", proj.ID)
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp map[string][]db.Task
	json.NewDecoder(w.Body).Decode(&resp)
	tasks := resp["tasks"]

	if len(tasks) != 4 {
		t.Fatalf("want 4 tasks, got %d", len(tasks))
	}

	// Verify sorted ASC by due_date
	expected := []string{"2026-07-20", "2026-07-22", "2026-07-25", "2026-07-30"}
	for i, expectedDate := range expected {
		if tasks[i].DueDate.Format("2006-01-02") != expectedDate {
			t.Fatalf("position %d: want %s, got %s", i, expectedDate, tasks[i].DueDate.Format("2006-01-02"))
		}
	}
}

// TestSearchByKeyword verifies GET /api/projects/{id}/search?q=buy
// returns matching tasks with case-insensitive LIKE matching.
// Creates 3 tasks: "Buy milk", "Sell books", "Buy books"
// Expects 2 matches: "Buy milk", "Buy books"
func TestSearchByKeyword(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	h := New(store)
	mux := h.Routes()

	// Create project and tasks
	p, err := store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	_, _ = store.CreateTask(p.ID, "Buy milk")
	_, _ = store.CreateTask(p.ID, "Sell books")
	_, _ = store.CreateTask(p.ID, "Buy books")

	// Search for "buy" (case-insensitive)
	url := fmt.Sprintf("/api/projects/%d/search?q=buy", p.ID)
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	tasks, ok := resp["tasks"].([]interface{})
	if !ok {
		t.Fatalf("response missing 'tasks' field or wrong type: %#v", resp)
	}

	if len(tasks) != 2 {
		t.Fatalf("want 2 matches, got %d: %#v", len(tasks), tasks)
	}

	// Verify returned tasks are correct (check title field exists)
	taskTitles := make([]string, 0)
	for _, tk := range tasks {
		tm := tk.(map[string]interface{})
		if title, ok := tm["title"].(string); ok {
			taskTitles = append(taskTitles, title)
		}
	}

	if len(taskTitles) != 2 {
		t.Fatalf("unable to extract titles from response: %#v", tasks)
	}

	// Verify both "Buy milk" and "Buy books" are in results
	hasBuyMilk, hasBuyBooks := false, false
	for _, title := range taskTitles {
		if title == "Buy milk" {
			hasBuyMilk = true
		}
		if title == "Buy books" {
			hasBuyBooks = true
		}
	}

	if !hasBuyMilk {
		t.Fatalf("'Buy milk' not found in search results")
	}
	if !hasBuyBooks {
		t.Fatalf("'Buy books' not found in search results")
	}

	// Verify "Sell books" is not in results
	for _, title := range taskTitles {
		if title == "Sell books" {
			t.Fatalf("'Sell books' should not match 'buy' query")
		}
	}
}

// TestSearchNoResults verifies GET /api/projects/{id}/search?q=xyz
// returns 200 OK with empty task array when no matches found.
func TestSearchNoResults(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	h := New(store)
	mux := h.Routes()

	// Create project and tasks
	p, err := store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	_, _ = store.CreateTask(p.ID, "Buy milk")
	_, _ = store.CreateTask(p.ID, "Sell books")

	// Search for non-existent keyword
	url := fmt.Sprintf("/api/projects/%d/search?q=xyz", p.ID)
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	tasks, ok := resp["tasks"].([]interface{})
	if !ok {
		t.Fatalf("response missing 'tasks' field or wrong type: %#v", resp)
	}

	if len(tasks) != 0 {
		t.Fatalf("want 0 results for non-existent keyword, got %d", len(tasks))
	}
}

// TestSearchPartialMatch verifies substring matching.
// Creates task: "JavaScript project"
// GET /api/projects/{id}/search?q=script
// Expects 1 match: "JavaScript project"
func TestSearchPartialMatch(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	h := New(store)
	mux := h.Routes()

	// Create project and task
	p, err := store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	_, _ = store.CreateTask(p.ID, "JavaScript project")

	// Search for substring "script"
	url := fmt.Sprintf("/api/projects/%d/search?q=script", p.ID)
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	tasks, ok := resp["tasks"].([]interface{})
	if !ok {
		t.Fatalf("response missing 'tasks' field or wrong type: %#v", resp)
	}

	if len(tasks) != 1 {
		t.Fatalf("want 1 match for 'script', got %d", len(tasks))
	}

	// Verify task title
	task := tasks[0].(map[string]interface{})
	title, ok := task["title"].(string)
	if !ok || title != "JavaScript project" {
		t.Fatalf("want title 'JavaScript project', got %q", title)
	}
}

// TestSearchEmpty verifies GET /api/projects/{id}/search?q= (empty query)
// returns 200 OK with empty task array.
func TestSearchEmpty(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	h := New(store)
	mux := h.Routes()

	// Create project and tasks
	p, err := store.CreateProject("Test Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	_, _ = store.CreateTask(p.ID, "Task 1")
	_, _ = store.CreateTask(p.ID, "Task 2")

	// Search with empty query parameter
	url := fmt.Sprintf("/api/projects/%d/search?q=", p.ID)
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	tasks, ok := resp["tasks"].([]interface{})
	if !ok {
		t.Fatalf("response missing 'tasks' field or wrong type: %#v", resp)
	}

	if len(tasks) != 0 {
		t.Fatalf("want 0 results for empty query, got %d", len(tasks))
	}
}

// newHandler creates a Handler with a fresh test database.
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

// TestAddTag verifies POST /tasks/{id}/tags/add creates tag and links to task.
// Returns 200 OK with { success: true, tag_id }.
func TestAddTag(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	// Create project and task
	p, _ := h.store.CreateProject("Test")
	task, _ := h.store.CreateTask(p.ID, "Task 1")

	// Add tag
	reqBody := map[string]string{"tag_name": "urgent"}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/tasks/%d/tags/add", task.ID)
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Verify 200 OK
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	// Verify response
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if success, ok := resp["success"].(bool); !ok || !success {
		t.Fatalf("want success: true, got %#v", resp)
	}
	if tagID, ok := resp["tag_id"].(float64); !ok || tagID <= 0 {
		t.Fatalf("want tag_id > 0, got %#v", resp)
	}

	// Verify DB: tag created and junction entry added
	tags, err := h.store.GetTaskTags(task.ID)
	if err != nil {
		t.Fatalf("get task tags: %v", err)
	}
	if len(tags) != 1 || tags[0] != "urgent" {
		t.Fatalf("want tags=[urgent], got %#v", tags)
	}
}

// TestAddTagDuplicate verifies adding same tag twice doesn't create duplicate.
// Second add returns existing tag_id.
func TestAddTagDuplicate(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	// Create project and task
	p, _ := h.store.CreateProject("Test")
	task, _ := h.store.CreateTask(p.ID, "Task 1")

	// Add tag first time
	reqBody := map[string]string{"tag_name": "urgent"}
	body1, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/tasks/%d/tags/add", task.ID)
	req1 := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body1)))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("first add: want 200, got %d", w1.Code)
	}

	var resp1 map[string]interface{}
	json.NewDecoder(w1.Body).Decode(&resp1)
	tagID1 := resp1["tag_id"]

	// Add same tag second time
	body2, _ := json.Marshal(reqBody)
	req2 := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body2)))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("second add: want 200, got %d", w2.Code)
	}

	var resp2 map[string]interface{}
	json.NewDecoder(w2.Body).Decode(&resp2)
	tagID2 := resp2["tag_id"]

	// Verify same tag_id returned
	if tagID1 != tagID2 {
		t.Fatalf("want same tag_id, got %v != %v", tagID1, tagID2)
	}

	// Verify only one tag in junction table
	tags, _ := h.store.GetTaskTags(task.ID)
	if len(tags) != 1 {
		t.Fatalf("want 1 tag, got %d", len(tags))
	}
}

// TestRemoveTag verifies DELETE /tasks/{id}/tags/{tag_id} removes junction entry.
// Task and tag still exist; returns 200 OK with { success: true }.
func TestRemoveTag(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	// Create project, task, and tag
	p, _ := h.store.CreateProject("Test")
	task, _ := h.store.CreateTask(p.ID, "Task 1")
	tagID, _ := h.store.AddTag(task.ID, "urgent")

	// Remove tag
	url := fmt.Sprintf("/tasks/%d/tags/%d", task.ID, tagID)
	req := httptest.NewRequest("DELETE", url, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Verify 200 OK
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	// Verify response
	var resp map[string]bool
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp["success"] {
		t.Fatalf("want success: true, got %#v", resp)
	}

	// Verify junction entry removed
	tags, _ := h.store.GetTaskTags(task.ID)
	if len(tags) != 0 {
		t.Fatalf("want 0 tags after remove, got %d", len(tags))
	}

	// Verify task still exists
	if _, err := h.store.GetTask(task.ID); err != nil {
		t.Fatalf("task should still exist: %v", err)
	}
}

// TestGetTaskTags verifies GetTask returns tags array with all tags.
func TestGetTaskTags(t *testing.T) {
	h := newHandler(t)

	// Create project, task, and 3 tags
	p, _ := h.store.CreateProject("Test")
	task, _ := h.store.CreateTask(p.ID, "Task 1")
	h.store.AddTag(task.ID, "urgent")
	h.store.AddTag(task.ID, "important")
	h.store.AddTag(task.ID, "review")

	// Fetch task
	retrieved, err := h.store.GetTask(task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}

	// Verify tags present (order may vary, so check set membership)
	if len(retrieved.Tags) != 3 {
		t.Fatalf("want 3 tags, got %d: %#v", len(retrieved.Tags), retrieved.Tags)
	}

	tagMap := make(map[string]bool)
	for _, tag := range retrieved.Tags {
		tagMap[tag] = true
	}
	expectedTags := []string{"urgent", "important", "review"}
	for _, expected := range expectedTags {
		if !tagMap[expected] {
			t.Fatalf("want tag %q in %#v", expected, retrieved.Tags)
		}
	}
}

// TestRemoveNonexistentTag verifies DELETE on non-existent tag_id.
func TestRemoveNonexistentTag(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	// Create project and task
	p, _ := h.store.CreateProject("Test")
	task, _ := h.store.CreateTask(p.ID, "Task 1")

	// Try to remove non-existent tag
	url := fmt.Sprintf("/tasks/%d/tags/999", task.ID)
	req := httptest.NewRequest("DELETE", url, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Should return either 404 or succeed silently (no-op)
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Fatalf("want 200 or 404, got %d", w.Code)
	}
}

// TestAddTagEmptyName verifies adding empty tag name returns 400.
func TestAddTagEmptyName(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	// Create project and task
	p, _ := h.store.CreateProject("Test")
	task, _ := h.store.CreateTask(p.ID, "Task 1")

	// Try to add empty tag
	reqBody := map[string]string{"tag_name": "  "}
	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("/tasks/%d/tags/add", task.ID)
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Verify 400
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "tag name required" {
		t.Fatalf("want 'tag name required', got %q", resp["error"])
	}
}

// TestAddTagInvalidJSON verifies malformed JSON returns 400.
func TestAddTagInvalidJSON(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	// Create project and task
	p, _ := h.store.CreateProject("Test")
	task, _ := h.store.CreateTask(p.ID, "Task 1")

	// Send invalid JSON
	invalidJSON := `{"tag_name": "unclosed`
	url := fmt.Sprintf("/tasks/%d/tags/add", task.ID)
	req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader([]byte(invalidJSON))))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Verify 400
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

// TestAddTagMultipleTags verifies adding multiple different tags to same task.
func TestAddTagMultipleTags(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	// Create project and task
	p, _ := h.store.CreateProject("Test")
	task, _ := h.store.CreateTask(p.ID, "Task 1")

	// Add 3 different tags
	tagNames := []string{"urgent", "important", "review"}

	for _, tagName := range tagNames {
		reqBody := map[string]string{"tag_name": tagName}
		body, _ := json.Marshal(reqBody)
		url := fmt.Sprintf("/tasks/%d/tags/add", task.ID)
		req := httptest.NewRequest("POST", url, io.NopCloser(bytes.NewReader(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("add %s: want 200, got %d", tagName, w.Code)
		}
	}

	// Verify all tags in DB
	tags, _ := h.store.GetTaskTags(task.ID)
	if len(tags) != 3 {
		t.Fatalf("want 3 tags, got %d", len(tags))
	}

	tagMap := make(map[string]bool)
	for _, tag := range tags {
		tagMap[tag] = true
	}
	for _, tagName := range tagNames {
		if !tagMap[tagName] {
			t.Fatalf("want tag %q", tagName)
		}
	}
}

// TestAddTagToMultipleTasks verifies same tag can be linked to multiple tasks.
func TestAddTagToMultipleTasks(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	// Create project and 2 tasks
	p, _ := h.store.CreateProject("Test")
	task1, _ := h.store.CreateTask(p.ID, "Task 1")
	task2, _ := h.store.CreateTask(p.ID, "Task 2")

	// Add same tag to both tasks
	reqBody := map[string]string{"tag_name": "urgent"}
	body, _ := json.Marshal(reqBody)

	// Add to task1
	url1 := fmt.Sprintf("/tasks/%d/tags/add", task1.ID)
	req1 := httptest.NewRequest("POST", url1, io.NopCloser(bytes.NewReader(body)))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("add to task1: want 200, got %d", w1.Code)
	}

	var resp1 map[string]interface{}
	json.NewDecoder(w1.Body).Decode(&resp1)
	tagID1 := resp1["tag_id"]

	// Add to task2
	body2, _ := json.Marshal(reqBody)
	url2 := fmt.Sprintf("/tasks/%d/tags/add", task2.ID)
	req2 := httptest.NewRequest("POST", url2, io.NopCloser(bytes.NewReader(body2)))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("add to task2: want 200, got %d", w2.Code)
	}

	var resp2 map[string]interface{}
	json.NewDecoder(w2.Body).Decode(&resp2)
	tagID2 := resp2["tag_id"]

	// Verify same tag_id (same underlying tag)
	if tagID1 != tagID2 {
		t.Fatalf("want same tag_id for both tasks, got %v != %v", tagID1, tagID2)
	}

	// Verify both tasks have the tag
	tags1, _ := h.store.GetTaskTags(task1.ID)
	tags2, _ := h.store.GetTaskTags(task2.ID)
	if len(tags1) != 1 || tags1[0] != "urgent" {
		t.Fatalf("task1 tags: want [urgent], got %#v", tags1)
	}
	if len(tags2) != 1 || tags2[0] != "urgent" {
		t.Fatalf("task2 tags: want [urgent], got %#v", tags2)
	}
}

// TestRemoveTagResponseFormat verifies success response structure.
func TestRemoveTagResponseFormat(t *testing.T) {
	h := newHandler(t)
	mux := h.Routes()

	// Create and tag a task
	p, _ := h.store.CreateProject("Test")
	task, _ := h.store.CreateTask(p.ID, "Task 1")
	tagID, _ := h.store.AddTag(task.ID, "urgent")

	// Remove tag
	url := fmt.Sprintf("/tasks/%d/tags/%d", task.ID, tagID)
	req := httptest.NewRequest("DELETE", url, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	// Verify response is exactly {"success": true}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if success, ok := resp["success"].(bool); !ok || !success {
		t.Fatalf("want success=true, got %#v", resp)
	}

	if len(resp) != 1 {
		t.Fatalf("want 1 field, got %d fields: %#v", len(resp), resp)
	}
}
