package db

import "testing"

func newTestDB(t *testing.T) *DB {
	t.Helper()
	d, err := Open(":memory:")
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { d.Close() })
	return d
}

func TestUserCreateAndGet(t *testing.T) {
	d := newTestDB(t)
	id, err := d.CreateUser("alice", "$2a$10$fakehash")
	if err != nil { t.Fatal(err) }
	u, err := d.GetUserByUsername("alice")
	if err != nil { t.Fatal(err) }
	if u.ID != id || u.Username != "alice" { t.Fatalf("%+v", u) }
}

func TestUserDuplicate(t *testing.T) {
	d := newTestDB(t)
	if _, err := d.CreateUser("bob", "x"); err != nil { t.Fatal(err) }
	if _, err := d.CreateUser("bob", "y"); err == nil { t.Fatal("expected dup") }
}

func TestUserSearch(t *testing.T) {
	d := newTestDB(t)
	d.CreateUser("alice", "x"); d.CreateUser("alicia", "x"); d.CreateUser("bob", "x")
	got, err := d.SearchUsers("ali")
	if err != nil { t.Fatal(err) }
	if len(got) != 2 { t.Fatalf("got %d: %+v", len(got), got) }
}
