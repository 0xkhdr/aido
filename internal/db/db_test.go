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

func TestRenameProject(t *testing.T) {
	s := newStore(t)
	p, _ := s.CreateProject("Original")
	task, _ := s.CreateTask(p.ID, "keep ownership")

	renamed, err := s.RenameProject(p.ID, "  Renamed  ")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if renamed.ID != p.ID || renamed.Name != "Renamed" {
		t.Fatalf("rename = %#v, want same id with trimmed name", renamed)
	}
	list, _ := s.ListTasksByProject(p.ID)
	if len(list) != 1 || list[0].ID != task.ID {
		t.Fatalf("rename changed task ownership: %#v", list)
	}
}

func TestRenameProjectRejectsBlankAndMissing(t *testing.T) {
	s := newStore(t)
	p, _ := s.CreateProject("Original")

	if _, err := s.RenameProject(p.ID, " \t\n"); !errors.Is(err, ErrEmptyName) {
		t.Fatalf("blank rename error = %v, want ErrEmptyName", err)
	}
	got, _ := s.GetProject(p.ID)
	if got.Name != "Original" {
		t.Fatalf("blank rename changed name to %q", got.Name)
	}
	if _, err := s.RenameProject(99999, "Missing"); !errors.Is(err, ErrNoProject) {
		t.Fatalf("missing rename error = %v, want ErrNoProject", err)
	}
}

func TestDeleteProjectCascadesTasksAndReturnsOldestSurvivor(t *testing.T) {
	s := newStore(t)
	oldest, _ := s.ListProjects()
	a, _ := s.CreateProject("A")
	b, _ := s.CreateProject("B")
	s.CreateTask(a.ID, "remove me")
	s.CreateTask(b.ID, "keep me")

	active, err := s.DeleteProject(a.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if active.ID != oldest[0].ID {
		t.Fatalf("active = %d, want oldest survivor %d", active.ID, oldest[0].ID)
	}
	if _, err := s.GetProject(a.ID); !errors.Is(err, ErrNoProject) {
		t.Fatalf("deleted project error = %v, want ErrNoProject", err)
	}
	if tasks, _ := s.ListTasksByProject(a.ID); len(tasks) != 0 {
		t.Fatalf("deleted project's tasks remain: %#v", tasks)
	}
	if tasks, _ := s.ListTasksByProject(b.ID); len(tasks) != 1 {
		t.Fatalf("surviving project's tasks = %#v, want one", tasks)
	}
}

func TestDeleteProjectsValidatesBeforeMutatingAndDeduplicates(t *testing.T) {
	s := newStore(t)
	a, _ := s.CreateProject("A")
	b, _ := s.CreateProject("B")
	s.CreateTask(a.ID, "a")
	s.CreateTask(b.ID, "b")

	if _, err := s.DeleteProjects([]int64{a.ID, 99999}); !errors.Is(err, ErrNoProject) {
		t.Fatalf("stale bulk delete error = %v, want ErrNoProject", err)
	}
	if _, err := s.GetProject(a.ID); err != nil {
		t.Fatalf("stale bulk delete changed existing project: %v", err)
	}

	if _, err := s.DeleteProjects([]int64{a.ID, a.ID}); err != nil {
		t.Fatalf("deduplicated bulk delete: %v", err)
	}
	if _, err := s.GetProject(a.ID); !errors.Is(err, ErrNoProject) {
		t.Fatalf("duplicate ID did not delete target: %v", err)
	}
	if _, err := s.GetProject(b.ID); err != nil {
		t.Fatalf("duplicate ID deleted an unselected project: %v", err)
	}
}

func TestDeleteProjectsRejectsEmptyAndRecoversDefault(t *testing.T) {
	s := newStore(t)
	if _, err := s.DeleteProjects(nil); !errors.Is(err, ErrEmptySelection) {
		t.Fatalf("empty bulk delete error = %v, want ErrEmptySelection", err)
	}

	projects, _ := s.ListProjects()
	last := projects[0]
	s.CreateTask(last.ID, "orphan check")
	active, err := s.DeleteProject(last.ID)
	if err != nil {
		t.Fatalf("delete last: %v", err)
	}
	if active.Name != "My Tasks" {
		t.Fatalf("recovered project = %#v, want My Tasks", active)
	}
	projects, _ = s.ListProjects()
	if len(projects) != 1 || projects[0].ID != active.ID {
		t.Fatalf("projects after last deletion = %#v", projects)
	}
	if tasks, _ := s.ListTasksByProject(active.ID); len(tasks) != 0 {
		t.Fatalf("recovered project inherited deleted tasks: %#v", tasks)
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
