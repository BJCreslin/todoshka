package auth

import (
	"testing"
	"time"
)

func TestJWTRoundtrip(t *testing.T) {
	const secret = "secretsecretsecretsecret"
	tok, err := IssueToken(42, "alice", secret, 30*time.Minute)
	if err != nil { t.Fatal(err) }
	uid, uname, err := ParseToken(tok, secret)
	if err != nil { t.Fatal(err) }
	if uid != 42 || uname != "alice" { t.Fatalf("got %d %s", uid, uname) }
}

func TestJWTWrongSecret(t *testing.T) {
	const s = "secretsecretsecretsecret"
	tok, _ := IssueToken(1, "x", s, time.Minute)
	if _, _, err := ParseToken(tok, "differentdifferentdifferent"); err == nil {
		t.Fatal("expected wrong-secret error")
	}
}

func TestJWTExpired(t *testing.T) {
	const s = "secretsecretsecretsecret"
	tok, _ := IssueToken(1, "x", s, -time.Minute)
	if _, _, err := ParseToken(tok, s); err == nil {
		t.Fatal("expected expired error")
	}
}
