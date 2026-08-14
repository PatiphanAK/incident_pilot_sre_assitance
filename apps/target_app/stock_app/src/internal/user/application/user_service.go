package application

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"stock_app/src/internal/user/domain"
)

// dummyHash is compared when the email is unknown, so that a wrong email and a
// wrong password take a similar amount of time (user-enumeration mitigation).
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("stock-app-dummy"), bcrypt.DefaultCost)

type UserService struct {
	repo domain.UserRepository
}

func NewUserService(repo domain.UserRepository) *UserService {
	return &UserService{repo: repo}
}

// Register creates a new user with a bcrypt-hashed password.
func (s *UserService) Register(ctx context.Context, username, email, password string) (*domain.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	user := &domain.User{
		ID:           uuid.NewString(),
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// Login returns the user when the email and password match.
// Unknown email and wrong password both yield ErrInvalidCredentials.
func (s *UserService) Login(ctx context.Context, email, password string) (*domain.User, error) {
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			// Burn the same bcrypt time as a real comparison, then fail identically.
			_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}
	return user, nil
}

func (s *UserService) CreateUser(ctx context.Context, username, email string) (*domain.User, error) {
	now := time.Now().UTC()
	user := &domain.User{
		ID:        uuid.NewString(),
		Username:  username,
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) GetUser(ctx context.Context, id string) (*domain.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UserService) ListUsers(ctx context.Context) ([]domain.User, error) {
	return s.repo.List(ctx)
}

func (s *UserService) UpdateUser(ctx context.Context, id, username, email string) (*domain.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	user.Username = username
	user.Email = email
	user.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) DeleteUser(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
