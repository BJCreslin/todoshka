package models

import "time"

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Task struct {
	ID          int64     `json:"id"`
	OwnerID     int64     `json:"owner_id"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	DueDate     *string   `json:"due_date"`
	Position    int64     `json:"position"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Subtasks    []Subtask `json:"subtasks,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	LinkedNotes []int64   `json:"linked_notes,omitempty"`
}

type Subtask struct {
	ID       int64  `json:"id"`
	TaskID   int64  `json:"task_id"`
	Title    string `json:"title"`
	Done     bool   `json:"done"`
	Position int64  `json:"position"`
}

type Note struct {
	ID          int64     `json:"id"`
	OwnerID     int64     `json:"owner_id"`
	Title       string    `json:"title"`
	BodyMD      string    `json:"body_md"`
	Pinned      bool      `json:"pinned"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Tags        []string  `json:"tags,omitempty"`
	LinkedTasks []int64   `json:"linked_tasks,omitempty"`
}

type NoteVersion struct {
	ID         int64     `json:"id"`
	NoteID     int64     `json:"note_id"`
	Title      string    `json:"title"`
	BodyMD     string    `json:"body_md"`
	EditorID   int64     `json:"editor_id"`
	SavedAt    time.Time `json:"saved_at"`
	EditorName string    `json:"editor_name,omitempty"`
}
