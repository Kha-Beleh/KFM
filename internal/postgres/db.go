package postgres

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"path/filepath"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed schema.sql
var schema string

type Store struct{ DB *sql.DB }

type User struct {
	ID           int64
	Username     string
	PasswordHash string
}

type File struct {
	Path    string
	Size    int64
	Deleted bool
}

type Operation struct {
	ID     int64
	Time   time.Time
	Type   string
	Path   string
	Status string
	Error  string
}

// подключается к PostgreSQL
func Open(address string) (*Store, error) {
	if address == "" {
		return nil, fmt.Errorf("задайте KFM_DATABASE_URL")
	}
	db, err := sql.Open("pgx", address)
	if err != nil {
		return nil, fmt.Errorf("не удалось настроить PostgreSQL")
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	store := &Store{DB: db}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("PostgreSQL недоступен")
	}
	return store, nil
}

// создаёт таблицы
func (store *Store) Migrate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err := store.DB.ExecContext(ctx, schema)
	return err
}

// закрывает подключение
func (store *Store) Close() error { return store.DB.Close() }

// регистрирует пользователя
func (store *Store) CreateUser(name string, hash string) (User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	user := User{Username: name}
	err := store.DB.QueryRowContext(ctx, `INSERT INTO users(username,password_hash) VALUES($1,$2) RETURNING id`, name, hash).Scan(&user.ID)
	if err != nil {
		return User{}, fmt.Errorf("не удалось зарегистрировать пользователя; возможно, логин занят")
	}
	return user, nil
}

// получает данные для входа
func (store *Store) GetUser(name string) (User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var user User
	err := store.DB.QueryRowContext(ctx, `SELECT id,username,password_hash FROM users WHERE username=$1`, name).Scan(&user.ID, &user.Username, &user.PasswordHash)
	return user, err
}

// фиксирует начало операции
func (store *Store) Start(userID int64, command string, path string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var id int64
	err := store.DB.QueryRowContext(ctx, `INSERT INTO operations(user_id,operation_type,path,status) VALUES(NULLIF($1,0),$2,$3,'pending') RETURNING id`, userID, command, path).Scan(&id)
	return id, err
}

// сохраняет результат операции и сведения о файле
func (store *Store) Finish(id int64, userID int64, file *File, operationErr error) error {
	var files []File
	if file != nil {
		files = append(files, *file)
	}
	return store.FinishFiles(id, userID, files, operationErr)
}

// сохраняет сведения обо всех затронутых файлах
func (store *Store) FinishFiles(id int64, userID int64, files []File, operationErr error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var fileID any
	for _, file := range files {
		var value int64
		err = tx.QueryRowContext(ctx, `INSERT INTO files(owner_id,location,filename,size,deleted) VALUES($1,$2,$3,$4,$5) ON CONFLICT(owner_id,location) DO UPDATE SET size=EXCLUDED.size,deleted=EXCLUDED.deleted,updated_at=CURRENT_TIMESTAMP RETURNING id`, userID, file.Path, filepath.Base(file.Path), file.Size, file.Deleted).Scan(&value)
		if err != nil {
			return err
		}
		if fileID == nil {
			fileID = value
		}
	}
	status, message := "success", ""
	if operationErr != nil {
		status, message = "failed", operationErr.Error()
	}
	_, err = tx.ExecContext(ctx, `SELECT kfm_finish_operation($1,$2,$3,$4,$5)`, id, userID, fileID, status, message)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// получает последние действия пользователя
func (store *Store) History(userID int64) ([]Operation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := store.DB.QueryContext(ctx, `SELECT id,timestamp,operation_type,path,status,error FROM operations WHERE user_id=$1 ORDER BY id DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var operations []Operation
	for rows.Next() {
		var operation Operation
		if err = rows.Scan(&operation.ID, &operation.Time, &operation.Type, &operation.Path, &operation.Status, &operation.Error); err != nil {
			return nil, err
		}
		operations = append(operations, operation)
	}
	return operations, rows.Err()
}
