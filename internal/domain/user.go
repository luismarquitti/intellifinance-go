package domain

import (
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

// User represents an authenticated identity within the system.
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	FullName     string    `json:"full_name"`
	AvatarURL    string    `json:"avatar_url,omitempty"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NewUser constructs and validates a new User entity.
func NewUser(email, fullName, passwordHash string) (*User, error) {
	now := time.Now().UTC()
	u := &User{
		ID:           uuid.New(),
		Email:        strings.ToLower(strings.TrimSpace(email)),
		PasswordHash: passwordHash,
		FullName:     strings.TrimSpace(fullName),
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := u.Validate(); err != nil {
		return nil, err
	}
	return u, nil
}

// Validate ensures user fields satisfy business constraints.
func (u *User) Validate() error {
	if u.ID == uuid.Nil {
		return ErrInvalidID
	}
	if strings.TrimSpace(u.Email) == "" {
		return ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(u.Email)
	if err != nil || addr.Address != u.Email || !strings.Contains(u.Email, ".") {
		return ErrInvalidEmail
	}
	if strings.TrimSpace(u.FullName) == "" {
		return ErrEmptyName
	}
	if strings.TrimSpace(u.PasswordHash) == "" {
		return ErrUnauthorized
	}
	return nil
}

// Deactivate disables the user account.
func (u *User) Deactivate() {
	u.IsActive = false
	u.UpdatedAt = time.Now().UTC()
}

// Activate enables the user account.
func (u *User) Activate() {
	u.IsActive = true
	u.UpdatedAt = time.Now().UTC()
}

// UpdateProfile updates the mutable profile fields of the user.
func (u *User) UpdateProfile(fullName, avatarURL string) error {
	trimmedName := strings.TrimSpace(fullName)
	if trimmedName == "" {
		return ErrEmptyName
	}
	u.FullName = trimmedName
	u.AvatarURL = strings.TrimSpace(avatarURL)
	u.UpdatedAt = time.Now().UTC()
	return nil
}
