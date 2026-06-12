package db

import (
	"testing"

	"github.com/vladimirkreslin/todoshka/internal/models"
)

func TestNoteCreateAndVersioning(t *testing.T) {
	d := newTestDB(t)
	uid, _ := d.CreateUser("alice", "x")
	id, err := d.CreateNote(uid, &models.Note{Title: "Hi", BodyMD: "# Hello"})
	if err != nil { t.Fatal(err) }
	if err := d.UpdateNote(id, uid, map[string]any{"title": "Hi v2", "body_md": "## Updated"}, uid); err != nil { t.Fatal(err) }
	vs, _ := d.ListNoteVersions(id, uid)
	if len(vs) != 1 { t.Fatalf("expected 1 version, got %d", len(vs)) }
	if vs[0].Title != "Hi" { t.Fatalf("version should hold old title, got %q", vs[0].Title) }
	if err := d.RestoreNoteVersion(id, vs[0].ID, uid); err != nil { t.Fatal(err) }
	n, _ := d.GetNote(id, uid)
	if n.Title != "Hi" { t.Fatalf("restore failed, got %q", n.Title) }
}

func TestNotePinAndTag(t *testing.T) {
	d := newTestDB(t)
	uid, _ := d.CreateUser("alice", "x")
	id, _ := d.CreateNote(uid, &models.Note{Title: "A", BodyMD: "x", Tags: []string{"work"}})
	if err := d.UpdateNote(id, uid, map[string]any{"pinned": true}, uid); err != nil { t.Fatal(err) }
	if err := d.AddNoteTag(id, uid, "home"); err != nil { t.Fatal(err) }
	if err := d.RemoveNoteTag(id, uid, "work"); err != nil { t.Fatal(err) }
	n, _ := d.GetNote(id, uid)
	if !n.Pinned { t.Fatal("not pinned") }
	if len(n.Tags) != 1 || n.Tags[0] != "home" { t.Fatalf("tags: %+v", n.Tags) }
}

func TestNoteListFilters(t *testing.T) {
	d := newTestDB(t)
	uid, _ := d.CreateUser("alice", "x")
	d.CreateNote(uid, &models.Note{Title: "Apples", BodyMD: "x"})
	d.CreateNote(uid, &models.Note{Title: "Oranges", BodyMD: "x"})
	d.CreateNote(uid, &models.Note{Title: "Mangoes", BodyMD: "x"})
	got, err := d.ListNotesForUser(uid, NoteFilter{Q: "an"})
	if err != nil { t.Fatal(err) }
	if len(got) < 2 { t.Fatalf("expected >=2, got %d", len(got)) }
}
