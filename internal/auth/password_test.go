package auth

import "testing"

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil { t.Fatal(err) }
	if hash == "hunter2" { t.Fatal("hash equals plaintext") }
	if !VerifyPassword(hash, "hunter2") { t.Fatal("verify failed for correct") }
	if VerifyPassword(hash, "wrong")     { t.Fatal("verify succeeded for wrong") }
}
