// Package user is the identity & profile bounded context.
package user

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound   = errors.New("user not found")
	ErrEmailTaken = errors.New("email or NIM already registered")
)

// User is the aggregate root for an account. NIM/faculty are self-declared
// and do not prove active-student status (see PRD 1.3).
// Contact fields (WhatsApp/Instagram) are restricted data.
type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name         string    `gorm:"size:120;not null"`
	Email        *string   `gorm:"size:160;uniqueIndex"`
	PasswordHash string    `gorm:"not null"`
	NIM          *string   `gorm:"size:20;uniqueIndex"`
	Faculty      *string   `gorm:"size:120"`
	WhatsApp     *string   `gorm:"size:30"`
	Instagram    *string   `gorm:"size:60"`
	AvatarURL    *string   `gorm:"size:512"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func New(name string, email *string, passwordHash string) *User {
	return &User{ID: uuid.New(), Name: name, Email: email, PasswordHash: passwordHash}
}

type Repository interface {
	Create(ctx context.Context, u *User) error
	Update(ctx context.Context, u *User) error
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByNIM(ctx context.Context, nim string) (*User, error)
	FindByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*User, error)
}

// PasswordHasher and TokenIssuer are ports implemented by infrastructure.
type PasswordHasher interface {
	Hash(plain string) (string, error)
	Compare(hash, plain string) bool
}

type TokenIssuer interface {
	Issue(userID uuid.UUID) (token string, expiresAt time.Time, err error)
	Parse(token string) (uuid.UUID, error)
}
