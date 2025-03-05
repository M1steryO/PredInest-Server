package postgres

import (
	"PredInvest/internal/config"
	"PredInvest/internal/storage"
	"database/sql"
	"errors"
	"fmt"
	"github.com/lib/pq"
)

type User struct {
	ID       int64  `json:"-"`
	Username string `json:"name"`
	Password string `json:"-"`
	Email    string `json:"email"`
}

type Storage struct {
	db *sql.DB
}

func New(storageConfig config.StorageConfig) (*Storage, error) {
	const op = "storage.postgres.New"
	connStr := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=%s",
		storageConfig.User, storageConfig.Password, storageConfig.DBName, storageConfig.SSLmode)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	stmt, err := db.Prepare(`
	CREATE TABLE IF NOT EXISTS users(
		id SERIAL PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		name TEXT,
		password TEXT NOT NULL);
	`)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	_, err = stmt.Exec()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	stmt, err = db.Prepare(`
	CREATE INDEX IF NOT EXISTS idx_email ON users(email);
	`)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	_, err = stmt.Exec()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	fmt.Printf("storage initialized\n")
	return &Storage{db: db}, nil
}

func (s *Storage) SaveUser(email string, name string, password string) (int64, error) {
	const op = "storage.postgres.SaveUser"

	var lastInsertId int64
	err := s.db.QueryRow("INSERT INTO users(email,name,password) VALUES($1,$2,$3) RETURNING id;",
		email, name, password,
	).Scan(&lastInsertId)

	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			if pqErr.Code == "23505" {
				return 0, fmt.Errorf("%s: %w", op, storage.ErrUserExists)
			}
		}

		return 0, fmt.Errorf("%s: %w", op, err)
	}
	return lastInsertId, nil
}

func (s *Storage) GetUser(email string) (User, error) {
	const op = "storage.postgres.GetUser"

	var id int64
	var name, password string
	err := s.db.QueryRow("SELECT id, name, password FROM users WHERE email = $1;", email).Scan(
		&id, &name, &password)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, storage.ErrUserNotFound
		}

		return User{}, fmt.Errorf("%s: execute statement: %w", op, err)
	}

	return User{
		ID:       id,
		Username: name,
		Password: password,
		Email:    email,
	}, nil
}

func (s *Storage) GetUserById(id int64) (User, error) {
	const op = "storage.postgres.GetUserById"

	var email, name, password string
	err := s.db.QueryRow("SELECT email, name, password FROM users WHERE id = $1;", id).Scan(
		&email, &name, &password)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, storage.ErrUserNotFound
		}

		return User{}, fmt.Errorf("%s: execute statement: %w", op, err)
	}

	return User{
		ID:       id,
		Username: name,
		Password: password,
		Email:    email,
	}, nil
}

func (s *Storage) UpdateUserById(id int64, name string, email string, password string) (int64, error) {
	const op = "storage.postgres.UpdateUser"

	err := s.db.QueryRow(
		"UPDATE users SET name=COALESCE(NULLIF($1, ''), name), "+
			"email=COALESCE(NULLIF($2, ''), email), "+
			"password=COALESCE(NULLIF($3, ''), password)"+
			"WHERE id = $4;",
		name, email, password, id).Err()

	if err != nil {
		var pqErr *pq.Error

		if errors.As(err, &pqErr) {
			if pqErr.Code == "23505" {

				return 0, fmt.Errorf("%s: %w", op, storage.ErrUserExists)
			}
		}
		return 0, fmt.Errorf("%s: execute statement: %w", op, err)
	}

	return id, nil
}
