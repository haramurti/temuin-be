// Package claim is the ownership-verification & handover bounded context (Fungsi 1).
package claim

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending         Status = "PENDING"          // Menunggu
	StatusApproved        Status = "APPROVED"         // Disetujui (not yet picked up)
	StatusRejected        Status = "REJECTED"         // Ditolak (may resubmit)
	StatusItemUnavailable Status = "ITEM_UNAVAILABLE" // Barang Tidak Tersedia
	StatusCompleted       Status = "COMPLETED"        // Serah terima selesai
	StatusCancelled       Status = "CANCELLED"        // dibatalkan pengaju
)

type PickupMethod string

const (
	PickupAtPoint    PickupMethod = "AT_PICKUP_POINT" // barang sudah dititipkan di pos
	PickupMeetFinder PickupMethod = "MEET_FINDER"     // masih dipegang penemu
)

var (
	ErrNotFound       = errors.New("claim not found")
	ErrNotFinder      = errors.New("only the finder can perform this action")
	ErrNotClaimant    = errors.New("only the claimant can perform this action")
	ErrInvalidState   = errors.New("invalid claim state for this action")
	ErrReasonRequired = errors.New("rejection reason is required")
	ErrConflict       = errors.New("claim conflicts with an existing open/approved claim")
)

// Claim is the aggregate root. SecretDescription/ProofPhotoURL are confidential:
// only claimant & finder can read them, never sent to AI.
type Claim struct {
	ID                uuid.UUID     `gorm:"type:uuid;primaryKey"`
	ReportID          uuid.UUID     `gorm:"type:uuid;not null;index"`
	ClaimantID        uuid.UUID     `gorm:"type:uuid;not null;index"`
	FinderID          uuid.UUID     `gorm:"type:uuid;not null;index"`
	SecretDescription string        `gorm:"type:text;not null"`
	ProofPhotoURL     *string       `gorm:"size:512"`
	Status            Status        `gorm:"size:20;not null;index"`
	RejectionReason   *string       `gorm:"type:text"`
	PickupMethod      *PickupMethod `gorm:"size:20"`
	PickupPointID     *uuid.UUID    `gorm:"type:uuid"`
	PreviousClaimID   *uuid.UUID    `gorm:"type:uuid"`
	DecidedAt         *time.Time
	CompletedAt       *time.Time
	CreatedAt         time.Time `gorm:"index"`
	UpdatedAt         time.Time
}

func New(reportID, claimantID, finderID uuid.UUID, secret string, proof *string, previous *uuid.UUID) *Claim {
	return &Claim{
		ID: uuid.New(), ReportID: reportID, ClaimantID: claimantID, FinderID: finderID,
		SecretDescription: secret, ProofPhotoURL: proof, Status: StatusPending, PreviousClaimID: previous,
	}
}

func (c *Claim) IsParty(uid uuid.UUID) bool { return c.ClaimantID == uid || c.FinderID == uid }

func (c *Claim) Approve(by uuid.UUID, method PickupMethod, pickupPointID *uuid.UUID, now time.Time) error {
	if by != c.FinderID {
		return ErrNotFinder
	}
	if c.Status != StatusPending {
		return ErrInvalidState
	}
	c.Status = StatusApproved
	c.PickupMethod = &method
	c.PickupPointID = pickupPointID
	c.DecidedAt = &now
	return nil
}

func (c *Claim) Reject(by uuid.UUID, reason string, now time.Time) error {
	if by != c.FinderID {
		return ErrNotFinder
	}
	if c.Status != StatusPending {
		return ErrInvalidState
	}
	if reason == "" {
		return ErrReasonRequired
	}
	c.Status = StatusRejected
	c.RejectionReason = &reason
	c.DecidedAt = &now
	return nil
}

func (c *Claim) Cancel(by uuid.UUID) error {
	if by != c.ClaimantID {
		return ErrNotClaimant
	}
	if c.Status != StatusPending {
		return ErrInvalidState
	}
	c.Status = StatusCancelled
	return nil
}

// Complete: finder confirms physical handover. Only after approval.
func (c *Claim) Complete(by uuid.UUID, now time.Time) error {
	if by != c.FinderID {
		return ErrNotFinder
	}
	if c.Status != StatusApproved {
		return ErrInvalidState
	}
	c.Status = StatusCompleted
	c.CompletedAt = &now
	return nil
}

// MarkItemUnavailable is a system transition when the item is gone.
func (c *Claim) MarkItemUnavailable() bool {
	if c.Status == StatusPending || c.Status == StatusApproved {
		c.Status = StatusItemUnavailable
		return true
	}
	return false
}

func (c *Claim) CanResubmit() bool { return c.Status == StatusRejected }

// RefreshPickup recomputes pickup instructions from the item's actual location.
func (c *Claim) RefreshPickup(method PickupMethod, pickupPointID *uuid.UUID) {
	if c.Status == StatusApproved {
		c.PickupMethod = &method
		c.PickupPointID = pickupPointID
	}
}

type ListFilter struct {
	Status *Status
	Page   int
	Limit  int
}

type Repository interface {
	Create(ctx context.Context, c *Claim) error
	Update(ctx context.Context, c *Claim) error
	FindByID(ctx context.Context, id uuid.UUID) (*Claim, error)
	FindByIDForUpdate(ctx context.Context, id uuid.UUID) (*Claim, error)
	ListByFinder(ctx context.Context, finderID uuid.UUID, f ListFilter) ([]Claim, int64, error)
	ListByClaimant(ctx context.Context, claimantID uuid.UUID, f ListFilter) ([]Claim, int64, error)
	ListByReport(ctx context.Context, reportID uuid.UUID) ([]Claim, error)
	// HasOpenClaim: claimant already has a PENDING/APPROVED claim on the report.
	HasOpenClaim(ctx context.Context, reportID, claimantID uuid.UUID) (bool, error)
	// HasApproved: report already has an APPROVED/COMPLETED claim.
	HasApproved(ctx context.Context, reportID uuid.UUID) (bool, error)
	ExistsForReport(ctx context.Context, reportID uuid.UUID) (bool, error)
	ExistsBetween(ctx context.Context, reportID, claimantID uuid.UUID, statuses []Status) (bool, error)
}
