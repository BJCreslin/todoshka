package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/vladimirkreslin/todoshka/internal/models"
)

type TaskFilter struct {
	Status string
	Tag    string
	Q      string
}

func defaultStr(s, d string) string { if s == "" { return d }; return s }

func (d *DB) CreateTask(ownerID int64, t *models.Task) (int64, error) {
	pos, err := d.nextTaskPosition(ownerID, defaultStr(t.Status, "todo"))
	if err != nil { return 0, err }
	res, err := d.Exec(`INSERT INTO tasks (owner_id, title, description, status, priority, due_date, position)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ownerID, t.Title, t.Description, defaultStr(t.Status, "todo"),
		defaultStr(t.Priority, "medium"), t.DueDate, pos)
	if err != nil { return 0, err }
	id, _ := res.LastInsertId()
	if len(t.Tags) > 0 { if err := d.setTaskTags(id, t.Tags); err != nil { return 0, err } }
	return id, nil
}

func (d *DB) nextTaskPosition(ownerID int64, status string) (int64, error) {
	var maxPos sql.NullInt64
	err := d.QueryRow(`SELECT MAX(position) FROM tasks WHERE owner_id = ? AND status = ?`, ownerID, status).Scan(&maxPos)
	if err != nil { return 0, err }
	if !maxPos.Valid { return 0, nil }
	return maxPos.Int64 + 1, nil
}

func (d *DB) GetTask(id, userID int64) (*models.Task, error) {
	if !d.userHasTaskAccess(id, userID) { return nil, ErrNotFound }
	var t models.Task
	err := d.QueryRow(`SELECT id, owner_id, title, description, status, priority, due_date, position, created_at, updated_at
		FROM tasks WHERE id = ?`, id).Scan(
		&t.ID, &t.OwnerID, &t.Title, &t.Description, &t.Status, &t.Priority, &t.DueDate, &t.Position, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) { return nil, ErrNotFound }
	if err != nil { return nil, err }
	_ = d.loadTaskRelations(&t)
	return &t, nil
}

func (d *DB) loadTaskRelations(t *models.Task) error {
	rows, err := d.Query(`SELECT id, task_id, title, done, position FROM subtasks WHERE task_id = ? ORDER BY position, id`, t.ID)
	if err != nil { return err }
	for rows.Next() {
		var s models.Subtask
		if err := rows.Scan(&s.ID, &s.TaskID, &s.Title, &s.Done, &s.Position); err != nil { rows.Close(); return err }
		t.Subtasks = append(t.Subtasks, s)
	}
	rows.Close()
	tagRows, err := d.Query(`SELECT tag FROM task_tags WHERE task_id = ? ORDER BY tag`, t.ID)
	if err != nil { return err }
	for tagRows.Next() {
		var tag string
		if err := tagRows.Scan(&tag); err != nil { tagRows.Close(); return err }
		t.Tags = append(t.Tags, tag)
	}
	tagRows.Close()
	linkRows, err := d.Query(`SELECT note_id FROM note_task_links WHERE task_id = ? ORDER BY note_id`, t.ID)
	if err != nil { return err }
	for linkRows.Next() {
		var nid int64
		if err := linkRows.Scan(&nid); err != nil { linkRows.Close(); return err }
		t.LinkedNotes = append(t.LinkedNotes, nid)
	}
	linkRows.Close()
	return nil
}

func (d *DB) ListTasksForUser(userID int64, f TaskFilter) ([]models.Task, error) {
	q := `SELECT id, owner_id, title, description, status, priority, due_date, position, created_at, updated_at
		FROM tasks
		WHERE (owner_id = ? OR id IN (SELECT resource_id FROM shares WHERE resource_type='task' AND user_id=?))`
	args := []any{userID, userID}
	if f.Status != "" { q += ` AND status = ?`; args = append(args, f.Status) }
	if f.Tag != "" { q += ` AND id IN (SELECT task_id FROM task_tags WHERE tag = ?)`; args = append(args, f.Tag) }
	if f.Q != "" { q += ` AND title LIKE ?`; args = append(args, "%"+f.Q+"%") }
	q += ` ORDER BY status, position, id`
	rows, err := d.Query(q, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.OwnerID, &t.Title, &t.Description, &t.Status, &t.Priority, &t.DueDate, &t.Position, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (d *DB) UpdateTask(id, userID int64, fields map[string]any) error {
	if !d.userHasTaskAccess(id, userID) { return ErrNotFound }
	allowed := map[string]bool{"title": true, "description": true, "status": true, "priority": true, "due_date": true, "position": true}
	var sets []string
	var args []any
	for k, v := range fields {
		if !allowed[k] { continue }
		sets = append(sets, fmt.Sprintf("%s = ?", k))
		args = append(args, v)
	}
	if len(sets) == 0 { return nil }
	args = append(args, id)
	_, err := d.Exec("UPDATE tasks SET "+strings.Join(sets, ", ")+", updated_at = CURRENT_TIMESTAMP WHERE id = ?", args...)
	return err
}

func (d *DB) DeleteTask(id, userID int64) error {
	if !d.userIsTaskOwner(id, userID) { return ErrNotFound }
	_, err := d.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}

func (d *DB) userHasTaskAccess(taskID, userID int64) bool {
	var n int
	_ = d.QueryRow(`SELECT COUNT(*) FROM tasks t
		WHERE t.id = ? AND (t.owner_id = ? OR EXISTS (
			SELECT 1 FROM shares s WHERE s.resource_type='task' AND s.resource_id=t.id AND s.user_id=?
		))`, taskID, userID, userID).Scan(&n)
	return n > 0
}

func (d *DB) userIsTaskOwner(taskID, userID int64) bool {
	var n int
	_ = d.QueryRow(`SELECT COUNT(*) FROM tasks WHERE id = ? AND owner_id = ?`, taskID, userID).Scan(&n)
	return n > 0
}

func (d *DB) setTaskTags(taskID int64, tags []string) error {
	tx, err := d.Begin()
	if err != nil { return err }
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM task_tags WHERE task_id = ?`, taskID); err != nil { return err }
	for _, t := range tags {
		if t == "" { continue }
		if _, err := tx.Exec(`INSERT OR IGNORE INTO task_tags (task_id, tag) VALUES (?, ?)`, taskID, t); err != nil { return err }
	}
	return tx.Commit()
}

func (d *DB) CreateSubtask(taskID, userID int64, title string) (int64, error) {
	if !d.userHasTaskAccess(taskID, userID) { return 0, ErrNotFound }
	var maxPos sql.NullInt64
	_ = d.QueryRow(`SELECT MAX(position) FROM subtasks WHERE task_id = ?`, taskID).Scan(&maxPos)
	pos := int64(0)
	if maxPos.Valid { pos = maxPos.Int64 + 1 }
	res, err := d.Exec(`INSERT INTO subtasks (task_id, title, position) VALUES (?, ?, ?)`, taskID, title, pos)
	if err != nil { return 0, err }
	return res.LastInsertId()
}

func (d *DB) UpdateSubtask(id, userID int64, fields map[string]any) error {
	var taskID int64
	if err := d.QueryRow(`SELECT task_id FROM subtasks WHERE id = ?`, id).Scan(&taskID); err != nil {
		if errors.Is(err, sql.ErrNoRows) { return ErrNotFound }
		return err
	}
	if !d.userHasTaskAccess(taskID, userID) { return ErrNotFound }
	allowed := map[string]bool{"title": true, "done": true, "position": true}
	var sets []string
	var args []any
	for k, v := range fields {
		if !allowed[k] { continue }
		sets = append(sets, fmt.Sprintf("%s = ?", k))
		args = append(args, v)
	}
	if len(sets) == 0 { return nil }
	args = append(args, id)
	_, err := d.Exec("UPDATE subtasks SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...)
	return err
}

func (d *DB) DeleteSubtask(id, userID int64) error {
	var taskID int64
	if err := d.QueryRow(`SELECT task_id FROM subtasks WHERE id = ?`, id).Scan(&taskID); err != nil { return ErrNotFound }
	if !d.userHasTaskAccess(taskID, userID) { return ErrNotFound }
	_, err := d.Exec(`DELETE FROM subtasks WHERE id = ?`, id)
	return err
}

func (d *DB) AddTaskTag(taskID, userID int64, tag string) error {
	if !d.userHasTaskAccess(taskID, userID) { return ErrNotFound }
	_, err := d.Exec(`INSERT OR IGNORE INTO task_tags (task_id, tag) VALUES (?, ?)`, taskID, tag)
	return err
}

func (d *DB) RemoveTaskTag(taskID, userID int64, tag string) error {
	if !d.userHasTaskAccess(taskID, userID) { return ErrNotFound }
	_, err := d.Exec(`DELETE FROM task_tags WHERE task_id = ? AND tag = ?`, taskID, tag)
	return err
}
