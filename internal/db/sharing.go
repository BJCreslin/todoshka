package db

func (d *DB) Share(resourceType string, resourceID, userID int64) error {
	_, err := d.Exec(`INSERT OR IGNORE INTO shares (resource_type, resource_id, user_id) VALUES (?, ?, ?)`,
		resourceType, resourceID, userID)
	return err
}

func (d *DB) Unshare(resourceType string, resourceID, userID int64) error {
	_, err := d.Exec(`DELETE FROM shares WHERE resource_type = ? AND resource_id = ? AND user_id = ?`,
		resourceType, resourceID, userID)
	return err
}

func (d *DB) IsOwner(resourceType string, resourceID, userID int64) (bool, error) {
	var n int
	var q string
	switch resourceType {
	case "task": q = `SELECT COUNT(*) FROM tasks WHERE id = ? AND owner_id = ?`
	case "note": q = `SELECT COUNT(*) FROM notes WHERE id = ? AND owner_id = ?`
	default:     return false, nil
	}
	if err := d.QueryRow(q, resourceID, userID).Scan(&n); err != nil { return false, err }
	return n > 0, nil
}

func (d *DB) userHasNoteAccess(noteID, userID int64) bool {
	var n int
	_ = d.QueryRow(`SELECT COUNT(*) FROM notes n
		WHERE n.id = ? AND (n.owner_id = ? OR EXISTS (
			SELECT 1 FROM shares s WHERE s.resource_type='note' AND s.resource_id=n.id AND s.user_id=?
		))`, noteID, userID, userID).Scan(&n)
	return n > 0
}

func (d *DB) LinkNoteToTask(noteID, taskID, userID int64) error {
	if !d.userHasNoteAccess(noteID, userID) || !d.userHasTaskAccess(taskID, userID) { return ErrNotFound }
	_, err := d.Exec(`INSERT OR IGNORE INTO note_task_links (note_id, task_id) VALUES (?, ?)`, noteID, taskID)
	return err
}

func (d *DB) UnlinkNoteFromTask(noteID, taskID, userID int64) error {
	if !d.userHasNoteAccess(noteID, userID) { return ErrNotFound }
	_, err := d.Exec(`DELETE FROM note_task_links WHERE note_id = ? AND task_id = ?`, noteID, taskID)
	return err
}

type SharedItem struct {
	Type string
	ID   int64
}

func (d *DB) ListSharedWithUser(userID int64) ([]SharedItem, error) {
	rows, err := d.Query(`SELECT resource_type, resource_id FROM shares WHERE user_id = ?`, userID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []SharedItem
	for rows.Next() {
		var s SharedItem
		if err := rows.Scan(&s.Type, &s.ID); err != nil { return nil, err }
		out = append(out, s)
	}
	return out, rows.Err()
}

type ShareUser struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
}

func (d *DB) ListSharesForResource(resourceType string, resourceID int64) ([]ShareUser, error) {
	rows, err := d.Query(`SELECT s.user_id, u.username FROM shares s JOIN users u ON u.id = s.user_id WHERE s.resource_type = ? AND s.resource_id = ? ORDER BY u.username`, resourceType, resourceID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []ShareUser
	for rows.Next() {
		var su ShareUser
		if err := rows.Scan(&su.UserID, &su.Username); err != nil { return nil, err }
		out = append(out, su)
	}
	return out, rows.Err()
}
