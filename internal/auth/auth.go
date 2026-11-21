package auth

import (
	"fmt"
	"regexp"
	"time"

	"github.com/Kha-Beleh/KFM/internal/postgres"
	"golang.org/x/crypto/bcrypt"
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,50}$`)

type Service struct {
	Store       *postgres.Store
	attempts    int
	lockedUntil time.Time
}

// хэширует пароль с солью
func HashPassword(password string) (string, error) {
	if len(password) < 12 || len(password) > 72 {
		return "", fmt.Errorf("пароль должен содержать от 12 до 72 байт")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(hash), err
}

// проверяет пароль
func CheckPassword(hash string, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// регистрирует пользователя
func (service *Service) Register(name string, password string) (postgres.User, error) {
	if !usernamePattern.MatchString(name) {
		return postgres.User{}, fmt.Errorf("логин: 3–50 латинских букв, цифр, точек, дефисов или подчёркиваний")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return postgres.User{}, err
	}
	return service.Store.CreateUser(name, hash)
}

// проверяет данные для входа
func (service *Service) Login(name string, password string) (postgres.User, error) {
	if time.Now().Before(service.lockedUntil) {
		return postgres.User{}, fmt.Errorf("слишком много попыток; повторите вход через минуту")
	}
	user, err := service.Store.GetUser(name)
	hash := user.PasswordHash
	if err != nil {
		hash = "$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxDyV4VoW/zH4QnI7V2Mo5fTaIu"
	}
	valid := CheckPassword(hash, password)
	if err != nil || !valid {
		service.attempts++
		if service.attempts >= 5 {
			service.lockedUntil = time.Now().Add(time.Minute)
			service.attempts = 0
		}
		return postgres.User{}, fmt.Errorf("неверный логин или пароль")
	}
	service.attempts = 0
	user.PasswordHash = ""
	return user, nil
}
