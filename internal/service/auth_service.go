package service

import (
	"context"
	"errors"
	"strings"

	"panel/internal/model"
	"panel/internal/repository"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	// ErrInvalidCredentials is returned on login failure.
	ErrInvalidCredentials = errors.New("invalid username or password")
	// ErrUsernameTaken is returned when the username already exists.
	ErrUsernameTaken = errors.New("username already exists")
	// ErrUsernameRequired is returned when the username is empty.
	ErrUsernameRequired = errors.New("username is required")
	// ErrCurrentPasswordRequired is returned when current password is empty.
	ErrCurrentPasswordRequired = errors.New("current password is required")
	// ErrPasswordTooShort is returned when new password is too short.
	ErrPasswordTooShort = errors.New("password too short")
	// ErrPasswordMismatch is returned when password confirmation mismatches.
	ErrPasswordMismatch = errors.New("password confirmation mismatch")
	// ErrCurrentPasswordInvalid is returned when current password is invalid.
	ErrCurrentPasswordInvalid = errors.New("current password is invalid")
)

// AuthService handles login.
type AuthService struct {
	userRepo *repository.UserRepository
}

// NewAuthService creates a service.
func NewAuthService(userRepo *repository.UserRepository) *AuthService {
	return &AuthService{userRepo: userRepo}
}

// Authenticate validates user credentials.
func (s *AuthService) Authenticate(ctx context.Context, username, password string) (*model.User, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := s.userRepo.UpdateLoginTime(ctx, user.ID); err != nil {
		return nil, err
	}

	return user, nil
}

// GetUserByID returns a user by id.
func (s *AuthService) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("user id is required")
	}

	return s.userRepo.FindByID(ctx, id)
}

// UpdateUsername updates the current user's username.
func (s *AuthService) UpdateUsername(ctx context.Context, id, username string) error {
	id = strings.TrimSpace(id)
	username = strings.TrimSpace(username)
	if id == "" {
		return errors.New("user id is required")
	}
	if username == "" {
		return ErrUsernameRequired
	}

	existing, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if existing != nil && existing.ID != id {
		return ErrUsernameTaken
	}

	return s.userRepo.UpdateUsername(ctx, id, username)
}

// UpdatePassword updates the current user's password.
func (s *AuthService) UpdatePassword(ctx context.Context, id, currentPassword, newPassword, confirmPassword string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("user id is required")
	}
	if strings.TrimSpace(currentPassword) == "" {
		return ErrCurrentPasswordRequired
	}
	if len(newPassword) < 6 {
		return ErrPasswordTooShort
	}
	if newPassword != confirmPassword {
		return ErrPasswordMismatch
	}

	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return ErrCurrentPasswordInvalid
	}

	passwordHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	return s.userRepo.UpdatePasswordHash(ctx, id, passwordHash)
}

// VerifyCurrentPassword validates the current user's password without changing it.
func (s *AuthService) VerifyCurrentPassword(ctx context.Context, id, currentPassword string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("user id is required")
	}
	if strings.TrimSpace(currentPassword) == "" {
		return ErrCurrentPasswordRequired
	}

	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return ErrCurrentPasswordInvalid
	}
	return nil
}

// HashPassword hashes a password.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
