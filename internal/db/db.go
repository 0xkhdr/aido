package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ErrEmptyName is returned when a project name or task title is blank.
var ErrEmptyName = errors.New("name must not be empty")

// ErrNoProject is returned when a task references a project that does not exist.
var ErrNoProject = errors.New("project does not exist")

// ErrEmptySelection is returned when a bulk project deletion has no targets.
var ErrEmptySelection = errors.New("at least one project must be selected")

// Store wraps the SQLite connection pool.
type Store struct {
	db *sql.DB
}

// Project is a container for tasks.
type Project struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

// Task is a single todo item belonging to exactly one project.
type Task struct {
	ID          int64
	ProjectID   int64
	Title       string
	Description string
	Done        bool
	Priority    string // "low", "medium", "high", or ""
	DueDate     *time.Time
	Tags        []string
	CreatedAt   time.Time
}

// Tag is a label for organizing tasks.
type Tag struct {
	ID   int64
	Name string
}

// Open connects to the SQLite database at path with sane pragmas.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// modernc sqlite is safe with a small pool; keep writes serialized.
	d.SetMaxOpenConns(1)
	if err := d.Ping(); err != nil {
		return nil, err
	}
	return &Store{db: d}, nil
}

// Close releases the connection pool.
func (s *Store) Close() error { return s.db.Close() }

// Migrate creates tables if absent and migrates a legacy flat task list into the
// project model: it adds project_id to tasks, ensures a default project, and
// backfills orphan tasks onto it.
func (s *Store) Migrate() error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS projects (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
	`); err != nil {
		return err
	}

	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id  INTEGER REFERENCES projects(id) ON DELETE CASCADE,
			title       TEXT NOT NULL,
			description TEXT,
			done        INTEGER NOT NULL DEFAULT 0,
			created_at  TEXT NOT NULL DEFAULT (datetime('now'))
		);
	`); err != nil {
		return err
	}

	// Legacy databases predate project_id; add the column when missing.
	hasProjectID, err := s.columnExists("tasks", "project_id")
	if err != nil {
		return err
	}
	if !hasProjectID {
		if _, err := s.db.Exec(`ALTER TABLE tasks ADD COLUMN project_id INTEGER REFERENCES projects(id) ON DELETE CASCADE`); err != nil {
			return err
		}
	}

	// Legacy databases predate description; add the column when missing.
	hasDescription, err := s.columnExists("tasks", "description")
	if err != nil {
		return err
	}
	if !hasDescription {
		if _, err := s.db.Exec(`ALTER TABLE tasks ADD COLUMN description TEXT`); err != nil {
			return err
		}
	}

	// Legacy databases predate priority; add the column when missing.
	hasPriority, err := s.columnExists("tasks", "priority")
	if err != nil {
		return err
	}
	if !hasPriority {
		if _, err := s.db.Exec(`ALTER TABLE tasks ADD COLUMN priority TEXT`); err != nil {
			return err
		}
	}

	// Legacy databases predate due_date; add the column when missing.
	hasDueDate, err := s.columnExists("tasks", "due_date")
	if err != nil {
		return err
	}
	if !hasDueDate {
		if _, err := s.db.Exec(`ALTER TABLE tasks ADD COLUMN due_date TEXT`); err != nil {
			return err
		}
	}

	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS tags (
			id   INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE
		);
	`); err != nil {
		return err
	}

	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS task_tags (
			task_id INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
			tag_id  INTEGER REFERENCES tags(id) ON DELETE CASCADE,
			PRIMARY KEY (task_id, tag_id)
		);
	`); err != nil {
		return err
	}

	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id)`); err != nil {
		return err
	}

	// Ensure a default project, then adopt any orphan tasks onto it (R1.4).
	defaultID, err := s.EnsureDefaultProject()
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`UPDATE tasks SET project_id = ? WHERE project_id IS NULL`, defaultID); err != nil {
		return err
	}
	return nil
}

// columnExists reports whether a table has a named column.
func (s *Store) columnExists(table, column string) (bool, error) {
	rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			ctype      string
			notNull    int
			dflt       sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &primaryKey); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

// EnsureDefaultProject provisions a default project when none exist so the home
// page is never empty (R1.1). It is idempotent and returns the id of the first
// project.
func (s *Store) EnsureDefaultProject() (int64, error) {
	var id int64
	err := s.db.QueryRow(`SELECT id FROM projects ORDER BY id ASC LIMIT 1`).Scan(&id)
	switch {
	case err == nil:
		return id, nil
	case errors.Is(err, sql.ErrNoRows):
		p, err := s.CreateProject("My Tasks")
		if err != nil {
			return 0, err
		}
		return p.ID, nil
	default:
		return 0, err
	}
}

// CreateProject inserts a project after trimming and guarding its name (R1.2,
// R1.3). A blank name is rejected with ErrEmptyName.
func (s *Store) CreateProject(name string) (Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Project{}, ErrEmptyName
	}
	res, err := s.db.Exec(`INSERT INTO projects (name) VALUES (?)`, name)
	if err != nil {
		return Project{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetProject(id)
}

// RenameProject trims and persists a project's name without changing its identity
// or task ownership. Missing projects return ErrNoProject.
func (s *Store) RenameProject(id int64, name string) (Project, error) {
	// D1: validate for empty (pure whitespace) but persist exactly as submitted
	if strings.TrimSpace(name) == "" {
		return Project{}, ErrEmptyName
	}

	result, err := s.db.Exec(`UPDATE projects SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return Project{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return Project{}, err
	}
	if changed == 0 {
		return Project{}, ErrNoProject
	}
	return s.GetProject(id)
}

// DeleteProject removes one project and its tasks atomically. It returns the
// oldest remaining project, or a newly provisioned default project when the
// deleted project was the last one.
func (s *Store) DeleteProject(id int64) (Project, error) {
	return s.DeleteProjects([]int64{id})
}

// DeleteProjects removes exactly the supplied, unique existing projects and
// their tasks in one transaction. Every supplied identifier is validated before
// mutation, so a stale ID cannot cause a partial bulk deletion.
func (s *Store) DeleteProjects(ids []int64) (active Project, err error) {
	ids = uniqueProjectIDs(ids)
	if len(ids) == 0 {
		return Project{}, ErrEmptySelection
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Project{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	for _, id := range ids {
		var exists int
		if err = tx.QueryRow(`SELECT 1 FROM projects WHERE id = ?`, id).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return Project{}, ErrNoProject
			}
			return Project{}, err
		}
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	if _, err = tx.Exec(`DELETE FROM projects WHERE id IN (`+placeholders+`)`, args...); err != nil {
		return Project{}, err
	}

	active, err = firstProject(tx)
	if errors.Is(err, sql.ErrNoRows) {
		result, insertErr := tx.Exec(`INSERT INTO projects (name) VALUES (?)`, "My Tasks")
		if insertErr != nil {
			return Project{}, insertErr
		}
		id, insertErr := result.LastInsertId()
		if insertErr != nil {
			return Project{}, insertErr
		}
		active, err = projectByID(tx, id)
	}
	if err != nil {
		return Project{}, err
	}
	if err = tx.Commit(); err != nil {
		return Project{}, err
	}
	return active, nil
}

// ListProjects returns all projects, oldest first (R2.1).
func (s *Store) ListProjects() ([]Project, error) {
	rows, err := s.db.Query(`SELECT id, name, created_at FROM projects ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetProject returns the project with id, or ErrNoProject when absent (R2.2).
func (s *Store) GetProject(id int64) (Project, error) {
	row := s.db.QueryRow(`SELECT id, name, created_at FROM projects WHERE id = ?`, id)
	p, err := scanProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNoProject
	}
	return p, err
}

// CreateTask inserts a task in an existing project after trimming and guarding
// its title, rejecting a blank title or an unknown project (R3.2, R3.3, R1.4).
func (s *Store) CreateTask(projectID int64, title string) (Task, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Task{}, ErrEmptyName
	}
	if _, err := s.GetProject(projectID); err != nil {
		return Task{}, err
	}
	res, err := s.db.Exec(`INSERT INTO tasks (project_id, title) VALUES (?, ?)`, projectID, title)
	if err != nil {
		return Task{}, err
	}
	id, _ := res.LastInsertId()
	return Task{ID: id, ProjectID: projectID, Title: title}, nil
}

// ListTasksByProject returns the tasks belonging to one project, newest first
// (R4.1).
func (s *Store) ListTasksByProject(projectID int64) ([]Task, error) {
	rows, err := s.db.Query(`SELECT id, project_id, title, COALESCE(description, ''), done, COALESCE(priority, ''), COALESCE(due_date, ''), created_at FROM tasks WHERE project_id = ? ORDER BY id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Task
	for rows.Next() {
		var (
			t       Task
			created string
			dueStr  string
		)
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Title, &t.Description, &t.Done, &t.Priority, &dueStr, &created); err != nil {
			return nil, err
		}
		t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		if dueStr != "" {
			dt, _ := time.Parse("2006-01-02", dueStr)
			t.DueDate = &dt
		}
		if tags, err := s.GetTaskTags(t.ID); err == nil {
			t.Tags = tags
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetTask returns a single task by ID.
func (s *Store) GetTask(id int64) (Task, error) {
	var (
		t       Task
		created string
		dueStr  sql.NullString
	)
	err := s.db.QueryRow(`SELECT id, project_id, title, COALESCE(description, ''), done, COALESCE(priority, ''), COALESCE(due_date, ''), created_at FROM tasks WHERE id = ?`, id).
		Scan(&t.ID, &t.ProjectID, &t.Title, &t.Description, &t.Done, &t.Priority, &dueStr, &created)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Task{}, errors.New("task not found")
		}
		return Task{}, err
	}
	t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
	if dueStr.Valid && dueStr.String != "" {
		dt, _ := time.Parse("2006-01-02", dueStr.String)
		t.DueDate = &dt
	}
	if tags, err := s.GetTaskTags(t.ID); err == nil {
		t.Tags = tags
	}
	return t, nil
}

// UpdateTaskDescription persists the task's description.
func (s *Store) UpdateTaskDescription(id int64, description string) error {
	_, err := s.db.Exec(`UPDATE tasks SET description = ? WHERE id = ?`, description, id)
	return err
}

// SetTaskPriority sets the priority for a task.
func (s *Store) SetTaskPriority(id int64, priority string) error {
	_, err := s.db.Exec(`UPDATE tasks SET priority = ? WHERE id = ?`, priority, id)
	return err
}

// ToggleTask flips the done flag and persists it (R4.2).
func (s *Store) ToggleTask(id int64) error {
	_, err := s.db.Exec(`UPDATE tasks SET done = 1 - done WHERE id = ?`, id)
	return err
}

// DeleteTask removes a task (R4.3).
func (s *Store) DeleteTask(id int64) error {
	_, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}

// GetTaskPriority returns the priority of a single task.
func (s *Store) GetTaskPriority(id int64) (string, error) {
	var priority sql.NullString
	err := s.db.QueryRow(`SELECT COALESCE(priority, '') FROM tasks WHERE id = ?`, id).Scan(&priority)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("task not found")
		}
		return "", err
	}
	return priority.String, nil
}

// UpdateTaskPriority sets the priority of a single task.
func (s *Store) UpdateTaskPriority(id int64, priority string) error {
	_, err := s.db.Exec(`UPDATE tasks SET priority = ? WHERE id = ?`, priority, id)
	return err
}

// BulkUpdateTaskPriority updates priority for multiple tasks in a transaction,
// silently skipping tasks that no longer exist. Returns count of updated tasks.
func (s *Store) BulkUpdateTaskPriority(taskIDs []int64, priority string) (int, error) {
	if len(taskIDs) == 0 {
		return 0, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	placeholders := strings.TrimRight(strings.Repeat("?,", len(taskIDs)), ",")
	args := make([]any, len(taskIDs))
	for i, id := range taskIDs {
		args[i] = id
	}

	result, err := tx.Exec(`UPDATE tasks SET priority = ? WHERE id IN (`+placeholders+`)`, append([]any{priority}, args...)...)
	if err != nil {
		return 0, err
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return int(count), nil
}

// BulkMarkDone sets done=true for multiple tasks in a transaction,
// silently skipping tasks that no longer exist. Returns count of updated tasks.
func (s *Store) BulkMarkDone(taskIDs []int64) (int, error) {
	if len(taskIDs) == 0 {
		return 0, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	placeholders := strings.TrimRight(strings.Repeat("?,", len(taskIDs)), ",")
	args := make([]any, len(taskIDs))
	for i, id := range taskIDs {
		args[i] = id
	}

	result, err := tx.Exec(`UPDATE tasks SET done = 1 WHERE id IN (`+placeholders+`)`, args...)
	if err != nil {
		return 0, err
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return int(count), nil
}

// BulkDeleteTasks deletes multiple tasks in a transaction,
// silently skipping tasks that no longer exist. Returns count of deleted tasks.
func (s *Store) BulkDeleteTasks(taskIDs []int64) (int, error) {
	if len(taskIDs) == 0 {
		return 0, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	placeholders := strings.TrimRight(strings.Repeat("?,", len(taskIDs)), ",")
	args := make([]any, len(taskIDs))
	for i, id := range taskIDs {
		args[i] = id
	}

	result, err := tx.Exec(`DELETE FROM tasks WHERE id IN (`+placeholders+`)`, args...)
	if err != nil {
		return 0, err
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return int(count), nil
}

// UpdateTask updates a task's title and done status atomically (R3.3).
// Returns ErrNoProject when task not found, or the updated task on success.
func (s *Store) UpdateTask(id int64, title string, done bool) (Task, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Task{}, ErrEmptyName
	}

	result, err := s.db.Exec(`UPDATE tasks SET title = ?, done = ? WHERE id = ?`, title, done, id)
	if err != nil {
		return Task{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return Task{}, err
	}
	if changed == 0 {
		return Task{}, ErrNoProject
	}

	// Fetch and return the updated task.
	rows, err := s.db.Query(`SELECT id, project_id, title, COALESCE(description, ''), done, COALESCE(priority, ''), COALESCE(due_date, ''), created_at FROM tasks WHERE id = ?`, id)
	if err != nil {
		return Task{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return Task{}, ErrNoProject
	}

	var (
		t       Task
		created string
		dueStr  string
	)
	if err := rows.Scan(&t.ID, &t.ProjectID, &t.Title, &t.Description, &t.Done, &t.Priority, &dueStr, &created); err != nil {
		return Task{}, err
	}
	t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
	if dueStr != "" {
		dt, _ := time.Parse("2006-01-02", dueStr)
		t.DueDate = &dt
	}
	if tags, err := s.GetTaskTags(t.ID); err == nil {
		t.Tags = tags
	}
	return t, nil
}

// SetDueDate sets or clears the due date for a task.
func (s *Store) SetDueDate(id int64, dueDate *time.Time) error {
	var val any
	if dueDate != nil {
		val = dueDate.Format("2006-01-02")
	}
	_, err := s.db.Exec(`UPDATE tasks SET due_date = ? WHERE id = ?`, val, id)
	return err
}

// ListTasksByDueDateRange returns tasks in a project with due dates within a range,
// sorted by due_date ASC.
func (s *Store) ListTasksByDueDateRange(projectID int64, after, before time.Time) ([]Task, error) {
	afterStr := after.Format("2006-01-02")
	beforeStr := before.Format("2006-01-02")
	rows, err := s.db.Query(`
		SELECT id, project_id, title, COALESCE(description, ''), done, COALESCE(priority, ''),
		       COALESCE(due_date, ''), created_at
		FROM tasks
		WHERE project_id = ? AND due_date >= ? AND due_date <= ?
		ORDER BY due_date ASC`, projectID, afterStr, beforeStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Task
	for rows.Next() {
		var (
			t       Task
			created string
			dueStr  string
		)
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Title, &t.Description, &t.Done, &t.Priority, &dueStr, &created); err != nil {
			return nil, err
		}
		t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		if dueStr != "" {
			dt, _ := time.Parse("2006-01-02", dueStr)
			t.DueDate = &dt
		}
		if tags, err := s.GetTaskTags(t.ID); err == nil {
			t.Tags = tags
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanProject(sc scanner) (Project, error) {
	var (
		p       Project
		created string
	)
	if err := sc.Scan(&p.ID, &p.Name, &created); err != nil {
		return Project{}, err
	}
	p.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
	return p, nil
}

type queryRower interface {
	QueryRow(query string, args ...any) *sql.Row
}

func firstProject(q queryRower) (Project, error) {
	return scanProject(q.QueryRow(`SELECT id, name, created_at FROM projects ORDER BY id ASC LIMIT 1`))
}

func projectByID(q queryRower, id int64) (Project, error) {
	return scanProject(q.QueryRow(`SELECT id, name, created_at FROM projects WHERE id = ?`, id))
}

func uniqueProjectIDs(ids []int64) []int64 {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}

// SearchTasksByKeyword returns tasks matching keyword in title, newest first.
func (s *Store) SearchTasksByKeyword(projectID int64, keyword string) ([]Task, error) {
	pattern := "%" + keyword + "%"
	rows, err := s.db.Query(`SELECT id, project_id, title, COALESCE(description, ''), done, COALESCE(priority, ''), COALESCE(due_date, ''), created_at FROM tasks WHERE project_id = ? AND title LIKE ? ORDER BY id DESC`, projectID, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Task
	for rows.Next() {
		var (
			t      Task
			created string
			dueDate string
		)
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Title, &t.Description, &t.Done, &t.Priority, &dueDate, &created); err != nil {
			return nil, err
		}
		t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		if dueDate != "" {
			if du, err := time.Parse("2006-01-02", dueDate); err == nil {
				t.DueDate = &du
			}
		}
		if tags, err := s.GetTaskTags(t.ID); err == nil {
			t.Tags = tags
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetTaskTags returns tag names for a task.
func (s *Store) GetTaskTags(taskID int64) ([]string, error) {
	rows, err := s.db.Query(`SELECT name FROM tags WHERE id IN (SELECT tag_id FROM task_tags WHERE task_id = ?) ORDER BY name`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tags = append(tags, name)
	}
	return tags, rows.Err()
}

// AddTag creates a tag if not exists and links it to a task. Returns tag ID.
func (s *Store) AddTag(taskID int64, tagName string) (int64, error) {
	tagName = strings.TrimSpace(tagName)
	if tagName == "" {
		return 0, ErrEmptyName
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var tagID int64
	err = tx.QueryRow(`SELECT id FROM tags WHERE name = ?`, tagName).Scan(&tagID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			res, err := tx.Exec(`INSERT INTO tags (name) VALUES (?)`, tagName)
			if err != nil {
				return 0, err
			}
			tagID, _ = res.LastInsertId()
		} else {
			return 0, err
		}
	}

	_, err = tx.Exec(`INSERT OR IGNORE INTO task_tags (task_id, tag_id) VALUES (?, ?)`, taskID, tagID)
	if err != nil {
		return 0, err
	}

	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return tagID, nil
}

// RemoveTag removes a tag from a task.
func (s *Store) RemoveTag(taskID, tagID int64) error {
	_, err := s.db.Exec(`DELETE FROM task_tags WHERE task_id = ? AND tag_id = ?`, taskID, tagID)
	return err
}
