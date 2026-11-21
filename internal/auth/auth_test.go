package auth

import "testing"

func TestPasswordHash(t *testing.T) {
	password := "correct password 123"
	first, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || first == password {
		t.Fatal("password salt missing")
	}
	if !CheckPassword(first, password) || CheckPassword(first, "wrong") || CheckPassword("bad hash", password) {
		t.Fatal("password verification failed")
	}
	if _, err = HashPassword("short"); err == nil {
		t.Fatal("accepted short password")
	}
}
