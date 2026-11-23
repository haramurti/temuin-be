// Package notification is the status-event / inbox bounded context (Fungsi 3).
package notification

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Type string

const (
	TypeClaimSubmitted       Type = "CLAIM_SUBMITTED"
	TypeClaimApproved        Type = "CLAIM_APPROVED"
	TypeClaimRejected        Type = "CLAIM_REJECTED"
	TypeClaimCancelled       Type = "CLAIM_CANCELLED"
	TypeClaimItemUnavailable Type = "CLAIM_ITEM_UNAVAILABLE"
	TypeClaimCompleted       Type = "CLAIM_COMPLETED"
	TypePickupChanged        Type = "PICKUP_CHANGED"
)

var ErrNotFound = errors.New("notification not found")

// Notification is a status event addressed to one recipient. EventKey is unique
// so one status change never yields duplicate notifications.
type Notification struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	RecipientID uuid.UUID  `gorm:"type:uuid;not null;index"`
	EventKey    string     `gorm:"size:200;uniqueIndex;not null"`
	Type        Type       `gorm:"size:40;not null"`
	Title       string     `gorm:"size:160;not null"`
	Body        string     `gorm:"type:text;not null"`
	ClaimID     *uuid.UUID `gorm:"type:uuid;index"`
	ReportID    *uuid.UUID `gorm:"type:uuid;index"`
	ReadAt      *time.Time
	CreatedAt   time.Time `gorm:"index"`
}

// EventKey builds an idempotency key: same entity + same transition + same recipient.
func EventKey(t Type, entityID uuid.UUID, recipient uuid.UUID, version string) string {
	return fmt.Sprintf("%s:%s:%s:%s", t, entityID, recipient, version)
}

type Repository interface {
	// CreateIfAbsent inserts unless EventKey already exists.
	CreateIfAbsent(ctx context.Context, n *Notification) error
	List(ctx context.Context, recipient uuid.UUID, unreadOnly bool, page, limit int) ([]Notification, int64, error)
	CountUnread(ctx context.Context, recipient uuid.UUID) (int64, error)
	MarkRead(ctx context.Context, id, recipient uuid.UUID, at time.Time) error
	MarkAllRead(ctx context.Context, recipient uuid.UUID, at time.Time) (int64, error)
	MarkReadByClaim(ctx context.Context, claimID, recipient uuid.UUID, at time.Time) error
}
