package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps the SQLite connection pool.
type Store struct {
	db *sql.DB
}

// Task is a single todo item.
type Task struct {
	ID        int64
	Title     string
	Done      bool
	CreatedAt time.Time
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

// Migrate creates tables if absent.
func (s *Store) Migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			title      TEXT NOT NULL,
			done       INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
	`)
	return err
}

// ListTasks returns all tasks, newest first.
func (s *Store) ListTasks() ([]Task, error) {
	rows, err := s.db.Query(`SELECT id, title, done, created_at FROM tasks ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Task
	for rows.Next() {
		var t Task
		var created string
		if err := rows.Scan(&t.ID, &t.Title, &t.Done, &created); err != nil {
			return nil, err
		}
		t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		out = append(out, t)
	}
	return out, rows.Err()
}

// AddTask inserts a task and returns it.
func (s *Store) AddTask(title string) (Task, error) {
	res, err := s.db.Exec(`INSERT INTO tasks (title) VALUES (?)`, title)
	if err != nil {
		return Task{}, err
	}
	id, _ := res.LastInsertId()
	return Task{ID: id, Title: title}, nil
}

// ToggleTask flips the done flag.
func (s *Store) ToggleTask(id int64) error {
	_, err := s.db.Exec(`UPDATE tasks SET done = 1 - done WHERE id = ?`, id)
	return err
}

// DeleteTask removes a task.
func (s *Store) DeleteTask(id int64) error {
	_, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}
