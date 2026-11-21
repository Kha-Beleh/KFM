package auth

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestSecurityPasswordHashesHaveIndependentSalts(t *testing.T) {
	password := "test password 123"
	first, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || strings.Contains(first, password) {
		t.Fatal("password not independently salted")
	}
	cost, err := bcrypt.Cost([]byte(first))
	if err != nil || cost < 12 {
		t.Fatalf("unexpected bcrypt cost: %d, %v", cost, err)
	}
	if !CheckPassword(first, password) || CheckPassword(first, password+"wrong") {
		t.Fatal("incorrect password verification")
	}
}

func TestSecurityPasswordLengthAndMalformedHashes(t *testing.T) {
	for _, password := range []string{"", strings.Repeat("a", 11), strings.Repeat("a", 73)} {
		if _, err := HashPassword(password); err == nil {
			t.Errorf("accepted password of %d bytes", len(password))
		}
	}
	hash, err := HashPassword(strings.Repeat("a", 72))
	if err != nil || !CheckPassword(hash, strings.Repeat("a", 72)) {
		t.Fatal("72-byte password rejected")
	}
	for _, hash := range []string{"", "plain-password", "$2a$12$broken", "$2a$99$invalid"} {
		if CheckPassword(hash, "test password 123") {
			t.Errorf("accepted invalid hash %q", hash)
		}
	}
}

func TestSecurityLoginLockIsCheckedBeforeDatabase(t *testing.T) {
	service := Service{lockedUntil: time.Now().Add(time.Minute)}
	if user, err := service.Login("student", "test password 123"); err == nil || user.ID != 0 {
		t.Fatal("login ignored the active lock")
	}
}
