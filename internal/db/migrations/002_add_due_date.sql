-- Add due_date column to tasks table
ALTER TABLE tasks ADD COLUMN due_date DATE;
CREATE INDEX IF NOT EXISTS idx_tasks_due_date ON tasks(due_date);
