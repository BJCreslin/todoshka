package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/vladimirkreslin/todoshka/internal/models"
)

type NoteFilter struct {
	Q      string
	Tag    string
	Pinned *bool
}

func (d *DB) CreateNote(ownerID int64, n *models.Note) (int64, error) {
	if strings.TrimSpace(n.Title) == "" { return 0, errors.New("title required") }
	tx, err := d.Begin()
	if err != nil { return 0, err }
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO notes (owner_id, title, body_md, pinned) VALUES (?, ?, ?, ?)`,
		ownerID, n.Title, n.BodyMD, n.Pinned)
	if err != nil { return 0, err }
	id, _ := res.LastInsertId()
	if len(n.Tags) > 0 {
		for _, t := range n.Tags {
			if t == "" { continue }
			if _, err := tx.Exec(`INSERT OR IGNORE INTO note_tags (note_id, tag) VALUES (?, ?)`, id, t); err != nil { return 0, err }
		}
	}
	if err := tx.Commit(); err != nil { return 0, err }
	return id, nil
}

func (d *DB) GetNote(id, userID int64) (*models.Note, error) {
	if !d.userHasNoteAccess(id, userID) { return nil, ErrNotFound }
	var n models.Note
	err := d.QueryRow(`SELECT id, owner_id, title, body_md, pinned, created_at, updated_at
		FROM notes WHERE id = ?`, id).Scan(&n.ID, &n.OwnerID, &n.Title, &n.BodyMD, &n.Pinned, &n.CreatedAt, &n.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) { return nil, ErrNotFound }
	if err != nil { return nil, err }
	_ = d.loadNoteRelations(&n)
	return &n, nil
}

func (d *DB) loadNoteRelations(n *models.Note) error {
	tagRows, err := d.Query(`SELECT tag FROM note_tags WHERE note_id = ? ORDER BY tag`, n.ID)
	if err != nil { return err }
	for tagRows.Next() {
		var t string
		if err := tagRows.Scan(&t); err != nil { tagRows.Close(); return err }
		n.Tags = append(n.Tags, t)
	}
	tagRows.Close()
	linkRows, err := d.Query(`SELECT task_id FROM note_task_links WHERE note_id = ? ORDER BY task_id`, n.ID)
	if err != nil { return err }
	for linkRows.Next() {
		var tid int64
		if err := linkRows.Scan(&tid); err != nil { linkRows.Close(); return err }
		n.LinkedTasks = append(n.LinkedTasks, tid)
	}
	linkRows.Close()
	return nil
}

func boolInt(b bool) int { if b { return 1 }; return 0 }

func (d *DB) ListNotesForUser(userID int64, f NoteFilter) ([]models.Note, error) {
	q := `SELECT id, owner_id, title, body_md, pinned, created_at, updated_at
		FROM notes
		WHERE (owner_id = ? OR id IN (SELECT resource_id FROM shares WHERE resource_type='note' AND user_id=?))`
	args := []any{userID, userID}
	if f.Q != "" {
		q += ` AND (title LIKE ? OR body_md LIKE ?)`
		like := "%" + f.Q + "%"
		args = append(args, like, like)
	}
	if f.Tag != "" { q += ` AND id IN (SELECT note_id FROM note_tags WHERE tag = ?)`; args = append(args, f.Tag) }
	if f.Pinned != nil { q += ` AND pinned = ?`; args = append(args, boolInt(*f.Pinned)) }
	q += ` ORDER BY pinned DESC, updated_at DESC`
	rows, err := d.Query(q, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []models.Note
	for rows.Next() {
		var n models.Note
		if err := rows.Scan(&n.ID, &n.OwnerID, &n.Title, &n.BodyMD, &n.Pinned, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (d *DB) UpdateNote(id, userID int64, fields map[string]any, editorID int64) error {
	if !d.userHasNoteAccess(id, userID) { return ErrNotFound }
	var title, body string
	if err := d.QueryRow(`SELECT title, body_md FROM notes WHERE id = ?`, id).Scan(&title, &body); err != nil { return err }
	if _, err := d.Exec(`INSERT INTO note_versions (note_id, title, body_md, editor_id) VALUES (?, ?, ?, ?)`, id, title, body, editorID); err != nil {
		return err
	}
	allowed := map[string]bool{"title": true, "body_md": true, "pinned": true}
	var sets []string
	var args []any
	for k, v := range fields {
		if !allowed[k] { continue }
		sets = append(sets, fmt.Sprintf("%s = ?", k))
		args = append(args, v)
	}
	if len(sets) == 0 { return nil }
	args = append(args, id)
	_, err := d.Exec("UPDATE notes SET "+strings.Join(sets, ", ")+", updated_at = CURRENT_TIMESTAMP WHERE id = ?", args...)
	return err
}

func (d *DB) DeleteNote(id, userID int64) error {
	if !d.userIsNoteOwner(id, userID) { return ErrNotFound }
	_, err := d.Exec(`DELETE FROM notes WHERE id = ?`, id)
	return err
}

func (d *DB) userIsNoteOwner(noteID, userID int64) bool {
	var n int
	_ = d.QueryRow(`SELECT COUNT(*) FROM notes WHERE id = ? AND owner_id = ?`, noteID, userID).Scan(&n)
	return n > 0
}

func (d *DB) ListNoteVersions(noteID, userID int64) ([]models.NoteVersion, error) {
	if !d.userHasNoteAccess(noteID, userID) { return nil, ErrNotFound }
	rows, err := d.Query(`SELECT v.id, v.note_id, v.title, v.body_md, v.editor_id, v.saved_at, u.username
		FROM note_versions v LEFT JOIN users u ON u.id = v.editor_id
		WHERE v.note_id = ? ORDER BY v.saved_at DESC`, noteID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []models.NoteVersion
	for rows.Next() {
		var v models.NoteVersion
		var username sql.NullString
		if err := rows.Scan(&v.ID, &v.NoteID, &v.Title, &v.BodyMD, &v.EditorID, &v.SavedAt, &username); err != nil {
			return nil, err
		}
		if username.Valid { v.EditorName = username.String }
		out = append(out, v)
	}
	return out, rows.Err()
}

func (d *DB) RestoreNoteVersion(noteID, versionID, userID int64) error {
	if !d.userHasNoteAccess(noteID, userID) { return ErrNotFound }
	var title, body string
	if err := d.QueryRow(`SELECT title, body_md FROM note_versions WHERE id = ? AND note_id = ?`, versionID, noteID).Scan(&title, &body); err != nil {
		if errors.Is(err, sql.ErrNoRows) { return ErrNotFound }
		return err
	}
	return d.UpdateNote(noteID, userID, map[string]any{"title": title, "body_md": body}, userID)
}

func (d *DB) AddNoteTag(noteID, userID int64, tag string) error {
	if !d.userHasNoteAccess(noteID, userID) { return ErrNotFound }
	_, err := d.Exec(`INSERT OR IGNORE INTO note_tags (note_id, tag) VALUES (?, ?)`, noteID, tag)
	return err
}

func (d *DB) RemoveNoteTag(noteID, userID int64, tag string) error {
	if !d.userHasNoteAccess(noteID, userID) { return ErrNotFound }
	_, err := d.Exec(`DELETE FROM note_tags WHERE note_id = ? AND tag = ?`, noteID, tag)
	return err
}
