package db

import (
	"errors"
	"path/filepath"
	"testing"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s
}

// R1.1: empty database gets a default project after migration; idempotent.
func TestEnsureDefaultProject(t *testing.T) {
	s := newStore(t)

	projects, err := s.ListProjects()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("want 1 default project after migrate, got %d", len(projects))
	}

	id1, err := s.EnsureDefaultProject()
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	id2, err := s.EnsureDefaultProject()
	if err != nil {
		t.Fatalf("ensure again: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("EnsureDefaultProject not idempotent: %d != %d", id1, id2)
	}
	projects, _ = s.ListProjects()
	if len(projects) != 1 {
		t.Fatalf("EnsureDefaultProject created duplicate; got %d projects", len(projects))
	}
}

// R1.2: a submitted name creates a project returned with id and name.
func TestCreateProject(t *testing.T) {
	s := newStore(t)

	p, err := s.CreateProject("  Work  ")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.ID == 0 {
		t.Fatalf("want non-zero id")
	}
	if p.Name != "Work" {
		t.Fatalf("want trimmed name Work, got %q", p.Name)
	}

	got, err := s.GetProject(p.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "Work" {
		t.Fatalf("persisted name %q", got.Name)
	}
}

// R1.3: blank/whitespace project name rejected, no row written.
func TestCreateProjectRejectsBlank(t *testing.T) {
	s := newStore(t)
	before, _ := s.ListProjects()

	for _, name := range []string{"", "   ", "\t\n"} {
		if _, err := s.CreateProject(name); !errors.Is(err, ErrEmptyName) {
			t.Fatalf("name %q: want ErrEmptyName, got %v", name, err)
		}
	}

	after, _ := s.ListProjects()
	if len(after) != len(before) {
		t.Fatalf("blank name created a project: %d -> %d", len(before), len(after))
	}
}

// R2.2: unknown project id is rejected.
func TestGetProjectUnknown(t *testing.T) {
	s := newStore(t)
	if _, err := s.GetProject(99999); !errors.Is(err, ErrNoProject) {
		t.Fatalf("want ErrNoProject, got %v", err)
	}
}

// R1.4 + R3.2: task is associated with an existing project.
func TestCreateTask(t *testing.T) {
	s := newStore(t)
	p, _ := s.CreateProject("P")

	tk, err := s.CreateTask(p.ID, "  buy milk  ")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if tk.ProjectID != p.ID {
		t.Fatalf("task project_id = %d, want %d", tk.ProjectID, p.ID)
	}
	if tk.Title != "buy milk" {
		t.Fatalf("want trimmed title, got %q", tk.Title)
	}
}

// R3.3: blank task title rejected, no row written.
func TestCreateTaskRejectsBlank(t *testing.T) {
	s := newStore(t)
	p, _ := s.CreateProject("P")

	if _, err := s.CreateTask(p.ID, "   "); !errors.Is(err, ErrEmptyName) {
		t.Fatalf("want ErrEmptyName, got %v", err)
	}
	list, _ := s.ListTasksByProject(p.ID)
	if len(list) != 0 {
		t.Fatalf("blank title created a task: %d", len(list))
	}
}

// R1.4: task creation against an unknown project is rejected.
func TestCreateTaskUnknownProject(t *testing.T) {
	s := newStore(t)
	if _, err := s.CreateTask(99999, "orphan"); !errors.Is(err, ErrNoProject) {
		t.Fatalf("want ErrNoProject, got %v", err)
	}
}

// R4.1: task list is scoped to one project.
func TestListTasksByProject(t *testing.T) {
	s := newStore(t)
	a, _ := s.CreateProject("A")
	b, _ := s.CreateProject("B")
	s.CreateTask(a.ID, "a1")
	s.CreateTask(a.ID, "a2")
	s.CreateTask(b.ID, "b1")

	la, _ := s.ListTasksByProject(a.ID)
	if len(la) != 2 {
		t.Fatalf("project A: want 2 tasks, got %d", len(la))
	}
	for _, tk := range la {
		if tk.ProjectID != a.ID {
			t.Fatalf("leak: task %d belongs to project %d", tk.ID, tk.ProjectID)
		}
	}
}

// R4.2: toggle persists the done state.
func TestToggleTask(t *testing.T) {
	s := newStore(t)
	p, _ := s.CreateProject("P")
	tk, _ := s.CreateTask(p.ID, "t")

	if err := s.ToggleTask(tk.ID); err != nil {
		t.Fatalf("toggle: %v", err)
	}
	list, _ := s.ListTasksByProject(p.ID)
	if !list[0].Done {
		t.Fatalf("toggle did not set done")
	}
	s.ToggleTask(tk.ID)
	list, _ = s.ListTasksByProject(p.ID)
	if list[0].Done {
		t.Fatalf("toggle did not clear done")
	}
}

// R4.3: delete removes the task.
func TestDeleteTask(t *testing.T) {
	s := newStore(t)
	p, _ := s.CreateProject("P")
	tk, _ := s.CreateTask(p.ID, "t")

	if err := s.DeleteTask(tk.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list, _ := s.ListTasksByProject(p.ID)
	if len(list) != 0 {
		t.Fatalf("task still present after delete: %d", len(list))
	}
}
