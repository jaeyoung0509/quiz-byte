package domain

import (
	"time"
)

// User represents a domain user object
type User struct {
	ID                string
	GoogleID          string
	Email             string
	Name              string
	ProfilePictureURL string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

// NewUser creates a new User instance
func NewUser(googleID, email string) *User {
	now := time.Now()
	return &User{
		GoogleID:  googleID,
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Validate validates the user
func (u *User) Validate() error {
	if u.GoogleID == "" {
		return NewValidationError("google_id is required")
	}
	if u.Email == "" {
		return NewValidationError("email is required")
	}
	return nil
}
