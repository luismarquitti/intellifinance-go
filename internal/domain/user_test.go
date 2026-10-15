package domain_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

func TestNewUser_Success(t *testing.T) {
	user, err := domain.NewUser("Luis@IntelliFinance.app", "Luis Marquitti", "hashed_secret_pw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.ID == uuid.Nil {
		t.Error("expected valid UUID")
	}
	if user.Email != "luis@intellifinance.app" {
		t.Errorf("expected normalized lowercase email, got %q", user.Email)
	}
	if user.FullName != "Luis Marquitti" {
		t.Errorf("expected full name %q, got %q", "Luis Marquitti", user.FullName)
	}
	if !user.IsActive {
		t.Error("expected new user to be active")
	}
	if user.CreatedAt.IsZero() || user.UpdatedAt.IsZero() {
		t.Error("expected non-zero timestamps")
	}
}

func TestNewUser_ValidationErrors(t *testing.T) {
	tests := []struct {
		name         string
		email        string
		fullName     string
		passwordHash string
		expectedErr  error
	}{
		{
			name:         "Empty email",
			email:        "   ",
			fullName:     "Luis Marquitti",
			passwordHash: "hash123",
			expectedErr:  domain.ErrInvalidEmail,
		},
		{
			name:         "Invalid email without domain",
			email:        "plainaddress",
			fullName:     "Luis Marquitti",
			passwordHash: "hash123",
			expectedErr:  domain.ErrInvalidEmail,
		},
		{
			name:         "Invalid email without dot",
			email:        "user@localhost",
			fullName:     "Luis Marquitti",
			passwordHash: "hash123",
			expectedErr:  domain.ErrInvalidEmail,
		},
		{
			name:         "Empty full name",
			email:        "luis@example.com",
			fullName:     "   ",
			passwordHash: "hash123",
			expectedErr:  domain.ErrEmptyName,
		},
		{
			name:         "Empty password hash",
			email:        "luis@example.com",
			fullName:     "Luis Marquitti",
			passwordHash: "   ",
			expectedErr:  domain.ErrUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.NewUser(tc.email, tc.fullName, tc.passwordHash)
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("expected error %v, got %v", tc.expectedErr, err)
			}
		})
	}
}

func TestUser_Validate_NilID(t *testing.T) {
	user := &domain.User{
		ID:           uuid.Nil,
		Email:        "luis@example.com",
		FullName:     "Luis",
		PasswordHash: "secret",
	}
	if err := user.Validate(); !errors.Is(err, domain.ErrInvalidID) {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}
}

func TestUser_LifecycleAndProfileUpdates(t *testing.T) {
	user, err := domain.NewUser("luis@example.com", "Luis Marquitti", "secret_hash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	user.Deactivate()
	if user.IsActive {
		t.Error("expected user to be inactive")
	}

	user.Activate()
	if !user.IsActive {
		t.Error("expected user to be active")
	}

	err = user.UpdateProfile("Luis A. Marquitti", "https://avatar.example.com/pic.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.FullName != "Luis A. Marquitti" {
		t.Errorf("expected updated name, got %q", user.FullName)
	}
	if user.AvatarURL != "https://avatar.example.com/pic.png" {
		t.Errorf("expected updated avatar URL, got %q", user.AvatarURL)
	}

	err = user.UpdateProfile("   ", "")
	if !errors.Is(err, domain.ErrEmptyName) {
		t.Errorf("expected ErrEmptyName, got %v", err)
	}
}
