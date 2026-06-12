package db

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/vladimirkreslin/todoshka/internal/models"
)

var ErrNotFound = errors.New("not found")

func (d *DB) CreateUser(username, passwordHash string) (int64, error) {
	res, err := d.Exec(`INSERT INTO users (username, password_hash) VALUES (?, ?)`, username, passwordHash)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") { return 0, errors.New("username taken") }
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) GetUserByUsername(username string) (*models.User, error) {
	var u models.User
	err := d.QueryRow(`SELECT id, username, password_hash, created_at FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) { return nil, ErrNotFound }
	if err != nil { return nil, err }
	return &u, nil
}

func (d *DB) GetUserByID(id int64) (*models.User, error) {
	var u models.User
	err := d.QueryRow(`SELECT id, username, password_hash, created_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) { return nil, ErrNotFound }
	if err != nil { return nil, err }
	return &u, nil
}

func (d *DB) SearchUsers(q string) ([]models.User, error) {
	rows, err := d.Query(`SELECT id, username, '' AS password_hash, created_at FROM users WHERE username LIKE ? ORDER BY username LIMIT 20`, q+"%")
	if err != nil { return nil, err }
	defer rows.Close()
	var out []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt); err != nil { return nil, err }
		out = append(out, u)
	}
	return out, rows.Err()
}
