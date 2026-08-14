package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailTaken         = errors.New("email already taken")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// PasswordHash is the bcrypt hash of the user's password.
	// json:"-" keeps it out of every API response.
	PasswordHash string `json:"-"`
}

// UserRepository is the outbound port for user persistence.
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	List(ctx context.Context) ([]User, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id string) error
}
